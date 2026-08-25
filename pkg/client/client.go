package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/GennoBou/localsend/pkg/crypto"
	"github.com/GennoBou/localsend/pkg/protocol"
)

// SendFileSource represents the data source of a file to be sent.
type SendFileSource struct {
	ID       string
	FileName string
	Size     int64
	FileType string
	Sha256   string
	Preview  string
	Open     func() (io.ReadCloser, error) // Callback to open the file dynamically during transfer
}

// SendResult represents the transmission result for each file.
type SendResult struct {
	FileID string
	Err    error
}

// progressReader is a struct that wraps io.Reader and counts read progress.
type progressReader struct {
	r        io.Reader
	fileID   string
	progress func(fileID string, sentBytes int64)
	total    int64
	current  int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.current += int64(n)
		if pr.progress != nil {
			pr.progress(pr.fileID, pr.current)
		}
	}
	return n, err
}

// Client is a client that handles the file sending process of the LocalSend protocol.
type Client struct {
	myDevice   protocol.Device
	httpClient *http.Client
	insecure   bool
}

// NewClient creates a new sending client.
// clientCert is the self mTLS certificate (nil if not specified).
// proxyURL is the URL of the HTTP/HTTPS proxy (empty string if not specified).
// insecure is true to skip self-signed certificate validation.
// caPath is the filepath to a custom CA certificate (empty string if not specified).
func NewClient(myDevice protocol.Device, clientCert *tls.Certificate, proxyURL string, insecure bool, caPath string) (*Client, error) {
	tlsConfig := &tls.Config{}

	if clientCert != nil {
		tlsConfig.Certificates = []tls.Certificate{*clientCert}
	}

	// LocalSend's HTTPS communication is fundamentally based on self-signed certificates,
	// so standard CA verification is always skipped unless a custom CA is specified.
	// (Actual validation is performed post-connection by verifying the fingerprint)
	if insecure || caPath == "" {
		tlsConfig.InsecureSkipVerify = true
	}

	if caPath != "" {
		caCert, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate to pool")
		}
		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(u)
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   0, // Set timeout to unlimited because file transfer can take a long time
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not follow HTTP redirects sent by peers (prevent SSRF and open redirect attacks)
			return http.ErrUseLastResponse
		},
	}

	// Leave default to control initial timeouts (like connection establishment) at the Transport level
	return &Client{
		myDevice:   myDevice,
		httpClient: httpClient,
		insecure:   insecure,
	}, nil
}

// verifyPeerFingerprint matches the fingerprint of the certificate retrieved from the destination with the expected value.
func (c *Client) verifyPeerFingerprint(resp *http.Response, expectedFingerprint string) error {
	if c.insecure {
		// Skip this custom validation if the --insecure flag is explicitly specified
		return nil
	}
	if resp.TLS == nil {
		// TLS verification is not needed for HTTP communication
		return nil
	}
	if expectedFingerprint == "" {
		// Allow it as unverifiable if no expected hash is specified
		return nil
	}
	if len(resp.TLS.PeerCertificates) == 0 {
		return fmt.Errorf("no peer certificate found")
	}

	peerCert := resp.TLS.PeerCertificates[0]
	actualFingerprint := crypto.CalculateFingerprint(peerCert.Raw)

	if actualFingerprint != expectedFingerprint {
		return fmt.Errorf("TLS certificate fingerprint mismatch. Expected: %s, Got: %s (potential MITM attack)", expectedFingerprint, actualFingerprint)
	}
	return nil
}

// SendFiles sends multiple files in bulk to the specified target device.
func (c *Client) SendFiles(ctx context.Context, target *protocol.Device, files []SendFileSource, pin string, progress func(fileID string, sentBytes int64)) ([]SendResult, error) {
	// 1. Prepare upload request
	filesMap := make(map[string]protocol.FileMetadata)
	for _, f := range files {
		filesMap[f.ID] = protocol.FileMetadata{
			ID:       f.ID,
			FileName: f.FileName,
			Size:     f.Size,
			FileType: f.FileType,
			Sha256:   f.Sha256,
			Preview:  f.Preview,
		}
	}

	prepReq := protocol.PrepareUploadRequest{
		Info:  c.myDevice,
		Files: filesMap,
	}

	reqData, err := json.Marshal(prepReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal prepare request: %w", err)
	}

	targetURL := fmt.Sprintf("%s://%s:%d/api/localsend/v2/prepare-upload", target.Protocol, target.IP, target.Port)
	if pin != "" {
		targetURL += "?pin=" + pin
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, strings.NewReader(string(reqData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute prepare request: %w", err)
	}
	defer resp.Body.Close()

	if err := c.verifyPeerFingerprint(resp, target.Fingerprint); err != nil {
		return nil, fmt.Errorf("failed to verify peer: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusNoContent: // 204: The same file already exists and transfer is not required
		results := make([]SendResult, len(files))
		for i, f := range files {
			results[i] = SendResult{FileID: f.ID, Err: nil}
		}
		return results, nil
	case http.StatusUnauthorized:
		return nil, protocol.ErrInvalidPIN
	case http.StatusForbidden:
		return nil, protocol.ErrRejected
	case http.StatusConflict:
		return nil, protocol.ErrSessionBusy
	case http.StatusOK:
		// Authorization succeeded
	default:
		return nil, fmt.Errorf("prepare-upload failed with status: %d", resp.StatusCode)
	}

	var prepResp protocol.PrepareUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&prepResp); err != nil {
		return nil, fmt.Errorf("failed to decode prepare response: %w", err)
	}

	sessionID := prepResp.SessionID
	var results []SendResult

	// Helper function to cancel the session
	cancelSession := func() {
		cancelURL := fmt.Sprintf("%s://%s:%d/api/localsend/v2/cancel?sessionId=%s", target.Protocol, target.IP, target.Port, sessionID)
		cancelReq, err := http.NewRequest("POST", cancelURL, nil)
		if err == nil {
			_, _ = c.httpClient.Do(cancelReq)
		}
	}

	// 2. Upload each file sequentially
	for _, f := range files {
		token, exists := prepResp.Files[f.ID]
		if !exists {
			results = append(results, SendResult{FileID: f.ID, Err: fmt.Errorf("file not accepted by receiver")})
			continue
		}

		err := func() error {
			fileReader, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer fileReader.Close()

			progReader := &progressReader{
				r:        fileReader,
				fileID:   f.ID,
				progress: progress,
				total:    f.Size,
			}

			uploadURL := fmt.Sprintf("%s://%s:%d/api/localsend/v2/upload?sessionId=%s&fileId=%s&token=%s", target.Protocol, target.IP, target.Port, sessionID, f.ID, token)

			req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, io.NopCloser(progReader))
			if err != nil {
				return fmt.Errorf("failed to create upload request: %w", err)
			}
			req.ContentLength = f.Size
			req.Header.Set("Content-Type", "application/octet-stream")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to upload file data: %w", err)
			}
			defer resp.Body.Close()

			if err := c.verifyPeerFingerprint(resp, target.Fingerprint); err != nil {
				return fmt.Errorf("failed to verify peer: %w", err)
			}

			if resp.StatusCode == http.StatusUnprocessableEntity {
				return fmt.Errorf("upload failed: %w", protocol.ErrChecksumMismatch)
			}

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("upload failed with status: %d", resp.StatusCode)
			}

			return nil
		}()

		if err != nil {
			// Cancel the entire session if even one file fails
			cancelSession()
			results = append(results, SendResult{FileID: f.ID, Err: err})
			return results, fmt.Errorf("transfer failed on file %s: %w", f.FileName, err)
		}

		results = append(results, SendResult{FileID: f.ID, Err: nil})
	}

	return results, nil
}

// Register registers (handshakes) own existence to the target device.
func (c *Client) Register(ctx context.Context, target *protocol.Device) (*protocol.Device, error) {
	url := fmt.Sprintf("%s://%s:%d/api/localsend/v2/register", target.Protocol, target.IP, target.Port)
	reqData, _ := json.Marshal(c.myDevice)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute register request: %w", err)
	}
	defer resp.Body.Close()

	if err := c.verifyPeerFingerprint(resp, target.Fingerprint); err != nil {
		return nil, fmt.Errorf("failed to verify peer: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("register failed with status: %d", resp.StatusCode)
	}

	var partner protocol.Device
	if err := json.NewDecoder(resp.Body).Decode(&partner); err != nil {
		return nil, fmt.Errorf("failed to decode register response: %w", err)
	}
	partner.IP = target.IP
	if partner.Port == 0 {
		partner.Port = target.Port
	}
	if partner.Protocol == "" {
		partner.Protocol = target.Protocol
	}
	return &partner, nil
}

// NewTextSendSource creates a SendFileSource for sending a text message.
func NewTextSendSource(text string) SendFileSource {
	id := uuid.NewString()
	bytesData := []byte(text)
	hasher := sha256.New()
	hasher.Write(bytesData)
	hashStr := hex.EncodeToString(hasher.Sum(nil))

	// Short preview for filename
	preview := text
	if len(preview) > 30 {
		preview = preview[:30] + "..."
	}
	fileName := preview + ".txt"

	return SendFileSource{
		ID:       id,
		FileName: fileName,
		Size:     int64(len(bytesData)),
		FileType: "text/plain",
		Sha256:   hashStr,
		Preview:  text,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bytesData)), nil
		},
	}
}
