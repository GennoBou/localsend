package server

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/GennoBou/localsend/pkg/crypto"
	"github.com/GennoBou/localsend/pkg/protocol"
	"github.com/GennoBou/localsend/pkg/session"
)

// ShareFile defines a file to be shared in reverse file transfer (Download API).
type ShareFile struct {
	ID       string
	Path     string
	FileName string
	Size     int64
	FileType string
}

// Server is the receiving API server for LocalSend.
type Server struct {
	myDevice        protocol.Device
	tlsCert         tls.Certificate
	sessionMgr      *session.SessionManager
	saveDir         string
	pin             string
	strictTLS       bool
	httpServer      *http.Server
	downloadServer  *http.Server
	downloadMu      sync.Mutex
	sharedFiles     []ShareFile
	downloadSession string

	// Callback functions
	OnPrepareUpload func(sender protocol.Device, files []protocol.FileMetadata) (acceptedFiles map[string]bool, accepted bool)
	OnProgress      func(sessionID string, fileID string, current int64, total int64)
	OnDone          func(sessionID string, fileID string, savedPath string)
	OnDiscover      func(device protocol.Device)
}

// NewServer creates a new receiving server.
func NewServer(myDevice protocol.Device, tlsCert tls.Certificate, saveDir string, pin string, strictTLS bool) *Server {
	return &Server{
		myDevice:   myDevice,
		tlsCert:    tlsCert,
		sessionMgr: session.NewSessionManager(10 * time.Minute),
		saveDir:    saveDir,
		pin:        pin,
		strictTLS:  strictTLS,
	}
}

// Start launches the HTTPS receiving server.
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/localsend/v1/info", s.handleInfo)
	mux.HandleFunc("/api/localsend/v2/info", s.handleInfo)
	mux.HandleFunc("/api/localsend/v2/register", s.handleRegister)
	mux.HandleFunc("/api/localsend/v2/prepare-upload", s.handlePrepareUpload)
	mux.HandleFunc("/api/localsend/v2/upload", s.handleUpload)
	mux.HandleFunc("/api/localsend/v2/cancel", s.handleCancel)

	// Create directory
	if err := os.MkdirAll(s.saveDir, 0755); err != nil {
		return fmt.Errorf("failed to create save directory: %w", err)
	}

	// Explicitly create a Listener that listens on tcp4 (IPv4)
	ln, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	if s.myDevice.Protocol == "http" {
		s.httpServer = &http.Server{
			Handler: mux,
		}
		return s.httpServer.Serve(ln)
	}

	// Request client certificate if available, but allow connections without it (hybrid mTLS)
	clientAuth := tls.RequestClientCert
	if s.strictTLS {
		clientAuth = tls.RequireAnyClientCert
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{s.tlsCert},
		ClientAuth:   clientAuth,
	}

	s.httpServer = &http.Server{
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	return s.httpServer.ServeTLS(ln, "", "")
}

// Shutdown gracefully stops the receiving server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.sessionMgr.Close()
	s.StopDownloadServer(ctx)

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// writeInfo writes a common device info response.
func (s *Server) writeInfo(w http.ResponseWriter) {
	resp := protocol.InfoResponse{
		Alias:       s.myDevice.Alias,
		Version:     protocol.ProtocolVersion,
		DeviceModel: s.myDevice.DeviceModel,
		DeviceType:  s.myDevice.DeviceType,
		Fingerprint: s.myDevice.Fingerprint,
		Download:    s.myDevice.Download,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleInfo handles the GET /api/localsend/v2/info endpoint.
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.writeInfo(w)
}

// handleRegister handles the POST /api/localsend/v2/register endpoint.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var sender protocol.Device
	if err := json.NewDecoder(r.Body).Decode(&sender); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Assign the sender's IP address
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	sender.IP = ip

	if s.OnDiscover != nil {
		s.OnDiscover(sender)
	}

	// Return own information
	s.writeInfo(w)
}

// handlePrepareUpload handles the POST /api/localsend/v2/prepare-upload endpoint.
func (s *Server) handlePrepareUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. PIN validation
	if s.pin != "" {
		reqPin := r.URL.Query().Get("pin")
		if reqPin != s.pin {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	// 2. Client certificate hash validation
	clientFingerprint, hasCert := s.checkClientCertificate(r)
	if s.strictTLS {
		if !hasCert {
			http.Error(w, "Client certificate required", http.StatusForbidden)
			return
		}
	}

	var req protocol.PrepareUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Check if it matches the registered fingerprint during prepare-upload

	// 2.5 Check for duplicate files (204 No Content).
	// Verify if all files already exist in the save directory (same name and size)
	allDuplicate := true
	for _, f := range req.Files {
		cleanName := filepath.Base(f.FileName)
		targetPath := filepath.Join(s.saveDir, cleanName)
		info, err := os.Stat(targetPath)
		if err != nil {
			allDuplicate = false
			break
		}
		if info.Size() != f.Size {
			allDuplicate = false
			break
		}
	}
	if len(req.Files) > 0 && allDuplicate {
		w.WriteHeader(http.StatusNoContent) // 204 No Content (ファイル転送不要)
		return
	}

	// 3. Check if other sessions are busy
	if s.sessionMgr.IsBusy() {
		w.WriteHeader(http.StatusConflict) // 409 Conflict (Busy)
		return
	}

	// 4. Confirm acceptance of reception (callback)
	senderIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	req.Info.IP = senderIP

	var filesList []protocol.FileMetadata
	for _, f := range req.Files {
		filesList = append(filesList, f)
	}

	acceptedFiles, accepted := map[string]bool(nil), true
	if s.OnPrepareUpload != nil {
		acceptedFiles, accepted = s.OnPrepareUpload(req.Info, filesList)
	}

	if !accepted {
		w.WriteHeader(http.StatusForbidden) // 403 Forbidden
		return
	}

	// Filter to only accepted files
	filteredFiles := make(map[string]protocol.FileMetadata)
	for id, f := range req.Files {
		if acceptedFiles == nil || acceptedFiles[id] {
			filteredFiles[id] = f
		}
	}

	// Return 403 Forbidden if none are accepted
	if len(filteredFiles) == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// 5. Session creation
	// Check if certificate validation succeeded
	clientVerified := hasCert && (req.Info.Fingerprint == clientFingerprint)

	sessionObj := s.sessionMgr.CreateSession(senderIP, clientVerified, filteredFiles)

	resp := protocol.PrepareUploadResponse{
		SessionID: sessionObj.ID,
		Files:     sessionObj.Files,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleUpload handles the POST /api/localsend/v2/upload endpoint.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	fileID := r.URL.Query().Get("fileId")
	token := r.URL.Query().Get("token")

	sessionObj, ok := s.sessionMgr.GetSession(sessionID)
	if !ok {
		http.Error(w, "Session not found", http.StatusBadRequest)
		return
	}

	expectedToken, fileMeta, exists := sessionObj.GetFileTokenAndMeta(fileID)
	clientIP, clientVerified := sessionObj.GetClientInfo()

	if !exists || expectedToken != token {
		http.Error(w, "Invalid token or fileId", http.StatusBadRequest)
		return
	}

	// Verify the client's IP address
	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if remoteIP != clientIP {
		http.Error(w, "IP address mismatch", http.StatusForbidden)
		return
	}

	// Re-validate the certificate hash if strictTLS is enabled
	if s.strictTLS {
		_, hasCert := s.checkClientCertificate(r)
		if !hasCert || !clientVerified {
			http.Error(w, "TLS connection untrusted", http.StatusForbidden)
			return
		}
		// Check if it matches the registered fingerprint during prepare-upload
		if fileMeta.ID == "" { // 無効チェック
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		// Ideally compared with the fingerprint recorded in sessionObj.
		// Here clientVerified flag indicates it is already verified.
	}

	// Determine a unique save path to avoid conflicts
	savedPath := getUniquePath(s.saveDir, fileMeta.FileName)

	file, err := os.Create(savedPath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)

	// Copy data while monitoring progress
	buffer := make([]byte, 32*1024)
	var written int64

	for {
		// Check if the session was canceled in the middle
		if _, ok := s.sessionMgr.GetSession(sessionID); !ok {
			file.Close()
			_ = os.Remove(savedPath)
			http.Error(w, "Upload canceled", http.StatusBadRequest)
			return
		}

		nr, er := r.Body.Read(buffer)
		if nr > 0 {
			nw, ew := writer.Write(buffer[:nr])
			if nw > 0 {
				written += int64(nw)
				sessionObj.UpdateProgress(fileID, written)
				if s.OnProgress != nil {
					s.OnProgress(sessionID, fileID, written, fileMeta.Size)
				}
			}
			if ew != nil {
				http.Error(w, "Write error", http.StatusInternalServerError)
				return
			}
			if nr != nw {
				http.Error(w, "Short write", http.StatusInternalServerError)
				return
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}
	}

	// Verify SHA-256 checksum if provided in prepare-upload metadata (LocalSend Protocol v2.2)
	if fileMeta.Sha256 != "" {
		calculatedHash := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(calculatedHash, fileMeta.Sha256) {
			file.Close()
			_ = os.Remove(savedPath)
			http.Error(w, "Checksum mismatch", http.StatusUnprocessableEntity)
			return
		}
	}

	if s.OnDone != nil {
		s.OnDone(sessionID, fileID, savedPath)
	}

	// Release the session if transfer for all files in the session is completed
	if sessionObj.IsCompleted() {
		s.sessionMgr.DeleteSession(sessionID)
	}

	w.WriteHeader(http.StatusOK)
}

// handleCancel handles the POST /api/localsend/v2/cancel endpoint.
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	s.sessionMgr.DeleteSession(sessionID)

	w.WriteHeader(http.StatusOK)
}

// checkClientCertificate extracts the fingerprint of the client certificate from the request.
func (s *Server) checkClientCertificate(r *http.Request) (string, bool) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return "", false
	}
	cert := r.TLS.PeerCertificates[0]
	fingerprint := crypto.CalculateFingerprint(cert.Raw)
	return fingerprint, true
}

// getUniquePath returns a unique path in the save directory that does not conflict with existing filenames.
func getUniquePath(dir, filename string) string {
	cleanName := filepath.Base(filename)
	ext := filepath.Ext(cleanName)
	base := cleanName[:len(cleanName)-len(ext)]
	path := filepath.Join(dir, cleanName)

	i := 1
	for {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		i++
	}
	return path
}

// StartDownloadServer dynamically starts an HTTP sharing server for browsers.
func (s *Server) StartDownloadServer(port int, sharedFiles []ShareFile) error {
	s.downloadMu.Lock()
	defer s.downloadMu.Unlock()

	if s.downloadServer != nil {
		return fmt.Errorf("download server is already running")
	}

	s.sharedFiles = sharedFiles
	s.downloadSession = uuid.NewString()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWebUI)
	mux.HandleFunc("/api/localsend/v2/prepare-download", s.handlePrepareDownload)
	mux.HandleFunc("/api/localsend/v2/download", s.handleDownload)

	// Explicitly create a Listener that listens on tcp4 (IPv4)
	ln, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d for download server: %w", port, err)
	}

	s.downloadServer = &http.Server{
		Handler: mux,
	}

	go func() {
		_ = s.downloadServer.Serve(ln)
	}()

	return nil
}

// StopDownloadServer gracefully stops the browser sharing server.
func (s *Server) StopDownloadServer(ctx context.Context) {
	s.downloadMu.Lock()
	defer s.downloadMu.Unlock()

	if s.downloadServer != nil {
		_ = s.downloadServer.Shutdown(ctx)
		s.downloadServer = nil
		s.sharedFiles = nil
		s.downloadSession = ""
	}
}

// handleWebUI displays a simple download screen for web browsers.
func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Validate if a PIN code is configured
	if s.pin != "" {
		// Handle POST submission of the PIN
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			inputPin := r.FormValue("pin")
			if inputPin == s.pin {
				// Save to cookie and redirect
				http.SetCookie(w, &http.Cookie{
					Name:     "session_pin",
					Value:    s.pin,
					Path:     "/",
					HttpOnly: true,
				})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}

		// Validate cookie
		cookiePin := ""
		if cookie, err := r.Cookie("session_pin"); err == nil {
			cookiePin = cookie.Value
		}

		if cookiePin != s.pin {
			// Show a simple PIN entry form
			s.handlePINForm(w, r)
			return
		}
	}

	const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LocalSend Download</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f7; padding: 20px; color: #333; }
        .container { max-width: 600px; margin: 0 auto; background: white; padding: 25px; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); }
        h1 { font-size: 24px; margin-bottom: 20px; }
        .file-list { list-style: none; padding: 0; }
        .file-item { display: flex; justify-content: space-between; align-items: center; padding: 12px; border-bottom: 1px solid #eee; }
        .file-info { display: flex; flex-direction: column; }
        .file-name { font-weight: 500; }
        .file-size { font-size: 12px; color: #888; margin-top: 4px; }
        .download-btn { background: #0071e3; color: white; border: none; padding: 8px 16px; border-radius: 6px; text-decoration: none; font-size: 14px; font-weight: 500; cursor: pointer; }
        .download-btn:hover { background: #0077ed; }
    </style>
</head>
<body>
    <div class="container">
        <h1>LocalSend Shared Files</h1>
        <p>From: <strong>{{.Alias}}</strong></p>
        <ul class="file-list">
            {{range .Files}}
            <li class="file-item">
                <div class="file-info">
                    <span class="file-name">{{.FileName}}</span>
                    <span class="file-size">{{.Size}} bytes</span>
                </div>
                <a href="/api/localsend/v2/download?sessionId={{$.SessionID}}&fileId={{.ID}}" class="download-btn">Download</a>
            </li>
            {{end}}
        </ul>
    </div>
</body>
</html>`

	s.downloadMu.Lock()
	files := s.sharedFiles
	sessionID := s.downloadSession
	alias := s.myDevice.Alias
	s.downloadMu.Unlock()

	tmpl, err := template.New("webui").Parse(htmlTemplate)
	if err != nil {
		http.Error(w, "Template compile error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Alias     string
		SessionID string
		Files     []ShareFile
	}{
		Alias:     alias,
		SessionID: sessionID,
		Files:     files,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

// handlePrepareDownload handles the POST /api/localsend/v2/prepare-download endpoint.
func (s *Server) handlePrepareDownload(w http.ResponseWriter, r *http.Request) {
	s.downloadMu.Lock()
	defer s.downloadMu.Unlock()

	if s.downloadSession == "" {
		http.Error(w, "No active shared files", http.StatusNotFound)
		return
	}

	// PIN code validation
	if s.pin != "" {
		reqPin := r.URL.Query().Get("pin")
		if reqPin == "" {
			if cookie, err := r.Cookie("session_pin"); err == nil {
				reqPin = cookie.Value
			}
		}
		if reqPin != s.pin {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	filesMap := make(map[string]protocol.FileMetadata)
	for _, f := range s.sharedFiles {
		filesMap[f.ID] = protocol.FileMetadata{
			ID:       f.ID,
			FileName: f.FileName,
			Size:     f.Size,
			FileType: f.FileType,
		}
	}

	resp := protocol.PrepareDownloadResponse{
		Info:      s.myDevice,
		SessionID: s.downloadSession,
		Files:     filesMap,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleDownload handles the GET /api/localsend/v2/download endpoint.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// PIN validation
	if s.pin != "" {
		reqPin := r.URL.Query().Get("pin")
		if reqPin == "" {
			if cookie, err := r.Cookie("session_pin"); err == nil {
				reqPin = cookie.Value
			}
		}
		if reqPin != s.pin {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	sessionID := r.URL.Query().Get("sessionId")
	fileID := r.URL.Query().Get("fileId")

	s.downloadMu.Lock()
	actualSession := s.downloadSession
	var targetFile *ShareFile
	for _, f := range s.sharedFiles {
		if f.ID == fileID {
			targetFile = &f
			break
		}
	}
	s.downloadMu.Unlock()

	if actualSession == "" || sessionID != actualSession {
		http.Error(w, "Invalid session ID", http.StatusForbidden)
		return
	}

	if targetFile == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	file, err := os.Open(targetFile.Path)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Configure Content-Disposition to prevent garbled Japanese filenames
	escapedName := url.PathEscape(targetFile.FileName)
	// Adjust space encoding like %+
	escapedName = strings.ReplaceAll(escapedName, "+", "%20")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", escapedName))
	w.Header().Set("Content-Type", targetFile.FileType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", targetFile.Size))

	_, _ = io.Copy(w, file)
}

// handlePINForm displays the PIN code entry screen for web browsers.
func (s *Server) handlePINForm(w http.ResponseWriter, r *http.Request) {
	const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LocalSend - Enter PIN</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f7; padding: 20px; color: #333; }
        .container { max-width: 400px; margin: 100px auto; background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); text-align: center; }
        h1 { font-size: 20px; margin-bottom: 20px; }
        input[type="password"] { width: 80%; padding: 10px; font-size: 16px; border: 1px solid #ccc; border-radius: 6px; margin-bottom: 20px; text-align: center; }
        button { background: #0071e3; color: white; border: none; padding: 10px 20px; border-radius: 6px; font-size: 15px; font-weight: 500; cursor: pointer; }
        button:hover { background: #0077ed; }
        .error { color: red; font-size: 14px; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Enter PIN to access files</h1>
        <form method="POST" action="/">
            <input type="password" name="pin" placeholder="PIN Code" required autocomplete="off" autofocus>
            <br>
            <button type="submit">Submit</button>
        </form>
        {{if .Error}}
        <p class="error">Invalid PIN code. Please try again.</p>
        {{end}}
    </div>
</body>
</html>`

	isPost := r.Method == http.MethodPost
	data := struct {
		Error bool
	}{
		Error: isPost,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.New("pinform").Parse(htmlTemplate)
	if err == nil {
		_ = tmpl.Execute(w, data)
	}
}
