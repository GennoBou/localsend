package crypto

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type errorRandReader struct{}

func (e *errorRandReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated random failure")
}

func TestGenerateSelfSignedCert(t *testing.T) {
	tlsCert, parsedCert, certPEM, keyPEM, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}

	if len(tlsCert.Certificate) == 0 {
		t.Error("TLS cert has no certificates")
	}

	if parsedCert == nil {
		t.Error("Parsed cert is nil")
	}

	if len(certPEM) == 0 {
		t.Error("Cert PEM is empty")
	}

	if len(keyPEM) == 0 {
		t.Error("Key PEM is empty")
	}
}

func TestGenerateSelfSignedCert_Error(t *testing.T) {
	origReader := rand.Reader
	defer func() { rand.Reader = origReader }()

	rand.Reader = &errorRandReader{}

	_, _, _, _, err := GenerateSelfSignedCert()
	if err == nil {
		t.Error("Expected error when random reader fails, got nil")
	}
}

func TestLoadOrGenerateCredentials(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")

	// 1. First run: Generates new credentials
	info1, err := LoadOrGenerateCredentials(certPath, keyPath)
	if err != nil {
		t.Fatalf("First run (generation) failed: %v", err)
	}

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Error("Certificate file was not written")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file was not written")
	}

	// 2. Second run: Expect to load from disk
	info2, err := LoadOrGenerateCredentials(certPath, keyPath)
	if err != nil {
		t.Fatalf("Second run (loading) failed: %v", err)
	}

	if info1.Fingerprint != info2.Fingerprint {
		t.Errorf("Fingerprints mismatch. Expected same fingerprint from cached files. Expected: %s, Got: %s", info1.Fingerprint, info2.Fingerprint)
	}
}

func TestLoadOrGenerateCredentials_InMemory(t *testing.T) {
	info, err := LoadOrGenerateCredentials("", "")
	if err != nil {
		t.Fatalf("LoadOrGenerateCredentials(\"\", \"\") failed: %v", err)
	}
	if info == nil {
		t.Fatal("Expected non-nil CertificateInfo")
	}
	if info.Fingerprint == "" {
		t.Error("Expected non-empty fingerprint")
	}
}

func TestLoadOrGenerateCredentials_InvalidExistingFilesFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")

	// Write invalid data to files
	if err := os.WriteFile(certPath, []byte("invalid cert pem"), 0600); err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("invalid key pem"), 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	// Should fallback to generating new valid credentials
	info, err := LoadOrGenerateCredentials(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateCredentials should fallback and succeed, got err: %v", err)
	}
	if info == nil {
		t.Fatal("Expected non-nil CertificateInfo")
	}
}

func TestLoadOrGenerateCredentials_MkdirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file where directory should be
	conflictFilePath := filepath.Join(tempDir, "conflict")
	if err := os.WriteFile(conflictFilePath, []byte("file"), 0600); err != nil {
		t.Fatalf("Failed to write conflict file: %v", err)
	}

	certPath := filepath.Join(conflictFilePath, "sub", "cert.pem")
	keyPath := filepath.Join(conflictFilePath, "sub", "key.pem")

	_, err = LoadOrGenerateCredentials(certPath, keyPath)
	if err == nil {
		t.Error("Expected error when directory creation fails, got nil")
	}
}

func TestLoadOrGenerateCredentials_WriteFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0500); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}

	certPath := filepath.Join(readOnlyDir, "cert.pem")
	keyPath := filepath.Join(readOnlyDir, "key.pem")

	_, err = LoadOrGenerateCredentials(certPath, keyPath)
	if err == nil {
		t.Error("Expected error when writing file to read-only dir fails, got nil")
	}
}

func TestLoadOrGenerateCredentials_GenerateError(t *testing.T) {
	origReader := rand.Reader
	defer func() { rand.Reader = origReader }()

	rand.Reader = &errorRandReader{}

	_, err := LoadOrGenerateCredentials("", "")
	if err == nil {
		t.Error("Expected error when generation fails inside LoadOrGenerateCredentials, got nil")
	}
}
