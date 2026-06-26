package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/GennoBou/localsend/pkg/protocol"
)

// UploadSession manages the state of a single file transfer session.
type UploadSession struct {
	ID             string
	ClientIP       string
	ClientVerified bool
	LastAccess     time.Time
	Files          map[string]string                // fileID -> token
	FilesMetadata  map[string]protocol.FileMetadata // fileID -> Metadata
	Progress       map[string]int64                 // fileID -> transferred bytes
	mu             sync.RWMutex
}

// NewUploadSession creates a new transfer session.
func NewUploadSession(clientIP string, clientVerified bool, filesMetadata map[string]protocol.FileMetadata) *UploadSession {
	files := make(map[string]string)
	progress := make(map[string]int64)

	for id := range filesMetadata {
		// Generate upload token for each file with UUID
		files[id] = uuid.NewString()
		progress[id] = 0
	}

	return &UploadSession{
		ID:             uuid.NewString(),
		ClientIP:       clientIP,
		ClientVerified: clientVerified,
		LastAccess:     time.Now(),
		Files:          files,
		FilesMetadata:  filesMetadata,
		Progress:       progress,
	}
}

// Touch updates the last access time to the current time.
func (s *UploadSession) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastAccess = time.Now()
}

// UpdateProgress updates the transfer progress of the specified file.
func (s *UploadSession) UpdateProgress(fileID string, bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Progress[fileID]; ok {
		s.Progress[fileID] = bytes
	}
	s.LastAccess = time.Now()
}

// GetFileTokenAndMeta thread-safely retrieves the token and metadata corresponding to the specified file ID.
func (s *UploadSession) GetFileTokenAndMeta(fileID string) (string, protocol.FileMetadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, tokenOk := s.Files[fileID]
	meta, metaOk := s.FilesMetadata[fileID]
	return token, meta, tokenOk && metaOk
}

// GetClientInfo thread-safely retrieves the client's IP and verification status of the session.
func (s *UploadSession) GetClientInfo() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ClientIP, s.ClientVerified
}

// SessionManager thread-safely manages active sessions.
type SessionManager struct {
	sessions sync.Map // string (sessionID) -> *UploadSession
	timeout  time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewSessionManager creates a new session manager and starts the background cleanup loop.
func NewSessionManager(timeout time.Duration) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &SessionManager{
		timeout: timeout,
		ctx:     ctx,
		cancel:  cancel,
	}
	go m.startCleanupLoop(1 * time.Minute)
	return m
}

// Close terminates the session manager and stops the cleanup loop.
func (m *SessionManager) Close() {
	m.cancel()
}

// CreateSession adds a new session and returns it.
func (m *SessionManager) CreateSession(clientIP string, clientVerified bool, filesMetadata map[string]protocol.FileMetadata) *UploadSession {
	session := NewUploadSession(clientIP, clientVerified, filesMetadata)
	m.sessions.Store(session.ID, session)
	return session
}

// GetSession retrieves the session by the specified ID and updates its LastAccess.
func (m *SessionManager) GetSession(id string) (*UploadSession, bool) {
	val, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	session := val.(*UploadSession)
	session.Touch()
	return session, true
}

// DeleteSession deletes the session by the specified ID.
func (m *SessionManager) DeleteSession(id string) {
	m.sessions.Delete(id)
}

// IsBusy returns whether there is any currently active session.
func (m *SessionManager) IsBusy() bool {
	busy := false
	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*UploadSession)
		session.mu.RLock()
		// Regard as busy if a session exists within the expiration period (time since last access is less than timeout)
		if time.Since(session.LastAccess) < m.timeout {
			busy = true
			session.mu.RUnlock()
			return false // Break the loop
		}
		session.mu.RUnlock()
		return true
	})
	return busy
}

// startCleanupLoop periodically discards expired sessions.
func (m *SessionManager) startCleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			m.sessions.Range(func(key, value interface{}) bool {
				session := value.(*UploadSession)
				session.mu.RLock()
				lastAccess := session.LastAccess
				session.mu.RUnlock()

				if now.Sub(lastAccess) > m.timeout {
					m.sessions.Delete(key)
				}
				return true
			})
		}
	}
}

// IsCompleted returns whether the upload of all files in the session is completed.
func (s *UploadSession) IsCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, meta := range s.FilesMetadata {
		progress, ok := s.Progress[id]
		if !ok || progress < meta.Size {
			return false
		}
	}
	return true
}
