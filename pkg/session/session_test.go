package session

import (
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

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
