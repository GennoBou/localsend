package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GennoBou/localsend/pkg/crypto"
	"github.com/GennoBou/localsend/pkg/protocol"
)

// setupTestServer はテストに必要な Server インスタンスをセットアップします。
func setupTestServer(t *testing.T) (*Server, func()) {
	// 一時ディレクトリの作成
	tempDir, err := os.MkdirTemp("", "localsend-server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// ダミー証明書の生成
	tlsCert, _, _, _, err := crypto.GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("failed to generate TLS cert: %v", err)
	}

	myDevice := protocol.Device{
		Alias:       "Test Device",
		DeviceModel: "GoTest",
		DeviceType:  "desktop",
		Fingerprint: "dummy-fingerprint",
		Port:        53317,
		Protocol:    "http",
		Download:    true,
	}

	s := NewServer(myDevice, tlsCert, tempDir, "", false)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return s, cleanup
}

// createTempSharedFile はテスト用のダミーファイルを作成し、ShareFile 構造体を返します。
func createTempSharedFile(t *testing.T, dir, filename, content string) (ShareFile, func()) {
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	sf := ShareFile{
		ID:       "test-file-id-123",
		Path:     path,
		FileName: filename,
		Size:     int64(len(content)),
		FileType: "text/plain",
	}

	cleanup := func() {
		os.Remove(path)
	}

	return sf, cleanup
}

// TestWebShare_WebUI はブラウザ向け Web UI (HTML) の返却テストを行います。
func TestWebShare_WebUI(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	sf, cleanupFile := createTempSharedFile(t, s.saveDir, "テストファイル.txt", "hello world")
	defer cleanupFile()

	s.sharedFiles = []ShareFile{sf}
	s.downloadSession = "test-session-uuid"

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.handleWebUI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type to contain text/html, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "テストファイル.txt") {
		t.Error("response body does not contain shared filename")
	}
	if !strings.Contains(bodyStr, "test-session-uuid") {
		t.Error("response body does not contain download session ID")
	}
	if !strings.Contains(bodyStr, "Test Device") {
		t.Error("response body does not contain sender alias")
	}
}

// TestWebShare_PrepareDownload は POST /api/localsend/v2/prepare-download の挙動をテストします。
func TestWebShare_PrepareDownload(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	sf, cleanupFile := createTempSharedFile(t, s.saveDir, "test.txt", "content")
	defer cleanupFile()

	s.sharedFiles = []ShareFile{sf}
	s.downloadSession = "test-session-uuid"

	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-download", nil)
	w := httptest.NewRecorder()

	s.handlePrepareDownload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type to be application/json, got %s", contentType)
	}

	var jsonResp protocol.PrepareDownloadResponse
	err := json.NewDecoder(resp.Body).Decode(&jsonResp)
	if err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if jsonResp.SessionID != "test-session-uuid" {
		t.Errorf("expected sessionID 'test-session-uuid', got '%s'", jsonResp.SessionID)
	}

	meta, exists := jsonResp.Files[sf.ID]
	if !exists {
		t.Fatalf("expected file ID '%s' to exist in response, but not found", sf.ID)
	}

	if meta.FileName != sf.FileName {
		t.Errorf("expected filename '%s', got '%s'", sf.FileName, meta.FileName)
	}
	if meta.Size != sf.Size {
		t.Errorf("expected size %d, got %d", sf.Size, meta.Size)
	}
}

// TestWebShare_Download_Success は正常系のファイルダウンロードをテストします。
func TestWebShare_Download_Success(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	fileContent := "日本語テキストのダウンロードテストデータ。"
	sf, cleanupFile := createTempSharedFile(t, s.saveDir, "日本語テスト.txt", fileContent)
	defer cleanupFile()

	s.sharedFiles = []ShareFile{sf}
	s.downloadSession = "test-session-uuid"

	// 正しい sessionId と fileId をクエリパラメータに指定
	query := url.Values{}
	query.Set("sessionId", "test-session-uuid")
	query.Set("fileId", sf.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/localsend/v2/download?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	s.handleDownload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// 日本語ファイル名のエンコード検証
	contentDisp := resp.Header.Get("Content-Disposition")
	if !strings.Contains(contentDisp, "filename*=UTF-8''") {
		t.Errorf("expected Content-Disposition to use UTF-8 RFC 5987, got %s", contentDisp)
	}
	
	// UTF-8'' の後のエンコード部分をデコードして期待通りか検証
	parts := strings.Split(contentDisp, "filename*=UTF-8''")
	if len(parts) < 2 {
		t.Fatalf("invalid Content-Disposition header: %s", contentDisp)
	}
	decodedName, err := url.PathUnescape(parts[1])
	if err != nil {
		t.Fatalf("failed to decode filename from header: %v", err)
	}
	if decodedName != sf.FileName {
		t.Errorf("expected decoded filename '%s', got '%s'", sf.FileName, decodedName)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read downloaded content: %v", err)
	}

	if string(body) != fileContent {
		t.Errorf("expected content '%s', got '%s'", fileContent, string(body))
	}
}

// TestWebShare_Download_InvalidSession は不正なセッションでのダウンロード要求が拒否されることをテストします。
func TestWebShare_Download_InvalidSession(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	sf, cleanupFile := createTempSharedFile(t, s.saveDir, "test.txt", "content")
	defer cleanupFile()

	s.sharedFiles = []ShareFile{sf}
	s.downloadSession = "test-session-uuid"

	// 誤った sessionId を指定
	query := url.Values{}
	query.Set("sessionId", "wrong-session-uuid")
	query.Set("fileId", sf.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/localsend/v2/download?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	s.handleDownload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden, got %d", resp.StatusCode)
	}
}

// TestWebShare_Download_FileNotFound は存在しないファイルのダウンロード要求が 404 エラーになることをテストします。
func TestWebShare_Download_FileNotFound(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	sf, cleanupFile := createTempSharedFile(t, s.saveDir, "test.txt", "content")
	defer cleanupFile()

	s.sharedFiles = []ShareFile{sf}
	s.downloadSession = "test-session-uuid"

	// 存在しない fileId を指定
	query := url.Values{}
	query.Set("sessionId", "test-session-uuid")
	query.Set("fileId", "non-existent-file-id")

	req := httptest.NewRequest(http.MethodGet, "/api/localsend/v2/download?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	s.handleDownload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404 Not Found, got %d", resp.StatusCode)
	}
}

// TestWebShare_PINRequired は Web Share における PINコードの検証テストを行います。
func TestWebShare_PINRequired(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	// PIN を設定
	s.pin = "123456"

	sf, cleanupFile := createTempSharedFile(t, s.saveDir, "test.txt", "content")
	defer cleanupFile()

	s.sharedFiles = []ShareFile{sf}
	s.downloadSession = "test-session-uuid"

	// 1. PINなしで Web UI にアクセス -> PINフォームが返るはず
	reqUI := httptest.NewRequest(http.MethodGet, "/", nil)
	wUI := httptest.NewRecorder()
	s.handleWebUI(wUI, reqUI)
	respUI := wUI.Result()
	bodyUI, _ := io.ReadAll(respUI.Body)
	if !strings.Contains(string(bodyUI), "Enter PIN") {
		t.Error("expected PIN form when access without PIN cookie")
	}

	// 2. 正しいPINを POST 送信 -> Cookie が設定されてリダイレクトされるはず
	postForm := url.Values{}
	postForm.Set("pin", "123456")
	reqPost := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(postForm.Encode()))
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wPost := httptest.NewRecorder()
	s.handleWebUI(wPost, reqPost)
	respPost := wPost.Result()
	if respPost.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303 Redirect after successful PIN submit, got %d", respPost.StatusCode)
	}
	cookies := respPost.Cookies()
	var pinCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_pin" {
			pinCookie = c
			break
		}
	}
	if pinCookie == nil || pinCookie.Value != "123456" {
		t.Error("session_pin cookie is missing or incorrect after successful PIN submission")
	}

	// 3. 正しい Cookie を持って Web UI にアクセス -> 通常のダウンロード画面が返るはず
	reqWithCookie := httptest.NewRequest(http.MethodGet, "/", nil)
	reqWithCookie.AddCookie(pinCookie)
	wWithCookie := httptest.NewRecorder()
	s.handleWebUI(wWithCookie, reqWithCookie)
	respWithCookie := wWithCookie.Result()
	bodyWithCookie, _ := io.ReadAll(respWithCookie.Body)
	if strings.Contains(string(bodyWithCookie), "Enter PIN") {
		t.Error("should not show PIN form when valid PIN cookie is present")
	}

	// 4. API 経由での PIN 検証 (prepare-download) - PIN 不一致
	reqAPIWrong := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-download?pin=wrong", nil)
	wAPIWrong := httptest.NewRecorder()
	s.handlePrepareDownload(wAPIWrong, reqAPIWrong)
	if wAPIWrong.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for incorrect PIN via API, got %d", wAPIWrong.Result().StatusCode)
	}

	// 5. API 経由での PIN 検証 (prepare-download) - PIN 一致
	reqAPIOK := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-download?pin=123456", nil)
	wAPIOK := httptest.NewRecorder()
	s.handlePrepareDownload(wAPIOK, reqAPIOK)
	if wAPIOK.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for correct PIN via API, got %d", wAPIOK.Result().StatusCode)
	}
}

// TestUpload_Cancel_Cleanup はファイルアップロード中にセッションがキャンセルされた場合、
// 受信が中断され一時ファイルが削除されることをテストします。
func TestUpload_Cancel_Cleanup(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	// ダミーファイルメタデータ
	meta := protocol.FileMetadata{
		ID:       "cancel-file-id",
		FileName: "cancel-test.txt",
		Size:     1024 * 1024, // 1MB の大きめのファイル
		FileType: "text/plain",
	}
	filesMeta := map[string]protocol.FileMetadata{meta.ID: meta}

	sessionObj := s.sessionMgr.CreateSession("127.0.0.1", false, filesMeta)
	token, _, _ := sessionObj.GetFileTokenAndMeta(meta.ID)

	// パイプを使用して、データを少しずつ送るシミュレーション
	pr, pw := io.Pipe()

	query := url.Values{}
	query.Set("sessionId", sessionObj.ID)
	query.Set("fileId", meta.ID)
	query.Set("token", token)

	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/upload?"+query.Encode(), pr)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	errChan := make(chan error, 1)
	go func() {
		// handleUpload はブロックするのでゴルーチンで実行
		s.handleUpload(w, req)
		errChan <- nil
	}()

	// データを一部だけ書き込む
	_, _ = pw.Write([]byte("some initial data"))

	// 途中でセッションを削除（キャンセル）
	s.sessionMgr.DeleteSession(sessionObj.ID)

	// パイプを閉じてブロックを解除し、ループの先頭でキャンセルを検知させる
	_ = pw.Close()

	<-errChan

	// レスポンスが 400 Bad Request であることを確認
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request on canceled upload, got %d", resp.StatusCode)
	}

	// 保存先フォルダに一時ファイルが残っていないことを検証
	files, _ := os.ReadDir(s.saveDir)
	for _, f := range files {
		if strings.Contains(f.Name(), "cancel-test") {
			t.Errorf("temporary file '%s' was not deleted after cancel", f.Name())
		}
	}
}

// TestPrepareUpload_PartialAcceptance は送信準備時の部分承諾（一部のファイルのみ受け入れ）をテストします。
func TestPrepareUpload_PartialAcceptance(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	// 3つのファイルを送信しようとする
	files := map[string]protocol.FileMetadata{
		"file1": {ID: "file1", FileName: "accept.txt", Size: 100},
		"file2": {ID: "file2", FileName: "reject.txt", Size: 200},
		"file3": {ID: "file3", FileName: "accept2.txt", Size: 300},
	}

	// コールバックで file1 と file3 のみ承諾する
	s.OnPrepareUpload = func(sender protocol.Device, files []protocol.FileMetadata) (map[string]bool, bool) {
		accepted := map[string]bool{
			"file1": true,
			"file2": false, // 明示的拒否
			"file3": true,
		}
		return accepted, true
	}

	reqBody := protocol.PrepareUploadRequest{
		Info:  s.myDevice,
		Files: files,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-upload", strings.NewReader(string(bodyBytes)))
	w := httptest.NewRecorder()

	s.handlePrepareUpload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	var jsonResp protocol.PrepareUploadResponse
	_ = json.NewDecoder(resp.Body).Decode(&jsonResp)

	// 承諾されたファイルのみトークンが返ってきているか検証
	if _, ok := jsonResp.Files["file1"]; !ok {
		t.Error("expected file1 to be accepted")
	}
	if _, ok := jsonResp.Files["file2"]; ok {
		t.Error("expected file2 to be rejected (no token should be returned)")
	}
	if _, ok := jsonResp.Files["file3"]; !ok {
		t.Error("expected file3 to be accepted")
	}
}

// TestPrepareUpload_204NoContent はすべてのファイルが重複している場合に 204 No Content が返ることをテストします。
func TestPrepareUpload_204NoContent(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	// 保存先にすでにあるファイルをモック作成
	filename := "duplicate.txt"
	content := "existing content"
	sf, cleanupFile := createTempSharedFile(t, s.saveDir, filename, content)
	defer cleanupFile()

	// 同一名・同一サイズで送信準備を投げる
	files := map[string]protocol.FileMetadata{
		"file-dup": {ID: "file-dup", FileName: filename, Size: sf.Size},
	}

	reqBody := protocol.PrepareUploadRequest{
		Info:  s.myDevice,
		Files: files,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-upload", strings.NewReader(string(bodyBytes)))
	w := httptest.NewRecorder()

	s.handlePrepareUpload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content for duplicate files, got %d", resp.StatusCode)
	}
}

// TestHandleUpload_ChecksumValidation tests SHA-256 verification (Protocol v2.2)
func TestHandleUpload_ChecksumValidation(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		metaSha256     string
		expectedStatus int
		expectSaved    bool
	}{
		{
			name:           "Valid SHA-256 checksum matches",
			content:        "hello world",
			metaSha256:     "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", // sha256 of "hello world"
			expectedStatus: http.StatusOK,
			expectSaved:    true,
		},
		{
			name:           "Invalid SHA-256 checksum returns 422",
			content:        "hello world",
			metaSha256:     "0000000000000000000000000000000000000000000000000000000000000000",
			expectedStatus: http.StatusUnprocessableEntity,
			expectSaved:    false,
		},
		{
			name:           "Empty SHA-256 skips check and succeeds",
			content:        "hello world",
			metaSha256:     "",
			expectedStatus: http.StatusOK,
			expectSaved:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, cleanupServer := setupTestServer(t)
			defer cleanupServer()

			fileID := "test-file-" + t.Name()
			fileName := "test-" + filepath.Base(t.Name()) + ".txt"

			meta := protocol.FileMetadata{
				ID:       fileID,
				FileName: fileName,
				Size:     int64(len(tt.content)),
				Sha256:   tt.metaSha256,
				FileType: "text/plain",
			}
			filesMeta := map[string]protocol.FileMetadata{meta.ID: meta}

			sessionObj := s.sessionMgr.CreateSession("127.0.0.1", false, filesMeta)
			token, _, _ := sessionObj.GetFileTokenAndMeta(fileID)

			query := url.Values{}
			query.Set("sessionId", sessionObj.ID)
			query.Set("fileId", fileID)
			query.Set("token", token)

			req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/upload?"+query.Encode(), strings.NewReader(tt.content))
			req.RemoteAddr = "127.0.0.1:12345"
			w := httptest.NewRecorder()

			s.handleUpload(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			savedFile := filepath.Join(s.saveDir, fileName)
			_, err := os.Stat(savedFile)
			if tt.expectSaved && os.IsNotExist(err) {
				t.Errorf("expected file %s to be saved, but does not exist", savedFile)
			} else if !tt.expectSaved && !os.IsNotExist(err) {
				t.Errorf("expected file %s to be deleted on failure, but it exists", savedFile)
			}
		})
	}
}

// TestNewServer tests the initialization of Server struct by NewServer.
func TestNewServer(t *testing.T) {
	tlsCert, _, _, _, err := crypto.GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("failed to generate TLS cert: %v", err)
	}

	myDevice := protocol.Device{
		Alias:       "Test Server Device",
		DeviceModel: "TestModel",
		DeviceType:  "desktop",
		Fingerprint: "test-fingerprint-123",
		Port:        53317,
		Protocol:    "https",
		Download:    true,
	}

	saveDir := "/tmp/test-savedir"
	pin := "123456"
	strictTLS := true

	srv := NewServer(myDevice, tlsCert, saveDir, pin, strictTLS)

	if srv == nil {
		t.Fatal("expected NewServer to return non-nil Server instance")
	}

	if srv.myDevice.Alias != myDevice.Alias || srv.myDevice.Fingerprint != myDevice.Fingerprint {
		t.Errorf("expected myDevice %v, got %v", myDevice, srv.myDevice)
	}

	if srv.saveDir != saveDir {
		t.Errorf("expected saveDir %s, got %s", saveDir, srv.saveDir)
	}

	if srv.pin != pin {
		t.Errorf("expected pin %s, got %s", pin, srv.pin)
	}

	if srv.strictTLS != strictTLS {
		t.Errorf("expected strictTLS %v, got %v", strictTLS, srv.strictTLS)
	}

	if srv.sessionMgr == nil {
		t.Error("expected sessionMgr to be initialized, got nil")
	}
}
