package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	var output bytes.Buffer
	log, err := New("info", &output)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	log.Info("hello")
	if !strings.Contains(output.String(), "hello") {
		t.Fatalf("log output = %q, want message", output.String())
	}
}

func TestNewLoggerRejectsInvalidArguments(t *testing.T) {
	if _, err := New("trace", &bytes.Buffer{}); err == nil {
		t.Fatal("New() expected invalid level error")
	}
	if _, err := New("info", nil); err == nil {
		t.Fatal("New() expected nil output error")
	}
}
