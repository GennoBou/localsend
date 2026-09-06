package crypto

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated rand failure")
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
	origRand := rand.Reader
	rand.Reader = errReader{}
	defer func() { rand.Reader = origRand }()

	_, _, _, _, err := GenerateSelfSignedCert()
	if err == nil {
		t.Error("Expected error when rand.Reader fails, got nil")
	} else if !strings.Contains(err.Error(), "failed to generate") {
		t.Errorf("Unexpected error message: %v", err)
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
	if info == nil || info.Fingerprint == "" {
		t.Error("Expected valid CertificateInfo with fingerprint")
	}
}

func TestLoadOrGenerateCredentials_GenerateError(t *testing.T) {
	origRand := rand.Reader
	rand.Reader = errReader{}
	defer func() { rand.Reader = origRand }()

	_, err := LoadOrGenerateCredentials("", "")
	if err == nil {
		t.Error("Expected error when GenerateSelfSignedCert fails, got nil")
	} else if !strings.Contains(err.Error(), "failed to generate credentials") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoadOrGenerateCredentials_CorruptFilesFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")

	// Write corrupted files
	if err := os.WriteFile(certPath, []byte("invalid cert"), 0600); err != nil {
		t.Fatalf("Failed to write corrupt cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("invalid key"), 0600); err != nil {
		t.Fatalf("Failed to write corrupt key file: %v", err)
	}

	// LoadOrGenerateCredentials should fail loading corrupted files and fallback to generating new ones
	info, err := LoadOrGenerateCredentials(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateCredentials failed to fallback and generate new certs: %v", err)
	}
	if info == nil || info.Fingerprint == "" {
		t.Error("Expected valid CertificateInfo after fallback")
	}
}

func TestLoadOrGenerateCredentials_MkdirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a regular file where a directory should be
	filePathAsDir := filepath.Join(tempDir, "file_as_dir")
	if err := os.WriteFile(filePathAsDir, []byte("data"), 0600); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	certPath := filepath.Join(filePathAsDir, "sub", "cert.pem")
	keyPath := filepath.Join(filePathAsDir, "sub", "key.pem")

	_, err = LoadOrGenerateCredentials(certPath, keyPath)
	if err == nil {
		t.Error("Expected error when MkdirAll fails, got nil")
	} else if !strings.Contains(err.Error(), "failed to create directory for certificates") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoadOrGenerateCredentials_WriteCertError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Make certPath a directory so WriteFile fails
	certPathDir := filepath.Join(tempDir, "cert_dir")
	if err := os.Mkdir(certPathDir, 0700); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	keyPath := filepath.Join(tempDir, "key.pem")

	_, err = LoadOrGenerateCredentials(certPathDir, keyPath)
	if err == nil {
		t.Error("Expected error when writing cert file fails, got nil")
	} else if !strings.Contains(err.Error(), "failed to write certificate to file") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoadOrGenerateCredentials_WriteKeyError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "cert.pem")
	// Make keyPath a directory so WriteFile fails
	keyPathDir := filepath.Join(tempDir, "key_dir")
	if err := os.Mkdir(keyPathDir, 0700); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	_, err = LoadOrGenerateCredentials(certPath, keyPathDir)
	if err == nil {
		t.Error("Expected error when writing key file fails, got nil")
	} else if !strings.Contains(err.Error(), "failed to write key to file") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
