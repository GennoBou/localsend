package session

import (
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
	"github.com/google/uuid"
)

func TestUploadSession_IsCompleted(t *testing.T) {
	files := map[string]protocol.FileMetadata{
		"file1": {ID: "file1", FileName: "test1.txt", Size: 100},
		"file2": {ID: "file2", FileName: "test2.txt", Size: 200},
		"file3": {ID: "file3", FileName: "empty.txt", Size: 0},
	}

	sess := NewUploadSession("127.0.0.1", true, files)

	// Initially not completed
	if sess.IsCompleted() {
		t.Errorf("Expected IsCompleted to be false initially")
	}

	// Partially complete file1
	sess.UpdateProgress("file1", 50)
	if sess.IsCompleted() {
		t.Errorf("Expected IsCompleted to be false when file1 is partial")
	}

	// Fully complete file1
	sess.UpdateProgress("file1", 100)
	if sess.IsCompleted() {
		t.Errorf("Expected IsCompleted to be false when only file1 is completed")
	}

	// Over-complete file1
	sess.UpdateProgress("file1", 150)
	if sess.IsCompleted() {
		t.Errorf("Expected IsCompleted to be false when file1 is over-completed")
	}

	// Fully complete file2
	sess.UpdateProgress("file2", 200)
	if !sess.IsCompleted() {
		t.Errorf("Expected IsCompleted to be true when all files (including 0-byte file3) are completed")
	}

	// Reduce progress of file1 below size
	sess.UpdateProgress("file1", 80)
	if sess.IsCompleted() {
		t.Errorf("Expected IsCompleted to be false after file1 progress dropped below size")
	}
}

func TestSessionManager(t *testing.T) {
	// テスト用に短いタイムアウト（50ms）を設定
	mgr := NewSessionManager(50 * time.Millisecond)
	defer mgr.Close()

	files := map[string]protocol.FileMetadata{
		"file1": {ID: "file1", FileName: "test.txt", Size: 100},
	}

	// 1. セッションの作成
	sess := mgr.CreateSession("192.168.1.100", false, files)
	if sess == nil {
		t.Fatal("Failed to create session")
	}

	// 2. セッションの取得
	retrieved, ok := mgr.GetSession(sess.ID)
	if !ok || retrieved.ID != sess.ID {
		t.Error("Failed to retrieve session")
	}

	// 3. ビジー状態の確認
	if !mgr.IsBusy() {
		t.Error("Manager should be busy with active session")
	}

	// 4. 進捗の更新
	sess.UpdateProgress("file1", 50)
	if sess.Progress["file1"] != 50 {
		t.Errorf("Progress not updated. Expected: 50, Got: %d", sess.Progress["file1"])
	}

	// 5. セッション削除
	mgr.DeleteSession(sess.ID)
	if _, ok := mgr.GetSession(sess.ID); ok {
		t.Error("Session should have been deleted")
	}

	if mgr.IsBusy() {
		t.Error("Manager should not be busy after deleting the session")
	}
}

func TestSessionTimeout(t *testing.T) {
	mgr := NewSessionManager(50 * time.Millisecond)
	defer mgr.Close()

	files := map[string]protocol.FileMetadata{
		"file1": {ID: "file1", FileName: "test.txt", Size: 100},
	}

	_ = mgr.CreateSession("192.168.1.100", false, files)

	if !mgr.IsBusy() {
		t.Error("Manager should be busy initially")
	}

	// タイムアウトを待つ
	time.Sleep(100 * time.Millisecond)

	if mgr.IsBusy() {
		t.Error("Manager should not be busy after session expires")
	}
}

func TestNewUploadSession(t *testing.T) {
	t.Run("with files metadata", func(t *testing.T) {
		clientIP := "192.168.1.50"
		clientVerified := true
		filesMeta := map[string]protocol.FileMetadata{
			"file1": {ID: "file1", FileName: "doc.pdf", Size: 2048},
			"file2": {ID: "file2", FileName: "img.png", Size: 4096},
		}

		before := time.Now()
		sess := NewUploadSession(clientIP, clientVerified, filesMeta)
		after := time.Now()

		if sess == nil {
			t.Fatal("Expected non-nil UploadSession")
		}

		if _, err := uuid.Parse(sess.ID); err != nil {
			t.Errorf("Expected valid UUID for session ID, got %q: %v", sess.ID, err)
		}

		if sess.ClientIP != clientIP {
			t.Errorf("Expected ClientIP %q, got %q", clientIP, sess.ClientIP)
		}

		if sess.ClientVerified != clientVerified {
			t.Errorf("Expected ClientVerified %v, got %v", clientVerified, sess.ClientVerified)
		}

		if sess.LastAccess.Before(before) || sess.LastAccess.After(after) {
			t.Errorf("Expected LastAccess between %v and %v, got %v", before, after, sess.LastAccess)
		}

		if len(sess.FilesMetadata) != len(filesMeta) {
			t.Errorf("Expected %d FilesMetadata entries, got %d", len(filesMeta), len(sess.FilesMetadata))
		}

		for id := range filesMeta {
			token, ok := sess.Files[id]
			if !ok {
				t.Errorf("Expected file token for %q", id)
			} else if _, err := uuid.Parse(token); err != nil {
				t.Errorf("Expected valid UUID for file token %q, got %q: %v", id, token, err)
			}

			prog, ok := sess.Progress[id]
			if !ok {
				t.Errorf("Expected progress entry for %q", id)
			} else if prog != 0 {
				t.Errorf("Expected initial progress 0 for %q, got %d", id, prog)
			}
		}
	})

	t.Run("with empty files metadata", func(t *testing.T) {
		clientIP := "10.0.0.1"
		clientVerified := false
		filesMeta := map[string]protocol.FileMetadata{}

		sess := NewUploadSession(clientIP, clientVerified, filesMeta)
		if sess == nil {
			t.Fatal("Expected non-nil UploadSession")
		}

		if _, err := uuid.Parse(sess.ID); err != nil {
			t.Errorf("Expected valid UUID for session ID, got %q: %v", sess.ID, err)
		}

		if sess.ClientIP != clientIP {
			t.Errorf("Expected ClientIP %q, got %q", clientIP, sess.ClientIP)
		}

		if sess.ClientVerified != clientVerified {
			t.Errorf("Expected ClientVerified %v, got %v", clientVerified, sess.ClientVerified)
		}

		if len(sess.Files) != 0 {
			t.Errorf("Expected empty Files map, got len %d", len(sess.Files))
		}

		if len(sess.Progress) != 0 {
			t.Errorf("Expected empty Progress map, got len %d", len(sess.Progress))
		}
	})
}
