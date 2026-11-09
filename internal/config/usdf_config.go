package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// DefaultConfigPath is used when no config path is given (e.g. no -config flag).
const DefaultConfigPath = "config/udsf_config.yaml"

// ClientTLS configures an optional outbound TLS connection (Redis, OpenTelemetry). Disabled by default.
type ClientTLS struct {
	Enabled bool `mapstructure:"enabled"`
	// CACertLocation is a PEM bundle used to verify the Redis server; empty = system roots.
	CACertLocation string `mapstructure:"ca_cert_location"`
	// ClientCertLocation/ClientKeyLocation enable mutual TLS; set both or neither.
	ClientCertLocation string `mapstructure:"client_cert_location"`
	ClientKeyLocation  string `mapstructure:"client_key_location"`
	// ServerName overrides the name used for certificate verification (default: redis.host).
	ServerName string `mapstructure:"server_name"`
	// InsecureSkipVerify disables server certificate verification. Development only.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

type UdsfConfig struct {
	Server struct {
		HTTPSPort       string `mapstructure:"https_port"`
		Mode            string `mapstructure:"mode"`
		ReadTimeout     int    `mapstructure:"read_timeout"`     // seconds
		WriteTimeout    int    `mapstructure:"write_timeout"`    // seconds
		IdleTimeout     int    `mapstructure:"idle_timeout"`     // seconds
		ShutdownTimeout int    `mapstructure:"shutdown_timeout"` // seconds to drain in-flight requests on SIGINT/SIGTERM
		MaxHeaderBytes  int    `mapstructure:"max_header_bytes"` // default 1 MiB
		CertPemLocation string `mapstructure:"cert_pem_location"`
		CertKeyLocation string `mapstructure:"cert_key_location"`
	} `mapstructure:"server"`
	App struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"app"`
	Redis struct {
		Host                string    `mapstructure:"host"`
		Port                int       `mapstructure:"port"`
		Password            string    `mapstructure:"password"`
		Database            int       `mapstructure:"database"`
		DialTimeout         int       `mapstructure:"dial_timeout"`  // seconds
		ReadTimeout         int       `mapstructure:"read_timeout"`  // seconds
		WriteTimeout        int       `mapstructure:"write_timeout"` // seconds
		PoolSize            int       `mapstructure:"pool_size"`
		MinIdleConns        int       `mapstructure:"min_idle_conns"`
		MaxIdleConns        int       `mapstructure:"max_idle_conns"`
		ConnMaxIdleTime     int       `mapstructure:"conn_max_idletime"` // minutes
		ConnMaxLifetime     int       `mapstructure:"conn_max_lifetime"` // minutes
		EnableOpenTelemetry bool      `mapstructure:"enable_opentelemetry"`
		TLS                 ClientTLS `mapstructure:"tls"`
	} `mapstructure:"redis"`
	OpenTelemetry struct {
		Enabled         bool      `mapstructure:"enabled"`
		ServiceName     string    `mapstructure:"service_name"`
		ServiceVersion  string    `mapstructure:"service_version"`
		JaegerEndpoint  string    `mapstructure:"jaeger_endpoint"`
		OTLPEndpoint    string    `mapstructure:"otlp_endpoint"`
		OTLPHeaders     string    `mapstructure:"otlp_headers"`
		OTLPCompression string    `mapstructure:"otlp_compression"`
		OTLPTimeout     int       `mapstructure:"otlp_timeout"` // seconds
		TLS             ClientTLS `mapstructure:"tls"`          // used for OTLP/Jaeger exporters
		// Attributes are extra resource attributes attached to every span.
		// Note: viper lower-cases map keys.
		Attributes map[string]string `mapstructure:"attributes"`
		Sampler    struct {
			// always_on, always_off, traceidratio, parentbased_always_on,
			// parentbased_always_off, parentbased_traceidratio
			Type  string  `mapstructure:"type"`
			Ratio float64 `mapstructure:"ratio"` // 0.0-1.0, for the *traceidratio types
		} `mapstructure:"sampler"`
	} `mapstructure:"open_telemetry"`
	Log struct {
		Level string `mapstructure:"level"` // debug, info, warn, error
		// LogFile is an optional file that also receives gin access/recovery logs.
		// Empty or "stdout" = stdout/stderr only (recommended in containers).
		LogFile string `mapstructure:"log_file"`
	} `mapstructure:"log"`
}

// Addr returns the listen address (e.g. ":9443"); a bare port gets a ":" prefix.
func (c *UdsfConfig) Addr() string {
	p := strings.TrimSpace(c.Server.HTTPSPort)
	if !strings.Contains(p, ":") {
		p = ":" + p
	}
	return p
}

// Build returns the *tls.Config for this block, or nil when TLS is disabled.
// defaultServerName is used for verification when ServerName is empty.
func (t ClientTLS) Build(name, defaultServerName string) (*tls.Config, error) {
	if !t.Enabled {
		return nil, nil
	}
	tc := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         t.ServerName,
		InsecureSkipVerify: t.InsecureSkipVerify, //nolint:gosec // opt-in, documented dev-only
	}
	if tc.ServerName == "" {
		tc.ServerName = defaultServerName
	}
	if t.CACertLocation != "" {
		pem, err := os.ReadFile(t.CACertLocation)
		if err != nil {
			return nil, fmt.Errorf("%s tls: read CA cert %q: %w", name, t.CACertLocation, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s tls: CA cert %q contains no valid PEM certificates", name, t.CACertLocation)
		}
		tc.RootCAs = pool
	}
	if (t.ClientCertLocation == "") != (t.ClientKeyLocation == "") {
		return nil, fmt.Errorf("%s tls: client_cert_location and client_key_location must be set together", name)
	}
	if t.ClientCertLocation != "" {
		cert, err := tls.LoadX509KeyPair(t.ClientCertLocation, t.ClientKeyLocation)
		if err != nil {
			return nil, fmt.Errorf("%s tls: load client cert/key: %w", name, err)
		}
		tc.Certificates = []tls.Certificate{cert}
	}
	return tc, nil
}

// RedisTLSConfig builds the *tls.Config for the Redis client, or nil when TLS is disabled.
func (c *UdsfConfig) RedisTLSConfig() (*tls.Config, error) {
	return c.Redis.TLS.Build("redis", c.Redis.Host)
}

// OTelTLSConfig builds the *tls.Config for OpenTelemetry exporters, or nil when TLS is disabled.
// The default verification name is the host of open_telemetry.otlp_endpoint.
func (c *UdsfConfig) OTelTLSConfig() (*tls.Config, error) {
	host := ""
	if u, err := url.Parse(c.OpenTelemetry.OTLPEndpoint); err == nil {
		host = u.Hostname()
	}
	return c.OpenTelemetry.TLS.Build("open_telemetry", host)
}

// RedisAddr returns host:port for Redis.
func (c *UdsfConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// Load reads the YAML config from DefaultConfigPath (config/udsf_config.yaml).
// Configuration comes only from the YAML file (and built-in defaults); environment
// variables are not read. A missing default file falls back to defaults.
func Load() (*UdsfConfig, error) { return LoadFrom("") }

// LoadFrom is like Load but reads the given config file when path is non-empty
// (it must exist); an empty path falls back to DefaultConfigPath.
func LoadFrom(path string) (*UdsfConfig, error) {
	v := viper.New()

	v.SetDefault("server.https_port", ":9443")
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.read_timeout", 10)
	v.SetDefault("server.write_timeout", 30)
	v.SetDefault("server.idle_timeout", 60)
	v.SetDefault("server.shutdown_timeout", 5)
	v.SetDefault("server.max_header_bytes", 1<<20) // 1 MiB
	v.SetDefault("server.cert_pem_location", "certs/server.pem")
	v.SetDefault("server.cert_key_location", "certs/server.key")
	v.SetDefault("app.name", "UDSF")
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.database", 0)
	v.SetDefault("redis.dial_timeout", 5)
	v.SetDefault("redis.read_timeout", 3)
	v.SetDefault("redis.write_timeout", 3)
	v.SetDefault("redis.pool_size", 10*runtime.GOMAXPROCS(0)) // 10 per CPU, like go-redis
	v.SetDefault("redis.min_idle_conns", 0)
	v.SetDefault("redis.max_idle_conns", 0)
	v.SetDefault("redis.conn_max_idletime", 30)
	v.SetDefault("redis.conn_max_lifetime", 0)
	v.SetDefault("redis.enable_opentelemetry", false)
	v.SetDefault("redis.tls.enabled", false)
	v.SetDefault("redis.tls.ca_cert_location", "")
	v.SetDefault("redis.tls.client_cert_location", "")
	v.SetDefault("redis.tls.client_key_location", "")
	v.SetDefault("redis.tls.server_name", "")
	v.SetDefault("redis.tls.insecure_skip_verify", false)
	v.SetDefault("open_telemetry.tls.enabled", false)
	v.SetDefault("open_telemetry.tls.ca_cert_location", "")
	v.SetDefault("open_telemetry.tls.client_cert_location", "")
	v.SetDefault("open_telemetry.tls.client_key_location", "")
	v.SetDefault("open_telemetry.tls.server_name", "")
	v.SetDefault("open_telemetry.tls.insecure_skip_verify", false)
	v.SetDefault("open_telemetry.enabled", false)
	v.SetDefault("open_telemetry.service_name", "udsf")
	v.SetDefault("open_telemetry.service_version", "0.0.1")
	v.SetDefault("open_telemetry.otlp_compression", "gzip")
	v.SetDefault("open_telemetry.otlp_timeout", 10)
	v.SetDefault("open_telemetry.sampler.type", "always_on")
	v.SetDefault("open_telemetry.sampler.ratio", 1.0)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.log_file", "logs/udsf.log")

	explicit := path != ""
	if !explicit {
		path = DefaultConfigPath
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		// Only tolerate a missing default file; an explicit path must exist.
		if explicit || !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read config %q: %w", path, err)
		}
	}

	var cfg UdsfConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	switch cfg.Server.Mode {
	case "debug", "release", "test":
	default:
		return nil, fmt.Errorf("invalid server.mode %q (want debug, release or test)", cfg.Server.Mode)
	}
	if _, err := cfg.RedisTLSConfig(); err != nil {
		return nil, err
	}
	if _, err := cfg.OTelTLSConfig(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
