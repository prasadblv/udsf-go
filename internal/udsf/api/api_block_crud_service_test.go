package api

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

	"udsf-go/internal/database"
	"udsf-go/internal/udsf/route"
)

// fakeDB is an in-memory database.Service.
type fakeDB struct{ m map[string]string }

func (f *fakeDB) Health() map[string]string      { return nil }
func (f *fakeDB) Ping(ctx context.Context) error { return nil }
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

func do(t *testing.T, h http.Handler, method, url, ct string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBlockOctetStreamAndMultipart(t *testing.T) {
	db := &fakeDB{m: map[string]string{}}
	h := route.NewRouter(NewBlockCRUDAPIController(NewBlockCRUDAPIService(db)))
	base := "/nudsf-dr/v1/Realm01/Storage01/records/Rec1/blocks"

	bin := make([]byte, 256)
	for i := range bin {
		bin[i] = byte(i)
	}

	// create: octet-stream and json
	if r := do(t, h, "PUT", base+"/Block03", "application/octet-stream", bin); r.Code != 201 {
		t.Fatalf("create octet-stream: %d %s", r.Code, r.Body)
	}
	if r := do(t, h, "PUT", base+"/Block01", "application/json", []byte(`{"value":42}`)); r.Code != 201 {
		t.Fatalf("create json: %d", r.Code)
	}

	// get round-trips bytes and content type
	r := do(t, h, "GET", base+"/Block03", "", nil)
	if r.Code != 200 || r.Header().Get("Content-Type") != "application/octet-stream" || !bytes.Equal(r.Body.Bytes(), bin) {
		t.Fatalf("get octet-stream: %d ct=%s equal=%v", r.Code, r.Header().Get("Content-Type"), bytes.Equal(r.Body.Bytes(), bin))
	}

	// update: 204 normally, 200 with previous when get-previous=true
	if r := do(t, h, "PUT", base+"/Block01", "application/json", []byte(`{"value":43}`)); r.Code != 204 {
		t.Fatalf("update: %d", r.Code)
	}
	r = do(t, h, "PUT", base+"/Block01?get-previous=true", "application/json", []byte(`{"value":44}`))
	if r.Code != 200 || r.Body.String() != `{"value":43}` {
		t.Fatalf("update get-previous: %d %q", r.Code, r.Body)
	}

	// list is multipart/parallel with Content-Id per part and raw bytes intact
	r = do(t, h, "GET", base, "", nil)
	mt, params, err := mime.ParseMediaType(r.Header().Get("Content-Type"))
	if err != nil || mt != "multipart/parallel" || r.Code != 200 {
		t.Fatalf("list: %d ct=%s", r.Code, r.Header().Get("Content-Type"))
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	got := map[string][]byte{}
	types := map[string]string{}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p)
		id := p.Header.Get("Content-Id")
		got[id], types[id] = b, p.Header.Get("Content-Type")
		if p.Header.Get("Content-Transfer-Encoding") != "binary" {
			t.Errorf("part %s missing Content-Transfer-Encoding", id)
		}
	}
	if len(got) != 2 || !bytes.Equal(got["Block03"], bin) || types["Block03"] != "application/octet-stream" || string(got["Block01"]) != `{"value":44}` {
		t.Fatalf("unexpected parts: %v %v", len(got), types)
	}

	// empty record -> empty multipart; delete -> 204 then 404
	if r := do(t, h, "GET", "/nudsf-dr/v1/Realm01/Storage01/records/Nope/blocks", "", nil); r.Code != 200 {
		t.Fatalf("empty list: %d", r.Code)
	}
	if r := do(t, h, "DELETE", base+"/Block03", "", nil); r.Code != 204 {
		t.Fatalf("delete: %d", r.Code)
	}
	if r := do(t, h, "GET", base+"/Block03", "", nil); r.Code != 404 {
		t.Fatalf("get deleted: %d", r.Code)
	}
}
