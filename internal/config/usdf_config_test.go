package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeCertPair writes a self-signed certificate and key and returns their paths.
func writeCertPair(t *testing.T, dir, name string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, name+".pem")
	keyPath = filepath.Join(dir, name+".key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func TestClientTLSBuild(t *testing.T) {
	dir := t.TempDir()
	cert, key := writeCertPair(t, dir, "good")
	otherCert, _ := writeCertPair(t, dir, "other")
	garbage := filepath.Join(dir, "garbage.pem")
	os.WriteFile(garbage, []byte("not a pem"), 0o600)

	t.Run("disabled returns nil", func(t *testing.T) {
		tc, err := ClientTLS{CACertLocation: "ignored"}.Build("x", "host")
		if tc != nil || err != nil {
			t.Fatalf("got %v %v", tc, err)
		}
	})
	t.Run("defaults: TLS1.2, default server name, system roots", func(t *testing.T) {
		tc, err := ClientTLS{Enabled: true}.Build("x", "host")
		if err != nil {
			t.Fatal(err)
		}
		if tc.MinVersion != tls.VersionTLS12 || tc.ServerName != "host" || tc.RootCAs != nil || tc.InsecureSkipVerify || len(tc.Certificates) != 0 {
			t.Fatalf("unexpected config %+v", tc)
		}
	})
	t.Run("server_name overrides default", func(t *testing.T) {
		tc, _ := ClientTLS{Enabled: true, ServerName: "custom"}.Build("x", "host")
		if tc.ServerName != "custom" {
			t.Fatalf("got %q", tc.ServerName)
		}
	})
	t.Run("insecure_skip_verify is propagated", func(t *testing.T) {
		tc, _ := ClientTLS{Enabled: true, InsecureSkipVerify: true}.Build("x", "host")
		if !tc.InsecureSkipVerify {
			t.Fatal("not propagated")
		}
	})
	t.Run("valid CA populates RootCAs", func(t *testing.T) {
		tc, err := ClientTLS{Enabled: true, CACertLocation: otherCert}.Build("x", "host")
		if err != nil || tc.RootCAs == nil {
			t.Fatalf("got %+v %v", tc, err)
		}
	})
	t.Run("CA with no certificates is rejected", func(t *testing.T) {
		_, err := ClientTLS{Enabled: true, CACertLocation: garbage}.Build("redis", "host")
		if err == nil || !strings.Contains(err.Error(), "no valid PEM") || !strings.Contains(err.Error(), "redis tls") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("missing CA file is rejected", func(t *testing.T) {
		if _, err := (ClientTLS{Enabled: true, CACertLocation: filepath.Join(dir, "nope.pem")}).Build("x", "h"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("client cert and key must be set together", func(t *testing.T) {
		if _, err := (ClientTLS{Enabled: true, ClientCertLocation: cert}).Build("x", "h"); err == nil {
			t.Error("cert without key accepted")
		}
		if _, err := (ClientTLS{Enabled: true, ClientKeyLocation: key}).Build("x", "h"); err == nil {
			t.Error("key without cert accepted")
		}
	})
	t.Run("valid mTLS pair is loaded", func(t *testing.T) {
		tc, err := ClientTLS{Enabled: true, ClientCertLocation: cert, ClientKeyLocation: key}.Build("x", "h")
		if err != nil || len(tc.Certificates) != 1 {
			t.Fatalf("got %+v %v", tc, err)
		}
	})
	t.Run("mismatched or unreadable pair is rejected", func(t *testing.T) {
		_, otherKey := writeCertPair(t, dir, "third")
		if _, err := (ClientTLS{Enabled: true, ClientCertLocation: cert, ClientKeyLocation: otherKey}).Build("x", "h"); err == nil {
			t.Error("mismatched cert/key accepted")
		}
		if _, err := (ClientTLS{Enabled: true, ClientCertLocation: filepath.Join(dir, "no.pem"), ClientKeyLocation: key}).Build("x", "h"); err == nil {
			t.Error("missing cert accepted")
		}
	})
}

func TestOTelTLSConfigServerNameFromEndpoint(t *testing.T) {
	c := &UdsfConfig{}
	c.OpenTelemetry.TLS.Enabled = true
	for endpoint, want := range map[string]string{
		"https://collector.example:4317": "collector.example",
		"collector.example:4317":         "", // no scheme: url.Parse yields no hostname
		"":                               "",
	} {
		c.OpenTelemetry.OTLPEndpoint = endpoint
		tc, err := c.OTelTLSConfig()
		if err != nil {
			t.Fatalf("%q: %v", endpoint, err)
		}
		if tc.ServerName != want {
			t.Errorf("endpoint %q: ServerName = %q, want %q", endpoint, tc.ServerName, want)
		}
	}
	c.OpenTelemetry.TLS.ServerName = "explicit"
	tc, _ := c.OTelTLSConfig()
	if tc.ServerName != "explicit" {
		t.Errorf("explicit server_name ignored: %q", tc.ServerName)
	}
}

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFromDefaults(t *testing.T) {
	c, err := LoadFrom(writeConfig(t, "{}\n"))
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"https_port", c.Server.HTTPSPort, ":9443"},
		{"mode", c.Server.Mode, "release"},
		{"read_timeout", c.Server.ReadTimeout, 10},
		{"write_timeout", c.Server.WriteTimeout, 30},
		{"idle_timeout", c.Server.IdleTimeout, 60},
		{"shutdown_timeout", c.Server.ShutdownTimeout, 5},
		{"max_header_bytes", c.Server.MaxHeaderBytes, 1 << 20},
		{"enable_pprof", c.Server.EnablePprof, false},
		{"cert_pem", c.Server.CertPemLocation, "certs/server.pem"},
		{"cert_key", c.Server.CertKeyLocation, "certs/server.key"},
		{"app.name", c.App.Name, "UDSF"},
		{"redis.host", c.Redis.Host, "127.0.0.1"},
		{"redis.port", c.Redis.Port, 6379},
		{"redis.database", c.Redis.Database, 0},
		{"redis.dial_timeout", c.Redis.DialTimeout, 5},
		{"redis.read_timeout", c.Redis.ReadTimeout, 3},
		{"redis.write_timeout", c.Redis.WriteTimeout, 3},
		{"redis.pool_size", c.Redis.PoolSize, 10 * runtime.GOMAXPROCS(0)},
		{"redis.conn_max_idletime", c.Redis.ConnMaxIdleTime, 30},
		{"redis.enable_opentelemetry", c.Redis.EnableOpenTelemetry, false},
		{"redis.tls.enabled", c.Redis.TLS.Enabled, false},
		{"otel.enabled", c.OpenTelemetry.Enabled, false},
		{"otel.service_name", c.OpenTelemetry.ServiceName, "udsf"},
		{"otel.service_version", c.OpenTelemetry.ServiceVersion, "0.0.1"},
		{"otel.otlp_compression", c.OpenTelemetry.OTLPCompression, "gzip"},
		{"otel.otlp_timeout", c.OpenTelemetry.OTLPTimeout, 10},
		{"otel.logs.enabled", c.OpenTelemetry.Logs.Enabled, false},
		{"otel.sampler.type", c.OpenTelemetry.Sampler.Type, "always_on"},
		{"otel.sampler.ratio", c.OpenTelemetry.Sampler.Ratio, 1.0},
		{"log.level", c.Log.Level, "info"},
		{"log.log_file", c.Log.LogFile, "logs/udsf.log"},
	}
	for _, tt := range checks {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestLoadFromOverrides(t *testing.T) {
	c, err := LoadFrom(writeConfig(t, `
server:
  https_port: "8443"
  mode: debug
  shutdown_timeout: 20
  enable_pprof: true
app:
  name: custom
redis:
  host: redis.internal
  port: 6380
  password: secret
  database: 2
  pool_size: 7
  enable_opentelemetry: true
open_telemetry:
  enabled: true
  otlp_endpoint: "collector:4317"
  attributes:
    Network-Function: staging
  logs:
    enabled: true
  sampler:
    type: traceidratio
    ratio: 0.25
log:
  level: debug
  log_file: stdout
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.HTTPSPort != "8443" || c.Addr() != ":8443" {
		t.Errorf("port %q addr %q", c.Server.HTTPSPort, c.Addr())
	}
	if c.Server.Mode != "debug" || c.Server.ShutdownTimeout != 20 || !c.Server.EnablePprof || c.App.Name != "custom" {
		t.Errorf("server/app overrides lost: %+v %+v", c.Server, c.App)
	}
	if c.RedisAddr() != "redis.internal:6380" || c.Redis.Password != "secret" || c.Redis.Database != 2 || c.Redis.PoolSize != 7 || !c.Redis.EnableOpenTelemetry {
		t.Errorf("redis overrides lost: %+v", c.Redis)
	}
	if !c.OpenTelemetry.Enabled || c.OpenTelemetry.OTLPEndpoint != "collector:4317" || !c.OpenTelemetry.Logs.Enabled {
		t.Errorf("otel overrides lost: %+v", c.OpenTelemetry)
	}
	if c.OpenTelemetry.Sampler.Type != "traceidratio" || c.OpenTelemetry.Sampler.Ratio != 0.25 {
		t.Errorf("sampler: %+v", c.OpenTelemetry.Sampler)
	}
	// viper lower-cases map keys
	if got := c.OpenTelemetry.Attributes["network-function"]; got != "staging" {
		t.Errorf("attributes = %v", c.OpenTelemetry.Attributes)
	}
	// untouched defaults survive a partial file
	if c.Server.ReadTimeout != 10 || c.OpenTelemetry.ServiceName != "udsf" {
		t.Errorf("defaults lost for unset keys")
	}
	if c.Log.Level != "debug" || c.Log.LogFile != "stdout" {
		t.Errorf("log: %+v", c.Log)
	}
}

func TestLoadFromErrors(t *testing.T) {
	dir := t.TempDir()
	cert, key := writeCertPair(t, dir, "c")

	t.Run("explicit path must exist", func(t *testing.T) {
		_, err := LoadFrom(filepath.Join(dir, "missing.yaml"))
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("malformed yaml", func(t *testing.T) {
		if _, err := LoadFrom(writeConfig(t, "server: [unclosed")); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("type mismatch fails to parse", func(t *testing.T) {
		_, err := LoadFrom(writeConfig(t, "redis:\n  port: not-a-number\n"))
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("invalid server.mode", func(t *testing.T) {
		_, err := LoadFrom(writeConfig(t, "server:\n  mode: production\n"))
		if err == nil || !strings.Contains(err.Error(), "invalid server.mode") {
			t.Fatalf("got %v", err)
		}
	})
	for _, mode := range []string{"debug", "release", "test"} {
		if _, err := LoadFrom(writeConfig(t, "server:\n  mode: "+mode+"\n")); err != nil {
			t.Errorf("mode %q rejected: %v", mode, err)
		}
	}
	t.Run("redis tls misconfiguration fails load", func(t *testing.T) {
		_, err := LoadFrom(writeConfig(t, "redis:\n  tls:\n    enabled: true\n    client_cert_location: "+cert+"\n"))
		if err == nil || !strings.Contains(err.Error(), "redis tls") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("otel tls misconfiguration fails load", func(t *testing.T) {
		_, err := LoadFrom(writeConfig(t, "open_telemetry:\n  tls:\n    enabled: true\n    ca_cert_location: "+filepath.Join(dir, "nope.pem")+"\n"))
		if err == nil || !strings.Contains(err.Error(), "open_telemetry tls") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("valid tls settings load", func(t *testing.T) {
		c, err := LoadFrom(writeConfig(t, "redis:\n  tls:\n    enabled: true\n    client_cert_location: "+cert+"\n    client_key_location: "+key+"\n"))
		if err != nil || !c.Redis.TLS.Enabled {
			t.Fatalf("got %v %v", c, err)
		}
	})
}

// Known limitation: viper splits keys on ".", so an attribute such as "deployment.environment"
// becomes a nested map and the file fails to load. Use dash-separated keys instead.
func TestLoadFromDottedAttributeKeysAreRejected(t *testing.T) {
	_, err := LoadFrom(writeConfig(t, "open_telemetry:\n  attributes:\n    deployment.environment: staging\n"))
	if err == nil {
		t.Fatal("dotted attribute keys unexpectedly load; if viper/key-delimiter handling changed, update docs/configuration.md")
	}
}

func TestLoadDefaultPathHandling(t *testing.T) {
	t.Run("missing default file falls back to defaults", func(t *testing.T) {
		t.Chdir(t.TempDir())
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if c.Server.Mode != "release" || c.Redis.Port != 6379 {
			t.Errorf("defaults not applied: %+v", c.Server)
		}
	})
	t.Run("default file is read when present", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "config"), 0o755)
		os.WriteFile(filepath.Join(dir, DefaultConfigPath), []byte("app:\n  name: from-default-file\n"), 0o600)
		t.Chdir(dir)
		c, err := Load()
		if err != nil || c.App.Name != "from-default-file" {
			t.Fatalf("got %+v %v", c, err)
		}
	})
	t.Run("unreadable default file is an error, not a fallback", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "config"), 0o755)
		os.WriteFile(filepath.Join(dir, DefaultConfigPath), []byte("server: [unclosed"), 0o600)
		t.Chdir(dir)
		if _, err := Load(); err == nil {
			t.Fatal("expected error for malformed default file")
		}
	})
}

// The shipped config files must stay loadable.
func TestShippedConfigsLoad(t *testing.T) {
	for _, f := range []string{"udsf_config.yaml", "udsf_config.docker.yaml"} {
		t.Run(f, func(t *testing.T) {
			if _, err := LoadFrom(filepath.Join("..", "..", "config", f)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEnablePprofFlag(t *testing.T) {
	for yaml, want := range map[string]bool{
		"{}\n":                             false, // omitted: disabled
		"server:\n  enable_pprof: true\n":  true,
		"server:\n  enable_pprof: false\n": false,
	} {
		c, err := LoadFrom(writeConfig(t, yaml))
		if err != nil {
			t.Fatalf("%q: %v", yaml, err)
		}
		if c.Server.EnablePprof != want {
			t.Errorf("%q: EnablePprof = %v, want %v", yaml, c.Server.EnablePprof, want)
		}
	}
	if _, err := LoadFrom(writeConfig(t, "server:\n  enable_pprof: maybe\n")); err == nil {
		t.Error("non-boolean enable_pprof must be rejected")
	}
}
