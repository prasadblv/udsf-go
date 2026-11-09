package config

import "testing"

func TestRedisTLSConfig(t *testing.T) {
	c := &UdsfConfig{}
	c.Redis.Host = "redis.local"
	if tc, err := c.RedisTLSConfig(); tc != nil || err != nil {
		t.Fatalf("disabled: got %v, %v; want nil, nil", tc, err)
	}

	c.Redis.TLS.Enabled = true
	tc, err := c.RedisTLSConfig()
	if err != nil || tc == nil || tc.ServerName != "redis.local" {
		t.Fatalf("enabled: got %+v, %v", tc, err)
	}

	c.Redis.TLS.ClientCertLocation = "only-cert.pem"
	if _, err := c.RedisTLSConfig(); err == nil {
		t.Error("expected error when only client cert is set")
	}
	c.Redis.TLS.ClientCertLocation = ""

	c.Redis.TLS.CACertLocation = "missing.pem"
	if _, err := c.RedisTLSConfig(); err == nil {
		t.Error("expected error for missing CA file")
	}
}

func TestLoadDefaultsTLSDisabled(t *testing.T) {
	t.Chdir("../..")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Redis.TLS.Enabled {
		t.Error("redis TLS must be disabled by default")
	}
}

func TestOTelTLSConfig(t *testing.T) {
	c := &UdsfConfig{}
	c.OpenTelemetry.OTLPEndpoint = "https://collector.example:4317"
	if tc, err := c.OTelTLSConfig(); tc != nil || err != nil {
		t.Fatalf("disabled: got %v, %v", tc, err)
	}
	c.OpenTelemetry.TLS.Enabled = true
	tc, err := c.OTelTLSConfig()
	if err != nil || tc == nil || tc.ServerName != "collector.example" {
		t.Fatalf("enabled: got %+v, %v", tc, err)
	}
	c.OpenTelemetry.TLS.ClientKeyLocation = "k.pem"
	if _, err := c.OTelTLSConfig(); err == nil {
		t.Error("expected error when only client key is set")
	}
}

func TestLoadOTelTLSDisabledByDefault(t *testing.T) {
	t.Chdir("../..")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.OpenTelemetry.TLS.Enabled {
		t.Error("otel TLS must be disabled by default")
	}
}

func TestEnvIsIgnored(t *testing.T) {
	t.Chdir("../..")
	t.Setenv("UDSF_LOG_LEVEL", "error")
	t.Setenv("UDSF_REDIS_HOST", "env-host")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Log.Level != "info" { // value in config/udsf_config.yaml
		t.Errorf("log level = %q; want config file value %q", c.Log.Level, "info")
	}
	if c.Redis.Host == "env-host" {
		t.Error("environment variables must not override the config file")
	}
}
