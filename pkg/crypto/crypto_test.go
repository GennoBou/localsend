package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errReader struct {
	err error
}

func (e errReader) Read(p []byte) (int, error) {
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

func TestGenerateSelfSignedCert_Errors(t *testing.T) {
	oldRandReader := randReader
	oldRsaGenKey := rsaGenKey
	defer func() {
		randReader = oldRandReader
		rsaGenKey = oldRsaGenKey
	}()

	t.Run("RSA Key Generation Error", func(t *testing.T) {
		rsaGenKey = func(random io.Reader, bits int) (*rsa.PrivateKey, error) {
			return nil, errors.New("simulated rsa keygen failure")
		}
		_, _, _, _, err := GenerateSelfSignedCert()
		if err == nil {
			t.Fatal("Expected error on RSA key generation, got nil")
		}
		if !strings.Contains(err.Error(), "failed to generate private key") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Serial Number Generation Error", func(t *testing.T) {
		// Mock rsaGenKey so it succeeds without reading from randReader,
		// allowing randReader failure to directly trigger serial number generation error.
		rsaGenKey = func(random io.Reader, bits int) (*rsa.PrivateKey, error) {
			return rsa.GenerateKey(rand.Reader, bits)
		}
		randReader = errReader{err: errors.New("simulated rand failure for serial number")}
		_, _, _, _, err := GenerateSelfSignedCert()
		if err == nil {
			t.Fatal("Expected error on serial number generation, got nil")
		}
		if !strings.Contains(err.Error(), "failed to generate serial number") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})
}

func TestLoadOrGenerateCredentials_Errors(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localsend-err-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("Generation Error Propagation", func(t *testing.T) {
		oldRandReader := randReader
		randReader = errReader{err: errors.New("simulated rand failure")}
		defer func() { randReader = oldRandReader }()

		_, err := LoadOrGenerateCredentials("", "")
		if err == nil {
			t.Fatal("Expected error when certificate generation fails, got nil")
		}
		if !strings.Contains(err.Error(), "failed to generate credentials") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Invalid Saved Files Load Failure Fallback and File Write Error", func(t *testing.T) {
		certPath := filepath.Join(tempDir, "invalid_cert.pem")
		keyPath := filepath.Join(tempDir, "invalid_key.pem")

		// Create invalid certificate and key files
		if err := os.WriteFile(certPath, []byte("invalid cert pem"), 0600); err != nil {
			t.Fatalf("Failed to write mock cert file: %v", err)
		}
		if err := os.WriteFile(keyPath, []byte("invalid key pem"), 0600); err != nil {
			t.Fatalf("Failed to write mock key file: %v", err)
		}

		// Loading should fail, and it should regenerate and overwrite valid certs
		info, err := LoadOrGenerateCredentials(certPath, keyPath)
		if err != nil {
			t.Fatalf("Expected LoadOrGenerateCredentials to overwrite invalid certs and succeed, got error: %v", err)
		}
		if info == nil || info.Fingerprint == "" {
			t.Error("Expected valid CertificateInfo after regenerating")
		}
	})

	t.Run("Create Directory Error", func(t *testing.T) {
		// Create a file where a parent directory should be to force os.MkdirAll to fail
		filePathAsParentDir := filepath.Join(tempDir, "file_as_parent")
		if err := os.WriteFile(filePathAsParentDir, []byte("file"), 0600); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
		certPath := filepath.Join(filePathAsParentDir, "cert.pem")
		keyPath := filepath.Join(tempDir, "key.pem")

		_, err := LoadOrGenerateCredentials(certPath, keyPath)
		if err == nil {
			t.Fatal("Expected error when creating directory fails, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create directory for certificates") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Write File Error On Cert Path", func(t *testing.T) {
		// Pass a directory path as the file path to cause os.WriteFile to fail
		subDir := filepath.Join(tempDir, "dir_as_cert")
		if err := os.MkdirAll(subDir, 0700); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		keyPath := filepath.Join(tempDir, "valid_key.pem")

		_, err := LoadOrGenerateCredentials(subDir, keyPath)
		if err == nil {
			t.Fatal("Expected error when writing cert to directory path, got nil")
		}
		if !strings.Contains(err.Error(), "failed to write certificate to file") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Write File Error On Key Path", func(t *testing.T) {
		certPath := filepath.Join(tempDir, "valid_cert_for_key_err.pem")
		subDirKey := filepath.Join(tempDir, "dir_as_key")
		if err := os.MkdirAll(subDirKey, 0700); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		_, err := LoadOrGenerateCredentials(certPath, subDirKey)
		if err == nil {
			t.Fatal("Expected error when writing key to directory path, got nil")
		}
		if !strings.Contains(err.Error(), "failed to write key to file") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})
}
