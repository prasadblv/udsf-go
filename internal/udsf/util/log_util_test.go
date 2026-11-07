package util

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug":     slog.LevelDebug,
		"DEBUG":     slog.LevelDebug,
		"warn":      slog.LevelWarn,
		" Warning ": slog.LevelWarn,
		"error":     slog.LevelError,
		"ERROR":     slog.LevelError,
	}
	for in, want := range tests {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q)=%v, want %v", in, got, want)
		}
	}
}

func withBufferLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	oldLogger := logger
	oldLevel := levelVar.Level()
	logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar}))
	t.Cleanup(func() {
		logger = oldLogger
		levelVar.Set(oldLevel)
	})
	return &buf
}

func TestLogLevelFiltering(t *testing.T) {
	buf := withBufferLogger(t)
	SetLogLevel("warn")

	LogDebug("d %d", 1)
	LogInfo("i %d", 2)
	LogWarn("w %d", 3)
	LogError("e %d", 4)

	out := buf.String()
	if strings.Contains(out, "d 1") || strings.Contains(out, "i 2") {
		t.Errorf("lower levels should be filtered: %q", out)
	}
	if !strings.Contains(out, "w 3") || !strings.Contains(out, "e 4") {
		t.Errorf("warn/error missing: %q", out)
	}
}

func TestSetLogLevelError(t *testing.T) {
	buf := withBufferLogger(t)
	SetLogLevel("error")
	LogWarn("hidden")
	LogError("shown")
	out := buf.String()
	if strings.Contains(out, "hidden") || !strings.Contains(out, "shown") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestLogDebugEnabled(t *testing.T) {
	buf := withBufferLogger(t)
	SetLogLevel("debug")
	LogDebug("hello %s", "world")
	LogInfo("info msg")
	out := buf.String()
	if !strings.Contains(out, "hello world") || !strings.Contains(out, "info msg") {
		t.Errorf("unexpected output %q", out)
	}
}
