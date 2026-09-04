package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestLogDebug_Disabled(t *testing.T) {
	oldDebugFlag := debugFlag
	defer func() { debugFlag = oldDebugFlag }()

	debugFlag = false
	out := captureStdout(func() {
		logDebug("test message %s", "hello")
	})

	if out != "" {
		t.Errorf("expected no output when debugFlag is false, got: %q", out)
	}
}

func TestLogDebug_Enabled(t *testing.T) {
	oldDebugFlag := debugFlag
	defer func() { debugFlag = oldDebugFlag }()

	debugFlag = true
	out := captureStdout(func() {
		logDebug("test message %s %d", "hello", 123)
	})

	expected := "[DEBUG] test message hello 123\n"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestLogDebug_PercentInFormat(t *testing.T) {
	oldDebugFlag := debugFlag
	defer func() { debugFlag = oldDebugFlag }()

	debugFlag = true
	out := captureStdout(func() {
		logDebug("Progress: 50%%")
	})

	expected := "[DEBUG] Progress: 50%\n"
	if !strings.Contains(out, "Progress: 50%") {
		t.Errorf("expected output to contain %q, got %q", expected, out)
	}
}
