package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"udsf-go/internal/database"
)

type pingDB struct {
	database.Service
	err error
}

func (p *pingDB) Ping(context.Context) error { return p.err }

func TestRejectsWith503WhenRedisDown(t *testing.T) {
	db := &pingDB{err: errors.New("connection refused")}
	s := &Server{db: db}
	h := s.RegisterRoutes()

	get := func(path string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Code
	}

	if got := get("/readyz"); got != http.StatusServiceUnavailable {
		t.Fatalf("/readyz with redis down = %d, want 503", got)
	}
	if got := get("/healthz"); got != http.StatusOK {
		t.Fatalf("/healthz must stay up, got %d", got)
	}

	// Recovery: force the cached probe to expire.
	db.err = nil
	s.redisGate.checkedAt.Store(0)
	if got := get("/readyz"); got != http.StatusOK {
		t.Fatalf("/readyz after recovery = %d, want 200", got)
	}
}
