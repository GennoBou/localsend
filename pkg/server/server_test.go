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
// TestNewServer は NewServer 関数のフィールド初期化をテストします。
func TestNewServer(t *testing.T) {
	tlsCert, _, _, _, err := crypto.GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("failed to generate TLS cert: %v", err)
	}

	tests := []struct {
		name      string
		myDevice  protocol.Device
		saveDir   string
		pin       string
		strictTLS bool
	}{
		{
			name: "Standard initialization",
			myDevice: protocol.Device{
				Alias:       "Device 1",
				DeviceModel: "Model A",
				DeviceType:  "mobile",
				Fingerprint: "fp1",
				Port:        53317,
				Protocol:    "https",
				Download:    true,
			},
			saveDir:   "/tmp/save1",
			pin:       "1234",
			strictTLS: true,
		},
		{
			name: "Initialization with empty PIN and strictTLS false",
			myDevice: protocol.Device{
				Alias:       "Device 2",
				DeviceModel: "Model B",
				DeviceType:  "desktop",
				Fingerprint: "fp2",
				Port:        53318,
				Protocol:    "http",
				Download:    false,
			},
			saveDir:   "/tmp/save2",
			pin:       "",
			strictTLS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(tt.myDevice, tlsCert, tt.saveDir, tt.pin, tt.strictTLS)

			if s == nil {
				t.Fatal("expected non-nil Server")
			}
			if s.myDevice != tt.myDevice {
				t.Errorf("expected myDevice %+v, got %+v", tt.myDevice, s.myDevice)
			}
			if len(s.tlsCert.Certificate) != len(tlsCert.Certificate) {
				t.Errorf("expected tlsCert len %d, got %d", len(tlsCert.Certificate), len(s.tlsCert.Certificate))
			}
			if s.saveDir != tt.saveDir {
				t.Errorf("expected saveDir %s, got %s", tt.saveDir, s.saveDir)
			}
			if s.pin != tt.pin {
				t.Errorf("expected pin %s, got %s", tt.pin, s.pin)
			}
			if s.strictTLS != tt.strictTLS {
				t.Errorf("expected strictTLS %v, got %v", tt.strictTLS, s.strictTLS)
			}
			if s.sessionMgr == nil {
				t.Error("expected sessionMgr to be initialized, got nil")
			}
		})
	}
}

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

// TestPathTraversal Prevention tests that filenames with path traversal elements do not escape s.saveDir.
func TestPathTraversalPrevention(t *testing.T) {
	s, cleanupServer := setupTestServer(t)
	defer cleanupServer()

	// Parent directory of saveDir
	parentDir := filepath.Dir(s.saveDir)
	targetFileNameInParent := "traversal_test_file.txt"
	targetFilePathInParent := filepath.Join(parentDir, targetFileNameInParent)

	// Clean up if created
	defer os.Remove(targetFilePathInParent)

	// Malicious filename attempting to write to parent directory
	traversalFileName := "../" + targetFileNameInParent

	fileID := "traversal-file-id"
	content := "traversal test content"

	meta := protocol.FileMetadata{
		ID:       fileID,
		FileName: traversalFileName,
		Size:     int64(len(content)),
		FileType: "text/plain",
	}
	filesMeta := map[string]protocol.FileMetadata{meta.ID: meta}

	// Test 1: prepare-upload duplicate check with path traversal filename
	// Create the file in parent directory first to test if duplicate check accesses it
	err := os.WriteFile(targetFilePathInParent, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create file in parent dir: %v", err)
	}

	reqBody := protocol.PrepareUploadRequest{
		Info:  s.myDevice,
		Files: map[string]protocol.FileMetadata{"file1": meta},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	reqPrep := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-upload", strings.NewReader(string(bodyBytes)))
	wPrep := httptest.NewRecorder()
	s.handlePrepareUpload(wPrep, reqPrep)

	// Since targetFilePathInParent is in parentDir, not saveDir, duplicate check must NOT find it (should return 200 OK with session, not 204 No Content)
	if wPrep.Result().StatusCode == http.StatusNoContent {
		t.Errorf("path traversal allowed duplicate check to access outside saveDir")
	}

	// Remove target file in parent dir to test upload creation
	_ = os.Remove(targetFilePathInParent)

	// Test 2: upload with path traversal filename
	sessionObj := s.sessionMgr.CreateSession("127.0.0.1", false, filesMeta)
	token, _, _ := sessionObj.GetFileTokenAndMeta(fileID)

	query := url.Values{}
	query.Set("sessionId", sessionObj.ID)
	query.Set("fileId", fileID)
	query.Set("token", token)

	reqUpload := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/upload?"+query.Encode(), strings.NewReader(content))
	reqUpload.RemoteAddr = "127.0.0.1:12345"
	wUpload := httptest.NewRecorder()

	s.handleUpload(wUpload, reqUpload)

	if wUpload.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected upload status 200 OK, got %d", wUpload.Result().StatusCode)
	}

	// Check file was NOT created in parent directory
	if _, err := os.Stat(targetFilePathInParent); err == nil {
		t.Errorf("path traversal vulnerability! File was written outside saveDir to %s", targetFilePathInParent)
	}

	// Check file WAS created inside saveDir with sanitized name
	sanitizedPath := filepath.Join(s.saveDir, targetFileNameInParent)
	if _, err := os.Stat(sanitizedPath); os.IsNotExist(err) {
		t.Errorf("expected file to be saved in saveDir as %s, but not found", sanitizedPath)
	}
}
