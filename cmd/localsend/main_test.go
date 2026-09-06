package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestHandleAnnounce(t *testing.T) {
	myDevice := protocol.Device{
		Alias:       "MyDevice",
		DeviceModel: "TestModel",
		Fingerprint: "my-fingerprint-1234",
		Port:        53317,
		Protocol:    "http",
	}

	// Setup mock HTTP server for successful registration
	mockPartner := protocol.Device{
		Alias:       "PartnerDevice",
		DeviceModel: "PartnerModel",
		Fingerprint: "partner-fingerprint-5678",
		Port:        53317,
		Protocol:    "http",
	}

	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/localsend/v2/register" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockPartner)
			return
		}
		http.NotFound(w, r)
	}))
	defer successServer.Close()

	uSuccess, err := url.Parse(successServer.URL)
	if err != nil {
		t.Fatalf("failed to parse success server url: %v", err)
	}
	successPort, _ := strconv.Atoi(uSuccess.Port())

	// Setup mock HTTP server for error response during registration
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	uError, err := url.Parse(errorServer.URL)
	if err != nil {
		t.Fatalf("failed to parse error server url: %v", err)
	}
	errorPort, _ := strconv.Atoi(uError.Port())

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name          string
		ctx           context.Context
		targetDev     protocol.Device
		expectErr     bool
		expectedAlias string
	}{
		{
			name: "Success path - valid registration response",
			ctx:  context.Background(),
			targetDev: protocol.Device{
				IP:       uSuccess.Hostname(),
				Port:     successPort,
				Protocol: "http",
			},
			expectErr:     false,
			expectedAlias: mockPartner.Alias,
		},
		{
			name: "Error path - target server returns error status code",
			ctx:  context.Background(),
			targetDev: protocol.Device{
				IP:       uError.Hostname(),
				Port:     errorPort,
				Protocol: "http",
			},
			expectErr: true,
		},
		{
			name: "Error path - target server unreachable",
			ctx:  context.Background(),
			targetDev: protocol.Device{
				IP:       "127.0.0.1",
				Port:     59999,
				Protocol: "http",
			},
			expectErr: true,
		},
		{
			name: "Error path - context canceled",
			ctx:  canceledCtx,
			targetDev: protocol.Device{
				IP:       uSuccess.Hostname(),
				Port:     successPort,
				Protocol: "http",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			if ctx == nil {
				var cancelCtx context.CancelFunc
				ctx, cancelCtx = context.WithTimeout(context.Background(), 2*time.Second)
				defer cancelCtx()
			}

			partner, err := handleAnnounce(ctx, myDevice, nil, tt.targetDev)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if partner == nil {
					t.Fatalf("expected non-nil partner, got nil")
				}
				if partner.Alias != tt.expectedAlias {
					t.Errorf("expected partner alias %q, got %q", tt.expectedAlias, partner.Alias)
				}
			}
		})
	}
}
