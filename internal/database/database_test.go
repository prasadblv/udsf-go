package database

import (
	"udsf-go/internal/config"

	"context"
	"log"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

var testCfg = &config.UdsfConfig{}

func mustStartRedisContainer() (func(context.Context, ...testcontainers.TerminateOption) error, error) {
	dbContainer, err := redis.Run(
		context.Background(),
		"docker.io/redis:7.2.4",
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		return nil, err
	}

	dbHost, err := dbContainer.Host(context.Background())
	if err != nil {
		return dbContainer.Terminate, err
	}

	dbPort, err := dbContainer.MappedPort(context.Background(), "6379/tcp")
	if err != nil {
		return dbContainer.Terminate, err
	}

	testCfg.Redis.Host = dbHost
	testCfg.Redis.Port = int(dbPort.Num())
	testCfg.Redis.Database = 0

	return dbContainer.Terminate, err
}

func TestMain(m *testing.M) {
	teardown, err := mustStartRedisContainer()
	if err != nil {
		log.Fatalf("could not start redis container: %v", err)
	}

	m.Run()

	if teardown != nil && teardown(context.Background()) != nil {
		log.Fatalf("could not teardown redis container: %v", err)
	}
}

func TestNew(t *testing.T) {
	srv := New(testCfg)
	if srv == nil {
		t.Fatal("New(testCfg) returned nil")
	}
}

func TestHealth(t *testing.T) {
	srv := New(testCfg)

	stats := srv.Health()

	if stats["redis_status"] != "up" {
		t.Fatalf("expected status to be up, got %s", stats["redis_status"])
	}

	if _, ok := stats["redis_version"]; !ok {
		t.Fatalf("expected redis_version to be present, got %v", stats["redis_version"])
	}
}
