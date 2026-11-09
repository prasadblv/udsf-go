package observability

import (
	"context"
	"log"
	"time"

	"udsf-go/internal/config"
	"udsf-go/internal/udsf/util"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/credentials"
)

// InitLogs exports application logs over OTLP/gRPC (same endpoint, TLS and resource attributes as
// tracing) when open_telemetry.enabled and open_telemetry.logs.enabled are both true. The collector
// forwards them to VictoriaLogs. Console logging is unchanged. Records emitted inside a traced
// request carry its trace context. Returns a shutdown func that flushes pending logs.
func InitLogs(cfg *config.UdsfConfig) func(context.Context) error {
	if !isOtelEnabled(cfg) || !cfg.OpenTelemetry.Logs.Enabled {
		signalStatus(cfg, "logs", false, "")
		return func(context.Context) error { return nil }
	}

	opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(setOtelUrl(cfg))}
	if cfg.OpenTelemetry.TLS.Enabled {
		tlsCfg, err := cfg.OTelTLSConfig()
		if err != nil {
			log.Fatalf("Failed to build OTLP TLS config: %v", err)
		}
		opts = append(opts, otlploggrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, otlploggrpc.WithInsecure())
	}
	if t := cfg.OpenTelemetry.OTLPTimeout; t > 0 {
		opts = append(opts, otlploggrpc.WithTimeout(time.Duration(t)*time.Second))
	}

	exporter, err := otlploggrpc.New(context.Background(), opts...)
	if err != nil {
		log.Fatalf("Failed to create log exporter: %v", err)
	}
	res, err := resource.New(context.Background(),
		resource.WithAttributes(buildAttributes(cfg, ServiceName(cfg))...))
	if err != nil {
		log.Fatalf("Could not set resources: %v", err)
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(provider)
	util.AddLogHandler(otelslog.NewHandler("udsf-go", otelslog.WithLoggerProvider(provider)))
	signalStatus(cfg, "logs", true, "")
	return provider.Shutdown
}
