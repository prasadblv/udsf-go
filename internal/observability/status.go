package observability

import (
	"strings"
	"sync/atomic"
	"time"

	"udsf-go/internal/config"
	"udsf-go/internal/udsf/util"

	"go.opentelemetry.io/otel"
)

// errorLogInterval rate-limits SDK/exporter error logs, which repeat on every failed export
// (for example while the collector is down).
const errorLogInterval = 10 * time.Second

// InitErrorHandler routes internal OpenTelemetry errors (failed exports, dropped data) to the
// application log, rate-limited so an unreachable collector does not flood it.
func InitErrorHandler() {
	var lastLogged, suppressed atomic.Int64
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		now := time.Now().UnixNano()
		last := lastLogged.Load()
		if now-last < int64(errorLogInterval) || !lastLogged.CompareAndSwap(last, now) {
			suppressed.Add(1)
			return
		}
		util.LogWarn("otel: %v (%d similar errors suppressed)", err, suppressed.Swap(0))
	}))
}

// signalStatus logs whether a telemetry signal ("tracing", "metrics", "logs") is exporting and,
// if so, where to. It also flags settings that are likely mistakes.
func signalStatus(cfg *config.UdsfConfig, signal string, enabled bool, extra string) {
	if !enabled {
		reason := "open_telemetry.enabled=false"
		if isOtelEnabled(cfg) && signal == "logs" {
			reason = "open_telemetry.logs.enabled=false"
		}
		util.LogInfo("otel %s disabled (%s)", signal, reason)
		return
	}
	ot := cfg.OpenTelemetry
	util.LogInfo("otel %s enabled: endpoint=%s tls=%t service=%s version=%s%s",
		signal, setOtelUrl(cfg), ot.TLS.Enabled, ServiceName(cfg), ot.ServiceVersion, extra)
	if ep := setOtelUrl(cfg); strings.Contains(ep, "://") {
		util.LogWarn("otel %s: endpoint %q contains a scheme; the gRPC exporter expects host:port (e.g. localhost:4317)", signal, ep)
	}
	if ot.TLS.Enabled && ot.TLS.InsecureSkipVerify {
		util.LogWarn("otel %s: tls.insecure_skip_verify is on; collector certificate is not verified", signal)
	}
}

// warnRedisOTelWithoutOTel reports redis.enable_opentelemetry having no effect.
func warnRedisOTelWithoutOTel(cfg *config.UdsfConfig) {
	if cfg.Redis.EnableOpenTelemetry && !isOtelEnabled(cfg) {
		util.LogWarn("redis.enable_opentelemetry is true but open_telemetry.enabled is false; Redis spans and metrics will not be exported")
	}
}
