package crypto

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (int, error) {
	return 0, e.err
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

func TestGenerateSelfSignedCert_ErrorHandling(t *testing.T) {
	origRand := rand.Reader
	defer func() { rand.Reader = origRand }()

	rand.Reader = &errorReader{err: errors.New("simulated rand error")}
	_, _, _, _, err := GenerateSelfSignedCert()
	if err == nil {
		t.Fatal("Expected error from GenerateSelfSignedCert when rand.Reader fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to generate serial number") && !strings.Contains(err.Error(), "failed to generate private key") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestCalculateFingerprint(t *testing.T) {
	der := []byte("test certificate data")
	fp := CalculateFingerprint(der)
	if fp == "" {
		t.Error("Calculated fingerprint should not be empty")
	}
	if len(fp) != 64 { // SHA-256 in hex is 64 characters
		t.Errorf("Expected fingerprint length 64, got %d (%s)", len(fp), fp)
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
		t.Fatalf("Expected successful in-memory credential generation, got: %v", err)
	}
	if info == nil || info.Fingerprint == "" {
		t.Error("Expected valid CertificateInfo with non-empty fingerprint")
	}
}

func TestLoadOrGenerateCredentials_CorruptedFilesFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")

	// Write invalid cert and key files
	if err := os.WriteFile(certPath, []byte("invalid cert"), 0600); err != nil {
		t.Fatalf("Failed to write invalid cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("invalid key"), 0600); err != nil {
		t.Fatalf("Failed to write invalid key file: %v", err)
	}

	// LoadOrGenerateCredentials should fail loading, fall back to generation, and overwrite invalid files
	info, err := LoadOrGenerateCredentials(certPath, keyPath)
	if err != nil {
		t.Fatalf("Expected LoadOrGenerateCredentials to regenerate credentials after load failure, got: %v", err)
	}
	if info == nil || info.Fingerprint == "" {
		t.Error("Expected valid CertificateInfo after fallback generation")
	}
}

func TestLoadOrGenerateCredentials_Errors(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("failed to generate credentials", func(t *testing.T) {
		origRand := rand.Reader
		defer func() { rand.Reader = origRand }()
		rand.Reader = &errorReader{err: errors.New("simulated rand error")}

		_, err := LoadOrGenerateCredentials("", "")
		if err == nil || !strings.Contains(err.Error(), "failed to generate credentials") {
			t.Errorf("Expected credentials generation error, got: %v", err)
		}
	})

	t.Run("failed to create directory for certificates", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "regular_file")
		if err := os.WriteFile(filePath, []byte("data"), 0600); err != nil {
			t.Fatalf("Failed to create dummy file: %v", err)
		}

		// certPath inside a regular file path causes MkdirAll to fail
		certPath := filepath.Join(filePath, "subdir", "cert.pem")
		keyPath := filepath.Join(tempDir, "key.pem")

		_, err := LoadOrGenerateCredentials(certPath, keyPath)
		if err == nil || !strings.Contains(err.Error(), "failed to create directory for certificates") {
			t.Errorf("Expected directory creation error, got: %v", err)
		}
	})

	t.Run("failed to write certificate to file", func(t *testing.T) {
		// Use tempDir itself as certPath, which will cause WriteFile to fail because it's a directory
		certPath := tempDir
		keyPath := filepath.Join(tempDir, "key.pem")

		_, err := LoadOrGenerateCredentials(certPath, keyPath)
		if err == nil || !strings.Contains(err.Error(), "failed to write certificate to file") {
			t.Errorf("Expected certificate write error, got: %v", err)
		}
	})

	t.Run("failed to write key to file", func(t *testing.T) {
		certPath := filepath.Join(tempDir, "valid_cert.pem")
		// Use tempDir itself as keyPath, which will cause WriteFile to fail because it's a directory
		keyPath := tempDir

		_, err := LoadOrGenerateCredentials(certPath, keyPath)
		if err == nil || !strings.Contains(err.Error(), "failed to write key to file") {
			t.Errorf("Expected key write error, got: %v", err)
		}
	})
}
