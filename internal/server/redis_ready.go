package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// redisProbeTTL is how long a Redis readiness result is reused, so a burst
	// of requests costs one PING instead of one per request.
	redisProbeTTL = time.Second
	// redisProbeTimeout bounds a single PING.
	redisProbeTimeout = 2 * time.Second
	// redisWarnInterval rate-limits the warning logged for rejected requests.
	redisWarnInterval = 5 * time.Second
)

// redisGate caches the result of the most recent Redis readiness probe.
type redisGate struct {
	mu        sync.Mutex
	checkedAt atomic.Int64 // unix nanos of the last probe
	ready     atomic.Bool
	warnedAt  atomic.Int64 // unix nanos of the last warning
	rejected  atomic.Int64 // requests rejected since the last warning
}

// redisReady reports whether Redis answered a recent PING.
func (s *Server) redisReady() bool {
	g := &s.redisGate
	now := time.Now()
	if now.UnixNano()-g.checkedAt.Load() < int64(redisProbeTTL) {
		return g.ready.Load()
	}
	// One goroutine probes; the rest keep using the previous result. Before the very first probe
	// there is no previous result (ready starts false), so concurrent first requests wait for it
	// instead of being rejected with a spurious 503 while the server is starting up.
	if !g.mu.TryLock() {
		if g.checkedAt.Load() == 0 {
			g.mu.Lock()
			g.mu.Unlock() //nolint:staticcheck // only waiting for the in-flight first probe
		}
		return g.ready.Load()
	}
	defer g.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), redisProbeTimeout)
	defer cancel()
	err := s.db.Ping(ctx)
	ok := err == nil
	was := g.ready.Swap(ok)
	g.checkedAt.Store(time.Now().UnixNano())
	if ok && !was && g.warnedAt.Load() != 0 {
		log.Printf("redis is reachable again; accepting requests")
	}
	if !ok {
		log.Printf("WARNING: redis not ready: %v", err)
	}
	return ok
}

// rejectReqWhenRedisDown answers 503 to every request (except the liveness
// probe) while Redis is unreachable.
func (s *Server) rejectReqWhenRedisDown() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.db == nil || c.Request.URL.Path == "/healthz" || s.redisReady() {
			c.Next()
			return
		}
		g := &s.redisGate
		g.rejected.Add(1)
		now := time.Now().UnixNano()
		if last := g.warnedAt.Load(); now-last >= int64(redisWarnInterval) && g.warnedAt.CompareAndSwap(last, now) {
			log.Printf("WARNING: rejecting requests with 503, redis is not ready (%d rejected, latest %s %s)",
				g.rejected.Swap(0), c.Request.Method, c.Request.URL.Path)
		}
		c.Header("Retry-After", "5")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "redis is not ready"})
	}
}
