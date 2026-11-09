package observability

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"udsf-go/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/credentials"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTracing configures the global tracer provider and returns a shutdown func.
func InitTracing(cfg *config.UdsfConfig) func(context.Context) error {
	warnRedisOTelWithoutOTel(cfg)
	signalStatus(cfg, "tracing", isOtelEnabled(cfg),
		fmt.Sprintf(" sampler=%s ratio=%v", cfg.OpenTelemetry.Sampler.Type, cfg.OpenTelemetry.Sampler.Ratio))
	if isOtelEnabled(cfg) {
		serviceName := ServiceName(cfg)
		otelUrl := setOtelUrl(cfg)
		secureOption := otlptracegrpc.WithInsecure()
		if cfg.OpenTelemetry.TLS.Enabled {
			// CA, client cert/key, server name and skip-verify come from open_telemetry.tls.
			tlsCfg, err := cfg.OTelTLSConfig()
			if err != nil {
				log.Fatalf("Failed to build OTLP TLS config: %v", err)
			}
			secureOption = otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg))
		}

		clientOpts := []otlptracegrpc.Option{
			secureOption,
			otlptracegrpc.WithEndpoint(otelUrl),
		}
		// open_telemetry.otlp_timeout (seconds): max time for each export request.
		if t := cfg.OpenTelemetry.OTLPTimeout; t > 0 {
			clientOpts = append(clientOpts, otlptracegrpc.WithTimeout(time.Duration(t)*time.Second))
		}

		exporter, err := otlptrace.New(
			context.Background(),
			otlptracegrpc.NewClient(clientOpts...),
		)
		if err != nil {
			log.Fatalf("Failed to create exporter: %v", err)
		}
		resources, err := resource.New(
			context.Background(),
			resource.WithAttributes(buildAttributes(cfg, serviceName)...),
		)
		if err != nil {
			log.Fatalf("Could not set resources: %v", err)
		}
		otel.SetTracerProvider(
			sdktrace.NewTracerProvider(
				sdktrace.WithSampler(buildSampler(cfg)),
				sdktrace.WithBatcher(exporter),
				sdktrace.WithResource(resources),
			),
		)
		return exporter.Shutdown
	}
	return func(context.Context) error { return nil }
}

func isOtelEnabled(cfg *config.UdsfConfig) bool {
	return cfg.OpenTelemetry.Enabled
}

// ServiceName returns open_telemetry.service_name, falling back to app.name.
func ServiceName(cfg *config.UdsfConfig) string {
	serviceName := cfg.OpenTelemetry.ServiceName
	if serviceName == "" {
		serviceName = cfg.App.Name
	}
	return serviceName
}

func setOtelUrl(cfg *config.UdsfConfig) string {
	ep := cfg.OpenTelemetry.OTLPEndpoint
	// If OTLP endpoint is not set, use Jaeger endpoint as fallback
	if ep == "" {
		ep = cfg.OpenTelemetry.JaegerEndpoint
	}
	// If both OTLP and Jaeger endpoints are not set, use localhost url as fallback
	if ep == "" {
		ep = "http://localhost:4317"
	}
	return ep
}

// buildAttributes returns the resource attributes: built-in defaults overlaid
// with open_telemetry.attributes from the config file.
func buildAttributes(cfg *config.UdsfConfig, serviceName string) []attribute.KeyValue {
	attrs := map[string]string{
		"library.language": "go",
	}
	if v := cfg.OpenTelemetry.ServiceVersion; v != "" {
		attrs["service.version"] = v
	}
	for k, v := range cfg.OpenTelemetry.Attributes {
		attrs[k] = v
	}
	attrs["service.name"] = serviceName // always wins

	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kvs = append(kvs, attribute.String(k, v))
	}
	return kvs
}

// buildSampler maps open_telemetry.sampler to an SDK sampler. Unknown types
// fall back to always_on with a warning.
func buildSampler(cfg *config.UdsfConfig) sdktrace.Sampler {
	s := cfg.OpenTelemetry.Sampler
	ratio := s.Ratio
	if ratio < 0 || ratio > 1 {
		log.Printf("otel: sampler ratio %v out of range [0,1], clamping", ratio)
		ratio = max(0, min(1, ratio))
	}
	switch strings.ToLower(s.Type) {
	case "", "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		log.Printf("otel: unknown sampler type %q, using always_on", s.Type)
		return sdktrace.AlwaysSample()
	}
}
