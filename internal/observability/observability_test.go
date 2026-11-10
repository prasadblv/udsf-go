package observability

import (
	"context"
	"strings"
	"testing"

	"udsf-go/internal/config"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSetOtelUrl(t *testing.T) {
	tests := []struct{ name, otlp, jaeger, want string }{
		{"otlp wins", "collector:4317", "jaeger:14250", "collector:4317"},
		{"jaeger fallback", "", "jaeger:14250", "jaeger:14250"},
		{"default", "", "", "http://localhost:4317"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &config.UdsfConfig{}
			c.OpenTelemetry.OTLPEndpoint, c.OpenTelemetry.JaegerEndpoint = tt.otlp, tt.jaeger
			if got := setOtelUrl(c); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServiceName(t *testing.T) {
	c := &config.UdsfConfig{}
	c.App.Name = "app"
	if got := ServiceName(c); got != "app" {
		t.Errorf("fallback to app.name: got %q", got)
	}
	c.OpenTelemetry.ServiceName = "svc"
	if got := ServiceName(c); got != "svc" {
		t.Errorf("service_name wins: got %q", got)
	}
}

func TestIsOtelEnabled(t *testing.T) {
	c := &config.UdsfConfig{}
	if isOtelEnabled(c) {
		t.Error("disabled by default")
	}
	c.OpenTelemetry.Enabled = true
	if !isOtelEnabled(c) {
		t.Error("enabled flag ignored")
	}
}

func attrMap(kvs []attribute.KeyValue) map[string]string {
	m := map[string]string{}
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value.AsString()
	}
	return m
}

func TestBuildAttributes(t *testing.T) {
	c := &config.UdsfConfig{}
	c.OpenTelemetry.ServiceVersion = "1.2.3"
	c.OpenTelemetry.Attributes = map[string]string{
		"deployment.environment": "test",
		"library.language":       "override",
		"service.name":           "spoofed",
	}
	m := attrMap(buildAttributes(c, "real-name"))

	if m["service.name"] != "real-name" {
		t.Errorf("service.name must always win, got %q", m["service.name"])
	}
	if m["library.language"] != "override" {
		t.Errorf("config attributes overlay defaults, got %q", m["library.language"])
	}
	if m["service.version"] != "1.2.3" || m["deployment.environment"] != "test" {
		t.Errorf("missing attributes: %v", m)
	}

	m = attrMap(buildAttributes(&config.UdsfConfig{}, "n"))
	if _, ok := m["service.version"]; ok {
		t.Error("service.version must be omitted when empty")
	}
	if m["library.language"] != "go" {
		t.Errorf("default attribute missing: %v", m)
	}
}

func TestBuildSampler(t *testing.T) {
	tests := []struct {
		typ   string
		ratio float64
		want  string // substring of sampler Description
	}{
		{"", 0, "AlwaysOnSampler"},
		{"always_on", 0, "AlwaysOnSampler"},
		{"ALWAYS_OFF", 0, "AlwaysOffSampler"},
		{"traceidratio", 0.25, "TraceIDRatioBased{0.25}"},
		{"traceidratio", 5, "TraceIDRatioBased{1}"},  // clamped down
		{"traceidratio", -1, "TraceIDRatioBased{0}"}, // clamped up
		{"parentbased_always_on", 0, "ParentBased{root:AlwaysOnSampler"},
		{"parentbased_always_off", 0, "ParentBased{root:AlwaysOffSampler"},
		{"parentbased_traceidratio", 0.5, "ParentBased{root:TraceIDRatioBased{0.5}"},
		{"bogus", 0, "AlwaysOnSampler"}, // unknown falls back
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			c := &config.UdsfConfig{}
			c.OpenTelemetry.Sampler.Type, c.OpenTelemetry.Sampler.Ratio = tt.typ, tt.ratio
			var s sdktrace.Sampler = buildSampler(c)
			if d := s.Description(); !strings.Contains(d, tt.want) {
				t.Errorf("type=%q ratio=%v: description %q does not contain %q", tt.typ, tt.ratio, d, tt.want)
			}
		})
	}
}

// With telemetry disabled every Init* returns a usable no-op shutdown func.
func TestInitDisabledReturnsNoopShutdown(t *testing.T) {
	c := &config.UdsfConfig{}
	c.Redis.EnableOpenTelemetry = true // exercises the "has no effect" warning path
	for name, fn := range map[string]func(*config.UdsfConfig) func(context.Context) error{
		"tracing": InitTracing, "metrics": InitMetrics, "logs": InitLogs,
	} {
		shutdown := fn(c)
		if shutdown == nil {
			t.Fatalf("%s: nil shutdown", name)
		}
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("%s: shutdown error %v", name, err)
		}
	}

	// logs stay off when only the global flag is on
	c.OpenTelemetry.Enabled = true
	c.OpenTelemetry.Logs.Enabled = false
	if err := InitLogs(c)(context.Background()); err != nil {
		t.Errorf("logs shutdown: %v", err)
	}
}

func TestSignalStatusDoesNotPanic(t *testing.T) {
	c := &config.UdsfConfig{}
	c.OpenTelemetry.OTLPEndpoint = "http://collector:4317" // scheme warning
	c.OpenTelemetry.TLS.Enabled = true
	c.OpenTelemetry.TLS.InsecureSkipVerify = true // insecure warning
	for _, enabled := range []bool{false, true} {
		for _, sig := range []string{"tracing", "metrics", "logs"} {
			signalStatus(c, sig, enabled, " extra")
		}
	}
	c.OpenTelemetry.Enabled = true
	signalStatus(c, "logs", false, "")
}

func TestInitErrorHandlerDoesNotPanic(t *testing.T) {
	InitErrorHandler()
	// not asserting on output: handler is rate-limited and writes to the app log
}
