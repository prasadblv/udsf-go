package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"udsf-go/internal/config"
	"udsf-go/internal/observability"
	"udsf-go/internal/server"
	"udsf-go/internal/udsf/util"
)

func gracefulShutdown(apiServer *http.Server, timeout time.Duration, done chan bool) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Listen for the interrupt signal.
	<-ctx.Done()

	log.Println("shutting down gracefully, press Ctrl+C again to force")
	stop() // Allow Ctrl+C to force shutdown

	// The context is used to inform the server it has server.shutdown_timeout seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown with error: %v", err)
	}

	log.Println("Server exiting")

	// Notify the main goroutine that the shutdown is complete
	done <- true
}

func main() {

	configPath := flag.String("config", "",
		"path to the YAML config file (default "+config.DefaultConfigPath+")")
	flag.StringVar(configPath, "c", "", "shorthand for -config")
	flag.Parse()

	cfg, err := config.LoadFrom(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	observability.InitErrorHandler()
	shutdownTracing := observability.InitTracing(cfg)
	shutdownMetrics := observability.InitMetrics(cfg)
	shutdownLogs := observability.InitLogs(cfg)

	server, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeout) * time.Second
	if shutdownTimeout <= 0 {
		shutdownTimeout = 5 * time.Second
	}

	// Create a done channel to signal when the shutdown is complete
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine
	go gracefulShutdown(server, shutdownTimeout, done)

	err = server.ListenAndServeTLS("", "")
	if err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("http server error: %s", err))
	}

	// Wait for the graceful shutdown to complete
	<-done

	// Flush any buffered spans before exiting.
	tctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if !cfg.OpenTelemetry.Enabled {
		log.Println("Graceful shutdown complete.")
		return
	}
	for _, f := range []struct {
		name  string
		flush func(context.Context) error
	}{{"tracing", shutdownTracing}, {"metrics", shutdownMetrics}, {"logs", shutdownLogs}} {
		start := time.Now()
		if err := f.flush(tctx); err != nil {
			util.LogWarn("otel %s shutdown error after %s: %v", f.name, time.Since(start).Round(time.Millisecond), err)
			continue
		}
		util.LogInfo("otel %s flushed (%s)", f.name, time.Since(start).Round(time.Millisecond))
	}
	log.Println("Graceful shutdown complete.")
}
