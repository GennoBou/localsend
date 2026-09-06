package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDetermineSaveDir_ExplicitDir(t *testing.T) {
	mockGetwd := func() (string, error) {
		return "", errors.New("getwd should not be called")
	}

	explicitDir := "my_custom_dir"
	expected, err := filepath.Abs(explicitDir)
	if err != nil {
		t.Fatalf("Failed to calculate absolute path: %v", err)
	}

	result, err := determineSaveDir(explicitDir, mockGetwd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestDetermineSaveDir_DefaultCwdSuccess(t *testing.T) {
	mockCwd := filepath.Join("tmp", "mock_cwd")
	mockGetwd := func() (string, error) {
		return mockCwd, nil
	}

	expected, err := filepath.Abs(mockCwd)
	if err != nil {
		t.Fatalf("Failed to calculate absolute path: %v", err)
	}

	result, err := determineSaveDir("", mockGetwd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestDetermineSaveDir_CwdErrorPath(t *testing.T) {
	expectedErr := errors.New("permission denied getting cwd")
	mockGetwd := func() (string, error) {
		return "", expectedErr
	}

	result, err := determineSaveDir("", mockGetwd)
	if err == nil {
		t.Fatalf("Expected error, got nil with result: %s", result)
	}

	if !errors.Is(err, expectedErr) && err.Error() != expectedErr.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
	}
}
