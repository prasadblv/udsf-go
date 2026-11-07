package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"udsf-go/internal/config"
	"udsf-go/internal/database"
	"udsf-go/internal/udsf/util"
)

type Server struct {
	cfg *config.UdsfConfig
	db  database.Service
	// shuttingDown is set once graceful shutdown starts; new requests get 503.
	shuttingDown atomic.Bool
}

// NewServer builds the HTTPS server from cfg, loading the certificate and key
// from server.cert_pem_location / server.cert_key_location. Start it with
// ListenAndServeTLS("", "") since the certificate is already in TLSConfig.
func NewServer(cfg *config.UdsfConfig) (*http.Server, error) {
	cert, err := tls.LoadX509KeyPair(cfg.Server.CertPemLocation, cfg.Server.CertKeyLocation)
	if err != nil {
		return nil, fmt.Errorf("load TLS cert %q / key %q: %w (run ./generate-cert.sh)",
			cfg.Server.CertPemLocation, cfg.Server.CertKeyLocation, err)
	}

	gin.SetMode(cfg.Server.Mode)
	util.SetLogLevel(cfg.Log.Level)
	setupGinLogging(cfg.Log.LogFile)

	s := &Server{cfg: cfg, db: database.New(cfg)}
	log.Printf("%s listening on %s (mode=%s, redis=%s db=%d)",
		cfg.App.Name, cfg.Addr(), cfg.Server.Mode, cfg.RedisAddr(), cfg.Redis.Database)

	srv := &http.Server{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h2", "http/1.1"},
		},
		Addr:           cfg.Addr(),
		Handler:        s.RegisterRoutes(),
		IdleTimeout:    time.Duration(cfg.Server.IdleTimeout) * time.Second,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}
	// Runs when srv.Shutdown is called: from then on new requests are rejected with 503.
	srv.RegisterOnShutdown(func() { s.shuttingDown.Store(true) })
	return srv, nil
}

// setupGinLogging points gin's output (route registration, recovery panics) at stdout/stderr so
// container runtimes capture them, and additionally tees them to logFile when set.
// It must run before RegisterRoutes (gin.Recovery captures the writer).
func setupGinLogging(logFile string) {
	gin.DefaultWriter = os.Stdout
	gin.DefaultErrorWriter = os.Stderr

	logFile = strings.TrimSpace(logFile)
	if logFile == "" || strings.EqualFold(logFile, "stdout") {
		return
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err == nil {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			gin.DefaultWriter = io.MultiWriter(os.Stdout, f)
			gin.DefaultErrorWriter = io.MultiWriter(os.Stderr, f)
			return
		}
		log.Printf("log file %q unavailable, logging to stdout only: %v", logFile, err)
		return
	}
	log.Printf("log dir for %q unavailable, logging to stdout only", logFile)
}
