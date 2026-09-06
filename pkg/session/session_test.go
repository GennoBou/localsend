package session

import (
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
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
