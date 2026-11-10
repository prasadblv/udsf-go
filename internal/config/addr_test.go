package config

import "testing"

func TestAddr(t *testing.T) {
	tests := []struct{ in, want string }{
		{":9443", ":9443"},
		{"9443", ":9443"},
		{" 9443 ", ":9443"},
		{"127.0.0.1:9443", "127.0.0.1:9443"},
	}
	for _, tt := range tests {
		c := &UdsfConfig{}
		c.Server.HTTPSPort = tt.in
		if got := c.Addr(); got != tt.want {
			t.Errorf("Addr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRedisAddr(t *testing.T) {
	c := &UdsfConfig{}
	c.Redis.Host, c.Redis.Port = "redis.local", 6380
	if got := c.RedisAddr(); got != "redis.local:6380" {
		t.Errorf("got %q", got)
	}
}
