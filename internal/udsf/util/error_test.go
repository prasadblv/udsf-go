package util

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParsingError(t *testing.T) {
	inner := errors.New("boom")
	if got := (&ParsingError{Param: "id", Err: inner}).Error(); got != "id: boom" {
		t.Errorf("got %q", got)
	}
	if got := (&ParsingError{Err: inner}).Error(); got != "boom" {
		t.Errorf("got %q", got)
	}
	if !errors.Is(&ParsingError{Err: inner}, inner) {
		t.Error("Unwrap should expose inner error")
	}
}

func TestRequiredError(t *testing.T) {
	if got := (&RequiredError{Field: "name"}).Error(); got != "field 'name' is required." {
		t.Errorf("got %q", got)
	}
}

func TestDefaultErrorHandler(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
		body string
	}{
		{"parsing", &ParsingError{Param: "p", Err: errors.New("bad")}, http.StatusBadRequest, "p: bad"},
		{"wrapped parsing", errors.Join(errors.New("x"), &ParsingError{Err: errors.New("bad")}), http.StatusBadRequest, "bad"},
		{"required", &RequiredError{Field: "f"}, http.StatusUnprocessableEntity, "field 'f' is required."},
		{"other uses result code", errors.New("nope"), http.StatusNotFound, "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			DefaultErrorHandler(w, httptest.NewRequest("GET", "/", nil), tt.err, &ImplResponse{Code: http.StatusNotFound})
			if w.Code != tt.want {
				t.Errorf("status %d, want %d", w.Code, tt.want)
			}
			if !strings.Contains(w.Body.String(), tt.body) {
				t.Errorf("body %q missing %q", w.Body.String(), tt.body)
			}
		})
	}
}
