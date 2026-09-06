package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

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
	// Empty certPath or keyPath generates credentials in memory without persisting
	info, err := LoadOrGenerateCredentials("", "")
	if err != nil {
		t.Fatalf("LoadOrGenerateCredentials(\"\", \"\") failed: %v", err)
	}
	if info == nil || info.Fingerprint == "" {
		t.Error("Expected valid CertificateInfo in in-memory mode")
	}
}

func TestLoadOrGenerateCredentials_InvalidDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file where directory should be
	conflictFile := filepath.Join(tempDir, "file_blocking_dir")
	if err := os.WriteFile(conflictFile, []byte("block"), 0600); err != nil {
		t.Fatalf("Failed to create conflict file: %v", err)
	}

	certPath := filepath.Join(conflictFile, "cert.pem")
	keyPath := filepath.Join(conflictFile, "key.pem")

	_, err = LoadOrGenerateCredentials(certPath, keyPath)
	if err == nil {
		t.Error("Expected error when MkdirAll fails due to existing file blocking directory path")
	}
}

func TestLoadOrGenerateCredentials_WriteFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Make certPath a directory so WriteFile fails
	certDir := filepath.Join(tempDir, "cert.pem")
	if err := os.Mkdir(certDir, 0700); err != nil {
		t.Fatalf("Failed to create directory for certPath: %v", err)
	}
	keyPath := filepath.Join(tempDir, "key.pem")

	_, err = LoadOrGenerateCredentials(certDir, keyPath)
	if err == nil {
		t.Error("Expected error when WriteFile fails on certPath")
	}
}

func TestLoadOrGenerateCredentials_CorruptedFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")

	// Write invalid/corrupted certificate and key content
	if err := os.WriteFile(certPath, []byte("invalid cert"), 0600); err != nil {
		t.Fatalf("Failed to write invalid cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("invalid key"), 0600); err != nil {
		t.Fatalf("Failed to write invalid key: %v", err)
	}

	// Should fallback to generating new credentials and overwriting
	info, err := LoadOrGenerateCredentials(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateCredentials failed on corrupted files fallback: %v", err)
	}
	if info == nil || info.Fingerprint == "" {
		t.Error("Expected valid CertificateInfo after fallback generation")
	}
}
