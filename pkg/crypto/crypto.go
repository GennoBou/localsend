package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

var (
	randReader io.Reader = rand.Reader
	rsaGenKey           = rsa.GenerateKey
)

// CertificateInfo holds the TLS certificate, the parsed x509 certificate, and its fingerprint.
type CertificateInfo struct {
	TLSCert     tls.Certificate
	X509Cert    *x509.Certificate
	Fingerprint string
}

// GenerateSelfSignedCert generates a new RSA 2048-bit private key and a self-signed certificate valid for 10 years.
// It returns the tls.Certificate, x509.Certificate, and their respective PEM-encoded bytes.
func GenerateSelfSignedCert() (tls.Certificate, *x509.Certificate, []byte, []byte, error) {
	priv, err := rsaGenKey(randReader, 2048)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(randReader, serialNumberLimit)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "LocalSend",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(randReader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("failed to load x509 key pair: %w", err)
	}

	parsedCert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return tlsCert, parsedCert, certPEM, keyPEM, nil
}

// CalculateFingerprint calculates the SHA-256 fingerprint (lowercase hex) from the DER bytes of the certificate.
func CalculateFingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// LoadOrGenerateCredentials loads the certificate and private key from the specified paths.
// If the files do not exist, it generates new credentials, writes them to disk, and returns them.
// If certPath or keyPath is empty, it generates them in-memory only without persisting.
func LoadOrGenerateCredentials(certPath, keyPath string) (*CertificateInfo, error) {
	if certPath != "" && keyPath != "" {
		if _, err := os.Stat(certPath); err == nil {
			if _, err := os.Stat(keyPath); err == nil {
				// Attempt to load if both files exist
				tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err == nil {
					x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
					if err == nil {
						fingerprint := CalculateFingerprint(tlsCert.Certificate[0])
						return &CertificateInfo{
							TLSCert:     tlsCert,
							X509Cert:    x509Cert,
							Fingerprint: fingerprint,
						}, nil
					}
				}
				// Proceed to generate new if loading fails
			}
		}
	}

	// Generate new
	tlsCert, x509Cert, certPEM, keyPEM, err := GenerateSelfSignedCert()
	if err != nil {
		return nil, fmt.Errorf("failed to generate credentials: %w", err)
	}
	fingerprint := CalculateFingerprint(tlsCert.Certificate[0])

	if certPath != "" && keyPath != "" {
		// Create directories
		certDir := filepath.Dir(certPath)
		if err := os.MkdirAll(certDir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create directory for certificates: %w", err)
		}

		// Write files securely (with 0600 permission)
		if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
			return nil, fmt.Errorf("failed to write certificate to file: %w", err)
		}
		if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
			return nil, fmt.Errorf("failed to write key to file: %w", err)
		}
	}

	return &CertificateInfo{
		TLSCert:     tlsCert,
		X509Cert:    x509Cert,
		Fingerprint: fingerprint,
	}, nil
}
