package session

import (
	"sync"
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
	// Short timeout (50ms) for testing
	mgr := NewSessionManager(50 * time.Millisecond)
	defer mgr.Close()

	files := map[string]protocol.FileMetadata{
		"file1": {ID: "file1", FileName: "test.txt", Size: 100},
	}

	// 1. Create session
	sess := mgr.CreateSession("192.168.1.100", false, files)
	if sess == nil {
		t.Fatal("Failed to create session")
	}

	// 2. Retrieve session
	retrieved, ok := mgr.GetSession(sess.ID)
	if !ok || retrieved.ID != sess.ID {
		t.Error("Failed to retrieve session")
	}

	// 3. Busy status check
	if !mgr.IsBusy() {
		t.Error("Manager should be busy with active session")
	}

	// 4. Update progress
	sess.UpdateProgress("file1", 50)
	if sess.Progress["file1"] != 50 {
		t.Errorf("Progress not updated. Expected: 50, Got: %d", sess.Progress["file1"])
	}

	// 5. Delete session
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

	// Wait for expiration
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

func TestUploadSession_Methods(t *testing.T) {
	meta := protocol.FileMetadata{
		ID:       "file-alpha",
		FileName: "alpha.txt",
		Size:     500,
	}
	sess := NewUploadSession("10.0.0.42", true, map[string]protocol.FileMetadata{meta.ID: meta})

	// GetClientInfo
	ip, verified := sess.GetClientInfo()
	if ip != "10.0.0.42" || !verified {
		t.Errorf("GetClientInfo() = (%q, %v), want ('10.0.0.42', true)", ip, verified)
	}

	// GetFileTokenAndMeta - Existing
	token, retrievedMeta, ok := sess.GetFileTokenAndMeta(meta.ID)
	if !ok || token == "" || retrievedMeta.FileName != "alpha.txt" {
		t.Errorf("GetFileTokenAndMeta(valid) failed: token=%q, ok=%v, meta=%+v", token, ok, retrievedMeta)
	}

	// GetFileTokenAndMeta - Non-existent
	_, _, okNotFound := sess.GetFileTokenAndMeta("non-existent-id")
	if okNotFound {
		t.Errorf("GetFileTokenAndMeta(non-existent) expected false, got true")
	}

	// Context & Cancel
	if sess.IsCanceled() {
		t.Errorf("expected IsCanceled() = false initially")
	}
	if sess.Context() == nil {
		t.Errorf("expected non-nil Context()")
	}

	sess.Cancel()
	if !sess.IsCanceled() {
		t.Errorf("expected IsCanceled() = true after Cancel()")
	}
}

func TestSessionManager_Close(t *testing.T) {
	mgr := NewSessionManager(100 * time.Millisecond)
	// Multiple Close calls should not panic
	mgr.Close()
	mgr.Close()
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	mgr := NewSessionManager(500 * time.Millisecond)
	defer mgr.Close()

	const numGoroutines = 20
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			meta := protocol.FileMetadata{
				ID:       "concurrent-file",
				FileName: "test.dat",
				Size:     1000,
			}
			sess := mgr.CreateSession("127.0.0.1", true, map[string]protocol.FileMetadata{meta.ID: meta})
			if sess == nil {
				t.Errorf("goroutine %d failed to create session", idx)
				return
			}

			_ = mgr.IsBusy()

			sess.UpdateProgress("concurrent-file", 500)

			retrieved, ok := mgr.GetSession(sess.ID)
			if !ok || retrieved == nil {
				t.Errorf("goroutine %d failed to get session %s", idx, sess.ID)
			}

			sess.UpdateProgress("concurrent-file", 1000)
			_ = sess.IsCompleted()

			mgr.DeleteSession(sess.ID)
		}(i)
	}

	wg.Wait()
}
