package observability

import (
	"context"
	"log"
	"strconv"
	"time"

	"udsf-go/internal/config"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/credentials"
)

// metricExportInterval is how often accumulated metrics are pushed to the collector.
const metricExportInterval = 15 * time.Second

// InitMetrics configures the global meter provider (OTLP/gRPC push, same endpoint,
// TLS and resource attributes as tracing) and returns a shutdown func that flushes
// pending metrics. When open_telemetry.enabled is false the global no-op provider
// stays in place, so instruments created from it cost nothing. HTTP metrics come from the otelgin middleware.
func InitMetrics(cfg *config.UdsfConfig) func(context.Context) error {
	if !isOtelEnabled(cfg) {
		return func(context.Context) error { return nil }
	}

	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(setOtelUrl(cfg))}
	if cfg.OpenTelemetry.TLS.Enabled {
		tlsCfg, err := cfg.OTelTLSConfig()
		if err != nil {
			log.Fatalf("Failed to build OTLP TLS config: %v", err)
		}
		opts = append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	if t := cfg.OpenTelemetry.OTLPTimeout; t > 0 {
		opts = append(opts, otlpmetricgrpc.WithTimeout(time.Duration(t)*time.Second))
	}

	exporter, err := otlpmetricgrpc.New(context.Background(), opts...)
	if err != nil {
		log.Fatalf("Failed to create metric exporter: %v", err)
	}
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(buildAttributes(cfg, ServiceName(cfg))...),
	)
	if err != nil {
		log.Fatalf("Could not set resources: %v", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(metricExportInterval))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(provider)
	return provider.Shutdown
}

// RouteKey is the gin context key a handler can set to override the http.route label
// (used for sub-routers mounted under a gin wildcard, where FullPath is just the wildcard).
const RouteKey = "udsf.http.route"

// RouteLatency returns a gin middleware recording udsf.http.route.duration (seconds),
// labelled with method, matched route template and status code. otelgin's own HTTP
// metrics carry no route label, so this is the per-route latency breakdown.
// Unmatched requests use route "unmatched" so raw paths never become label values.
// Call it after InitMetrics.
func RouteLatency() gin.HandlerFunc {
	duration, err := otel.Meter("udsf-go").Float64Histogram("udsf.http.route.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Inbound HTTP request duration by route"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		log.Printf("otel: create route duration instrument: %v", err)
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.GetString(RouteKey)
		if route == "" {
			route = c.FullPath()
		}
		if route == "" {
			route = "unmatched"
		}
		duration.Record(c.Request.Context(), time.Since(start).Seconds(), metric.WithAttributes(
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.String("http.response.status_code", strconv.Itoa(c.Writer.Status())),
		))
	}
}
