package server

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"udsf-go/internal/database"
)

// fakeDB is an in-memory database.Service.
type fakeDB struct{ m map[string]string }

func newFakeDB() *fakeDB { return &fakeDB{m: map[string]string{}} }

func (f *fakeDB) Health() map[string]string { return map[string]string{"redis_status": "up"} }
func (f *fakeDB) Exists(_ context.Context, k string) (bool, error) {
	_, ok := f.m[k]
	return ok, nil
}
func (f *fakeDB) Create(_ context.Context, k string, v any, _ time.Duration) error {
	if _, ok := f.m[k]; ok {
		return database.ErrKeyExists
	}
	f.m[k] = string(v.([]byte))
	return nil
}
func (f *fakeDB) Get(_ context.Context, k string) (string, error) {
	v, ok := f.m[k]
	if !ok {
		return "", database.ErrKeyNotFound
	}
	return v, nil
}
func (f *fakeDB) Update(_ context.Context, k string, v any) error {
	if _, ok := f.m[k]; !ok {
		return database.ErrKeyNotFound
	}
	f.m[k] = string(v.([]byte))
	return nil
}
func (f *fakeDB) Delete(_ context.Context, k string) error {
	if _, ok := f.m[k]; !ok {
		return database.ErrKeyNotFound
	}
	delete(f.m, k)
	return nil
}
func (f *fakeDB) Scan(_ context.Context, pattern string) ([]string, error) {
	var keys []string
	for k := range f.m {
		if ok, _ := path.Match(pattern, k); ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
func (f *fakeDB) GetMany(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, k := range keys {
		if v, ok := f.m[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func serve(h http.Handler, method, url, contentType string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHelloWorldHandler(t *testing.T) {
	s := &Server{}
	r := gin.New()
	r.GET("/", s.HelloWorldHandler)

	rr := serve(r, "GET", "/", "", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
	expected := "{\"message\":\"Hello World\"}"
	if rr.Body.String() != expected {
		t.Errorf("Handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestHealthHandler(t *testing.T) {
	t.Run("no database", func(t *testing.T) {
		r := gin.New()
		r.GET("/dbhealthz", (&Server{}).healthHandler)
		if rr := serve(r, "GET", "/dbhealthz", "", nil); rr.Code != http.StatusServiceUnavailable {
			t.Errorf("got %d want %d", rr.Code, http.StatusServiceUnavailable)
		}
	})
	t.Run("database up", func(t *testing.T) {
		r := gin.New()
		r.GET("/dbhealthz", (&Server{db: newFakeDB()}).healthHandler)
		rr := serve(r, "GET", "/dbhealthz", "", nil)
		if rr.Code != http.StatusOK || rr.Body.String() != `{"redis_status":"up"}` {
			t.Errorf("got %d %s", rr.Code, rr.Body)
		}
	})
}

// TestBlockRoutes exercises the block CRUD API through the full gin router
// (RegisterRoutes), including binary content and the multipart/parallel list.
func TestBlockRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := (&Server{db: newFakeDB()}).RegisterRoutes()
	base := "/nudsf-dr/v1/Realm01/Storage01/records/Rec1/blocks"

	bin := make([]byte, 256)
	for i := range bin {
		bin[i] = byte(i)
	}

	if rr := serve(h, "PUT", base+"/Block01", "application/octet-stream", bin); rr.Code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", rr.Code, rr.Body)
	}
	if rr := serve(h, "PUT", base+"/Block02", "application/json", []byte(`{"v":1}`)); rr.Code != http.StatusCreated {
		t.Fatalf("create json: got %d", rr.Code)
	}

	rr := serve(h, "GET", base+"/Block01", "", nil)
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "application/octet-stream" || !bytes.Equal(rr.Body.Bytes(), bin) {
		t.Fatalf("get: got %d ct=%q, bytes equal=%v", rr.Code, rr.Header().Get("Content-Type"), bytes.Equal(rr.Body.Bytes(), bin))
	}

	if rr := serve(h, "PUT", base+"/Block02", "application/json", []byte(`{"v":2}`)); rr.Code != http.StatusNoContent {
		t.Fatalf("update: got %d want 204", rr.Code)
	}
	rr = serve(h, "PUT", base+"/Block02?get-previous=true", "application/json", []byte(`{"v":3}`))
	if rr.Code != http.StatusOK || rr.Body.String() != `{"v":2}` {
		t.Fatalf("update get-previous: got %d %q", rr.Code, rr.Body)
	}

	rr = serve(h, "GET", base, "", nil)
	mediaType, params, err := mime.ParseMediaType(rr.Header().Get("Content-Type"))
	if rr.Code != http.StatusOK || err != nil || mediaType != "multipart/parallel" {
		t.Fatalf("list: got %d ct=%q", rr.Code, rr.Header().Get("Content-Type"))
	}
	parts := map[string][]byte{}
	partTypes := map[string]string{}
	mr := multipart.NewReader(rr.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(p)
		id := p.Header.Get("Content-Id")
		parts[id], partTypes[id] = data, p.Header.Get("Content-Type")
	}
	if len(parts) != 2 || !bytes.Equal(parts["Block01"], bin) || partTypes["Block01"] != "application/octet-stream" || string(parts["Block02"]) != `{"v":3}` {
		t.Fatalf("unexpected list parts: n=%d types=%v", len(parts), partTypes)
	}

	if rr := serve(h, "DELETE", base+"/Block01", "", nil); rr.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d", rr.Code)
	}
	if rr := serve(h, "GET", base+"/Block01", "", nil); rr.Code != http.StatusNotFound {
		t.Fatalf("get deleted: got %d want 404", rr.Code)
	}
	if rr := serve(h, "DELETE", base+"/Block01", "", nil); rr.Code != http.StatusNotFound {
		t.Fatalf("delete missing: got %d want 404", rr.Code)
	}
}
