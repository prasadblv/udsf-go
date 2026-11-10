package database

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// uniquePrefix isolates each test's keys inside the shared container.
func uniquePrefix(t *testing.T) string {
	return fmt.Sprintf("test:%s:%d:", t.Name(), time.Now().UnixNano())
}

// rawClient gives direct access to Redis to inspect TTLs.
func rawClient(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: testCfg.RedisAddr()})
	t.Cleanup(func() { c.Close() })
	return c
}

func TestPing(t *testing.T) {
	if err := New(testCfg).Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestCreateGetExists(t *testing.T) {
	ctx, db, k := context.Background(), New(testCfg), uniquePrefix(t)+"k"

	if ok, err := db.Exists(ctx, k); err != nil || ok {
		t.Fatalf("exists before create: %v %v", ok, err)
	}
	if _, err := db.Get(ctx, k); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("get missing: %v", err)
	}
	if err := db.Create(ctx, k, "v1", 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := db.Create(ctx, k, "v2", 0); !errors.Is(err, ErrKeyExists) {
		t.Fatalf("second create must fail with ErrKeyExists, got %v", err)
	}
	if v, err := db.Get(ctx, k); err != nil || v != "v1" {
		t.Fatalf("get: %q %v (second create must not overwrite)", v, err)
	}
	if ok, err := db.Exists(ctx, k); err != nil || !ok {
		t.Fatalf("exists after create: %v %v", ok, err)
	}
}

func TestCreateBinarySafe(t *testing.T) {
	ctx, db, k := context.Background(), New(testCfg), uniquePrefix(t)+"bin"
	in := make([]byte, 256)
	for i := range in {
		in[i] = byte(i)
	}
	if err := db.Create(ctx, k, in, 0); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(ctx, k)
	if err != nil || got != string(in) {
		t.Fatalf("binary round trip failed: %v", err)
	}
}

func TestCreateTTL(t *testing.T) {
	ctx, db, p := context.Background(), New(testCfg), uniquePrefix(t)
	raw := rawClient(t)

	if err := db.Create(ctx, p+"ttl", "v", time.Hour); err != nil {
		t.Fatal(err)
	}
	if d := raw.TTL(ctx, p+"ttl").Val(); d <= 59*time.Minute || d > time.Hour {
		t.Fatalf("ttl = %v", d)
	}
	if err := db.Create(ctx, p+"nottl", "v", 0); err != nil {
		t.Fatal(err)
	}
	if d := raw.TTL(ctx, p+"nottl").Val(); d != -1 {
		t.Fatalf("no-ttl key has ttl %v, want -1 (persistent)", d)
	}
}

func TestUpdate(t *testing.T) {
	ctx, db, p := context.Background(), New(testCfg), uniquePrefix(t)

	if err := db.Update(ctx, p+"missing", "v"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if ok, _ := db.Exists(ctx, p+"missing"); ok {
		t.Fatal("update must not create the key")
	}

	if err := db.Create(ctx, p+"k", "old", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(ctx, p+"k", "new"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if v, _ := db.Get(ctx, p+"k"); v != "new" {
		t.Fatalf("value after update = %q", v)
	}
	if d := rawClient(t).TTL(ctx, p+"k").Val(); d <= 0 {
		t.Fatalf("update must keep the TTL, got %v", d)
	}
}

func TestDelete(t *testing.T) {
	ctx, db, k := context.Background(), New(testCfg), uniquePrefix(t)+"k"

	if err := db.Delete(ctx, k); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
	if err := db.Create(ctx, k, "v", 0); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(ctx, k); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.Get(ctx, k); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
}

func TestScan(t *testing.T) {
	ctx, db, p := context.Background(), New(testCfg), uniquePrefix(t)

	if keys, err := db.Scan(ctx, p+"*"); err != nil || len(keys) != 0 {
		t.Fatalf("scan empty: %v %v", keys, err)
	}
	// more keys than the SCAN page size (100) to exercise iteration
	var want []string
	for i := 0; i < 250; i++ {
		k := fmt.Sprintf("%sa:%03d", p, i)
		want = append(want, k)
		if err := db.Create(ctx, k, "v", 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(ctx, p+"b:other", "v", 0); err != nil {
		t.Fatal(err)
	}
	got, err := db.Scan(ctx, p+"a:*")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scan returned %d keys, want %d", len(got), len(want))
	}
}

func TestGetMany(t *testing.T) {
	ctx, db, p := context.Background(), New(testCfg), uniquePrefix(t)

	if m, err := db.GetMany(ctx, nil); err != nil || len(m) != 0 {
		t.Fatalf("no keys: %v %v", m, err)
	}
	db.Create(ctx, p+"a", "1", 0)
	db.Create(ctx, p+"b", "2", 0)
	m, err := db.GetMany(ctx, []string{p + "a", p + "missing", p + "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, map[string]string{p + "a": "1", p + "b": "2"}) {
		t.Fatalf("got %v (missing keys must be omitted)", m)
	}
}

func TestDeleteMany(t *testing.T) {
	ctx, db, p := context.Background(), New(testCfg), uniquePrefix(t)

	if err := db.DeleteMany(ctx); err != nil {
		t.Fatalf("no keys: %v", err)
	}
	db.Create(ctx, p+"a", "1", 0)
	db.Create(ctx, p+"b", "2", 0)
	db.Create(ctx, p+"keep", "3", 0)
	if err := db.DeleteMany(ctx, p+"a", p+"b", p+"missing"); err != nil {
		t.Fatalf("delete many: %v", err)
	}
	for _, k := range []string{"a", "b"} {
		if ok, _ := db.Exists(ctx, p+k); ok {
			t.Errorf("%s not deleted", k)
		}
	}
	if ok, _ := db.Exists(ctx, p+"keep"); !ok {
		t.Error("unrelated key deleted")
	}
}

func TestSets(t *testing.T) {
	ctx, db, k := context.Background(), New(testCfg), uniquePrefix(t)+"set"

	if m, err := db.SetMembers(ctx, k); err != nil || len(m) != 0 {
		t.Fatalf("missing set: %v %v", m, err)
	}
	if err := db.SetAdd(ctx, k); err != nil { // no members is a no-op
		t.Fatal(err)
	}
	if err := db.SetAdd(ctx, k, "a", "b", "a"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetAdd(ctx, k, "b", "c"); err != nil { // duplicates ignored
		t.Fatal(err)
	}
	m, _ := db.SetMembers(ctx, k)
	sort.Strings(m)
	if !reflect.DeepEqual(m, []string{"a", "b", "c"}) {
		t.Fatalf("members = %v", m)
	}
	if err := db.SetRemove(ctx, k, "a", "missing"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetRemove(ctx, k); err != nil {
		t.Fatal(err)
	}
	m, _ = db.SetMembers(ctx, k)
	sort.Strings(m)
	if !reflect.DeepEqual(m, []string{"b", "c"}) {
		t.Fatalf("members after remove = %v", m)
	}
	// removing the last members drops the key; DeleteMany also removes a set
	db.SetRemove(ctx, k, "b", "c")
	if ok, _ := db.Exists(ctx, k); ok {
		t.Error("empty set should not exist")
	}
	db.SetAdd(ctx, k, "x")
	db.DeleteMany(ctx, k)
	if m, _ := db.SetMembers(ctx, k); len(m) != 0 {
		t.Errorf("set survived DeleteMany: %v", m)
	}
}
