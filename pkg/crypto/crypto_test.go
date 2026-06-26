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
