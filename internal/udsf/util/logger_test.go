package util

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	called := false
	h := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}), "MyRoute")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/some/path", nil))

	if !called {
		t.Error("inner handler not called")
	}
	if w.Code != http.StatusTeapot {
		t.Errorf("status %d", w.Code)
	}
	out := buf.String()
	for _, s := range []string{"POST", "/some/path", "MyRoute"} {
		if !strings.Contains(out, s) {
			t.Errorf("log %q missing %q", out, s)
		}
	}
}
