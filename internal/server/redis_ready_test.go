package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

// slowPingDB blocks the first Ping until released, counting calls.
type slowPingDB struct {
	database.Service
	release chan struct{}
	calls   atomic.Int32
}

func (p *slowPingDB) Ping(context.Context) error {
	p.calls.Add(1)
	<-p.release
	return nil
}

// Requests that arrive while the very first probe is in flight must wait for it, not get a
// 503 from the initial "not ready" state (this made load tests fail during startup).
func TestConcurrentFirstRequestsWaitForFirstProbe(t *testing.T) {
	db := &slowPingDB{release: make(chan struct{})}
	s := &Server{db: db}

	const n = 50
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() { results <- s.redisReady() }()
	}
	// let every goroutine reach the gate while the probe is blocked
	time.Sleep(100 * time.Millisecond)
	if len(results) != 0 {
		t.Fatalf("%d requests returned before the first probe finished", len(results))
	}
	close(db.release)

	for i := 0; i < n; i++ {
		if ok := <-results; !ok {
			t.Fatal("a request saw not-ready although Redis is healthy")
		}
	}
	if c := db.calls.Load(); c != 1 {
		t.Errorf("Ping called %d times, want 1 (single probe)", c)
	}
}
