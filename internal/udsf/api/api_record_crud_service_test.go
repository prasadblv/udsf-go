package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"udsf-go/internal/udsf/route"
	"udsf-go/internal/udsf/util"
)

func multipartForm(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()
	return buf.Bytes(), mw.FormDataContentType()
}

func TestRecordCRUD(t *testing.T) {
	db := &fakeDB{m: map[string]string{}}
	h := route.NewRouter(
		NewRecordCRUDAPIController(NewRecordCRUDAPIService(db)),
		NewBlockCRUDAPIController(NewBlockCRUDAPIService(db)),
	)
	base := "/nudsf-dr/v1/Realm01/Storage01/records/Rec1"
	put := func(url, metaJSON string) int {
		body, ct := multipartForm(t, map[string]string{"meta": metaJSON})
		return do(t, h, "PUT", url, ct, body).Code
	}

	if r := do(t, h, "GET", base, "", nil); r.Code != 404 {
		t.Fatalf("get missing: %d", r.Code)
	}
	if c := put(base, `{"tags":{"a":["1"]}}`); c != 201 {
		t.Fatalf("create: %d", c)
	}
	if c := put(base, `{"tags":{"a":["2"]}}`); c != 204 {
		t.Fatalf("update: %d", c)
	}
	body, ct := multipartForm(t, map[string]string{"meta": `{}`})
	if r := do(t, h, "PUT", base+"?get-previous=true", ct, body); r.Code != 200 {
		t.Fatalf("update get-previous: %d %s", r.Code, r.Body)
	}

	// blocks show up in GetRecord
	do(t, h, "PUT", base+"/blocks/B1", "application/json", []byte(`{}`))
	r := do(t, h, "GET", base, "", nil)
	var rec struct {
		Blocks []string `json:"blocks"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &rec) != nil || len(rec.Blocks) != 1 || rec.Blocks[0] != "B1" {
		t.Fatalf("get: %d %s", r.Code, r.Body)
	}
	if r := do(t, h, "GET", base+"/meta", "", nil); r.Code != 200 {
		t.Fatalf("get meta: %d", r.Code)
	}

	// delete removes the record and its blocks
	if r := do(t, h, "DELETE", base, "", nil); r.Code != 204 {
		t.Fatalf("delete: %d", r.Code)
	}
	if len(db.m) != 0 {
		t.Fatalf("leftover keys: %v", db.m)
	}
	if r := do(t, h, "DELETE", base, "", nil); r.Code != 404 {
		t.Fatalf("delete missing: %d", r.Code)
	}
}

// errDB wraps fakeDB and fails selected operations.
type errDB struct {
	*fakeDB
	failGet, failCreate, failUpdate, failDelete, failScan, failSets, failDeleteMany, failGetMany bool
	lastTTL                                                                                      time.Duration
}

var errBoom = errors.New("boom")

func (e *errDB) Get(ctx context.Context, k string) (string, error) {
	if e.failGet {
		return "", errBoom
	}
	return e.fakeDB.Get(ctx, k)
}
func (e *errDB) Create(ctx context.Context, k string, v any, ttl time.Duration) error {
	e.lastTTL = ttl
	if e.failCreate {
		return errBoom
	}
	return e.fakeDB.Create(ctx, k, v, ttl)
}
func (e *errDB) Update(ctx context.Context, k string, v any) error {
	if e.failUpdate {
		return errBoom
	}
	return e.fakeDB.Update(ctx, k, v)
}
func (e *errDB) Delete(ctx context.Context, k string) error {
	if e.failDelete {
		return errBoom
	}
	return e.fakeDB.Delete(ctx, k)
}
func (e *errDB) Scan(ctx context.Context, p string) ([]string, error) {
	if e.failScan {
		return nil, errBoom
	}
	return e.fakeDB.Scan(ctx, p)
}

func (e *errDB) GetMany(ctx context.Context, keys []string) (map[string]string, error) {
	if e.failGetMany {
		return nil, errBoom
	}
	return e.fakeDB.GetMany(ctx, keys)
}
func (e *errDB) SetMembers(ctx context.Context, k string) ([]string, error) {
	if e.failSets {
		return nil, errBoom
	}
	return e.fakeDB.SetMembers(ctx, k)
}
func (e *errDB) SetAdd(ctx context.Context, k string, m ...string) error {
	if e.failSets {
		return errBoom
	}
	return e.fakeDB.SetAdd(ctx, k, m...)
}
func (e *errDB) DeleteMany(ctx context.Context, keys ...string) error {
	if e.failDeleteMany {
		return errBoom
	}
	return e.fakeDB.DeleteMany(ctx, keys...)
}

// seedBlock stores a block and indexes it, as the services do.
func seedBlock(db *errDB, rec, id, stored string) {
	db.m[util.CreateBlockKey(tRealm, tStore, rec, id)] = stored
	db.fakeDB.SetAdd(context.Background(), util.CreateBlockIndexKey(tRealm, tStore, rec), id)
}

func newErrDB() *errDB { return &errDB{fakeDB: &fakeDB{m: map[string]string{}}} }

const (
	tRealm, tStore, tRec = "R", "S", "Rec"
)

func seedRecord(t *testing.T, db *errDB, meta RecordMeta) {
	t.Helper()
	svc := NewRecordCRUDAPIService(db)
	r, _ := svc.CreateOrModifyRecord(context.Background(), tRealm, tStore, tRec, meta, "", "", false, "", nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("seed: %d", r.Code)
	}
}

func TestRecordService_CreateTTL(t *testing.T) {
	ctx := context.Background()
	db := newErrDB()
	svc := NewRecordCRUDAPIService(db)

	r, _ := svc.CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{Ttl: time.Now().Add(-time.Hour)}, "", "", false, "", nil)
	if r.Code != http.StatusBadRequest {
		t.Fatalf("past ttl: %d", r.Code)
	}
	if len(db.m) != 0 {
		t.Fatal("record stored despite past ttl")
	}

	r, _ = svc.CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{Ttl: time.Now().Add(time.Hour)}, "", "", false, "", nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("future ttl: %d", r.Code)
	}
	if db.lastTTL <= 59*time.Minute || db.lastTTL > time.Hour {
		t.Fatalf("ttl passed to db = %v", db.lastTTL)
	}

	db2 := newErrDB()
	NewRecordCRUDAPIService(db2).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{}, "", "", false, "", nil)
	if db2.lastTTL != 0 {
		t.Fatalf("no ttl should be 0, got %v", db2.lastTTL)
	}
}

func TestRecordService_CreateOrModify(t *testing.T) {
	ctx := context.Background()
	meta := RecordMeta{Tags: map[string][]string{"k": {"v"}}, CallbackReference: "http://cb"}

	t.Run("if-none-match star on existing is 412", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, meta)
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, meta, "*", "", false, "", nil)
		if r.Code != http.StatusPreconditionFailed {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("if-none-match star on new is 201", func(t *testing.T) {
		r, _ := NewRecordCRUDAPIService(newErrDB()).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, meta, "*", "", false, "", nil)
		if r.Code != http.StatusCreated {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("get-previous returns old meta and stores new", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, meta)
		svc := NewRecordCRUDAPIService(db)
		r, _ := svc.CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{CallbackReference: "http://new"}, "", "", true, "", nil)
		rec, ok := r.Body.(Record)
		if r.Code != http.StatusOK || !ok || rec.Meta.CallbackReference != "http://cb" {
			t.Fatalf("got %d %+v", r.Code, r.Body)
		}
		g, _ := svc.GetMeta(ctx, tRealm, tStore, tRec, "", "", "")
		if g.Body.(RecordMeta).CallbackReference != "http://new" {
			t.Fatalf("not updated: %+v", g.Body)
		}
	})
	t.Run("create db error is 500", func(t *testing.T) {
		db := newErrDB()
		db.failCreate = true
		r, err := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, meta, "", "", false, "", nil)
		if err != nil || r.Code != http.StatusInternalServerError {
			t.Fatalf("got %d %v", r.Code, err)
		}
	})
	t.Run("read error on existing is 500", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, meta)
		db.failGet = true
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, meta, "", "", false, "", nil)
		if r.Code != http.StatusInternalServerError {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("update error is 500", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, meta)
		db.failUpdate = true
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, meta, "", "", false, "", nil)
		if r.Code != http.StatusInternalServerError {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("corrupt stored record is 500", func(t *testing.T) {
		db := newErrDB()
		db.m[util.CreateRecordKey(tRealm, tStore, tRec)] = "not json"
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, meta, "", "", false, "", nil)
		if r.Code != http.StatusInternalServerError {
			t.Fatalf("got %d", r.Code)
		}
	})
}

func TestRecordService_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("missing is 404", func(t *testing.T) {
		r, _ := NewRecordCRUDAPIService(newErrDB()).GetRecord(ctx, tRealm, tStore, tRec, "", "", "")
		if r.Code != http.StatusNotFound {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("returns meta and sorted block ids, ignores other records", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{CallbackReference: "http://cb"})
		seedBlock(db, tRec, "b2", "x\n1")
		seedBlock(db, tRec, "b1", "x\n1")
		seedBlock(db, "Other", "b9", "x\n1")
		r, _ := NewRecordCRUDAPIService(db).GetRecord(ctx, tRealm, tStore, tRec, "", "", "")
		rec := r.Body.(Record)
		if r.Code != 200 || rec.Meta.CallbackReference != "http://cb" || !reflect.DeepEqual(rec.Blocks, []interface{}{"b1", "b2"}) {
			t.Fatalf("got %d %+v", r.Code, rec)
		}
	})
	t.Run("no blocks yields empty non-nil list", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{})
		r, _ := NewRecordCRUDAPIService(db).GetRecord(ctx, tRealm, tStore, tRec, "", "", "")
		if rec := r.Body.(Record); rec.Blocks == nil || len(rec.Blocks) != 0 {
			t.Fatalf("blocks=%#v", rec.Blocks)
		}
	})
	t.Run("get error is 500", func(t *testing.T) {
		db := newErrDB()
		db.failGet = true
		r, _ := NewRecordCRUDAPIService(db).GetRecord(ctx, tRealm, tStore, tRec, "", "", "")
		if r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("index read error is 500", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{})
		db.failSets = true
		r, _ := NewRecordCRUDAPIService(db).GetRecord(ctx, tRealm, tStore, tRec, "", "", "")
		if r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
}

func TestRecordService_GetMeta(t *testing.T) {
	ctx := context.Background()
	db := newErrDB()
	svc := NewRecordCRUDAPIService(db)

	if r, _ := svc.GetMeta(ctx, tRealm, tStore, tRec, "", "", ""); r.Code != 404 {
		t.Fatalf("missing: %d", r.Code)
	}
	seedRecord(t, db, RecordMeta{Tags: map[string][]string{"a": {"1", "2"}}})
	r, _ := svc.GetMeta(ctx, tRealm, tStore, tRec, "", "", "")
	if m, ok := r.Body.(RecordMeta); r.Code != 200 || !ok || !reflect.DeepEqual(m.Tags["a"], []string{"1", "2"}) {
		t.Fatalf("got %d %+v", r.Code, r.Body)
	}
	db.failGet = true
	if r, _ := svc.GetMeta(ctx, tRealm, tStore, tRec, "", "", ""); r.Code != 500 {
		t.Fatalf("error: %d", r.Code)
	}
}

func TestRecordService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("missing is 404", func(t *testing.T) {
		r, _ := NewRecordCRUDAPIService(newErrDB()).DeleteRecord(ctx, tRealm, tStore, tRec, "", false, "")
		if r.Code != 404 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("removes record and blocks only for that record", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{})
		seedBlock(db, tRec, "b1", "x\n1")
		seedBlock(db, "Other", "b1", "x\n1")
		other := util.CreateBlockKey(tRealm, tStore, "Other", "b1")
		r, _ := NewRecordCRUDAPIService(db).DeleteRecord(ctx, tRealm, tStore, tRec, "", false, "")
		if r.Code != 204 || len(db.m) != 1 || db.m[other] == "" || len(db.sets) != 1 || db.sets[util.CreateBlockIndexKey(tRealm, tStore, "Other")] == nil {
			t.Fatalf("got %d keys=%v", r.Code, db.m)
		}
	})
	t.Run("get-previous returns the deleted record", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{CallbackReference: "http://cb"})
		r, _ := NewRecordCRUDAPIService(db).DeleteRecord(ctx, tRealm, tStore, tRec, "", true, "")
		if rec, ok := r.Body.(Record); r.Code != 200 || !ok || rec.Meta.CallbackReference != "http://cb" {
			t.Fatalf("got %d %+v", r.Code, r.Body)
		}
	})
	t.Run("blocks delete error is 500", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{})
		db.failDeleteMany = true
		r, _ := NewRecordCRUDAPIService(db).DeleteRecord(ctx, tRealm, tStore, tRec, "", false, "")
		if r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("read error is 500", func(t *testing.T) {
		db := newErrDB()
		db.failGet = true
		r, _ := NewRecordCRUDAPIService(db).DeleteRecord(ctx, tRealm, tStore, tRec, "", false, "")
		if r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("delete error is 500", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{})
		db.failDelete = true
		r, _ := NewRecordCRUDAPIService(db).DeleteRecord(ctx, tRealm, tStore, tRec, "", false, "")
		if r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("index read error on delete is 500", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{})
		db.failSets = true
		r, _ := NewRecordCRUDAPIService(db).DeleteRecord(ctx, tRealm, tStore, tRec, "", false, "")
		if r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
}

func TestRecordService_NotImplemented(t *testing.T) {
	ctx := context.Background()
	svc := NewRecordCRUDAPIService(newErrDB())
	calls := map[string]func() (ImplResponse, error){
		"SearchRecord": func() (ImplResponse, error) {
			return svc.SearchRecord(ctx, "", "", 0, SearchExpression{}, false, "", RetrieveRecords{}, 0, CountExpression{})
		},
		"BulkDeleteRecords": func() (ImplResponse, error) { return svc.BulkDeleteRecords(ctx, "", "", SearchExpression{}, "") },
		"PatchRecord":       func() (ImplResponse, error) { return svc.PatchRecord(ctx, "", "", "", nil, "", "", nil) },
		"UpdateMeta":        func() (ImplResponse, error) { return svc.UpdateMeta(ctx, "", "", "", nil, "", "") },
	}
	for name, call := range calls {
		r, err := call()
		if r.Code != http.StatusNotImplemented || err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: code=%d err=%v", name, r.Code, err)
		}
	}
}

// multipartWithBlocks builds a PUT body with a meta field and file-style block parts.
func multipartWithBlocks(t *testing.T, metaJSON string, blocks []struct{ id, ct, data string }) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("meta", metaJSON); err != nil {
		t.Fatal(err)
	}
	for i, b := range blocks {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="blocks"; filename="f%d"`, i))
		if b.ct != "" {
			h.Set("Content-Type", b.ct)
		}
		if b.id != "" {
			h.Set("Content-Id", b.id)
		}
		pw, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		pw.Write([]byte(b.data))
	}
	mw.Close()
	return buf.Bytes(), mw.FormDataContentType()
}

func TestRecordPutStoresBlocks(t *testing.T) {
	db := &fakeDB{m: map[string]string{}}
	h := route.NewRouter(
		NewRecordCRUDAPIController(NewRecordCRUDAPIService(db)),
		NewBlockCRUDAPIController(NewBlockCRUDAPIService(db)),
	)
	base := "/nudsf-dr/v1/Realm01/Storage01/records/Rec1"
	bin := []byte{0, 1, 2, 255, 254}

	body, ct := multipartWithBlocks(t, `{}`, []struct{ id, ct, data string }{
		{"<b1>", "application/json", `{"a":1}`},
		{"b2", "application/octet-stream", string(bin)},
	})
	if r := do(t, h, "PUT", base, ct, body); r.Code != 201 {
		t.Fatalf("create: %d %s", r.Code, r.Body)
	}

	// blocks are readable through the Block API with bytes and type intact
	r := do(t, h, "GET", base+"/blocks/b1", "", nil)
	if r.Code != 200 || r.Header().Get("Content-Type") != "application/json" || r.Body.String() != `{"a":1}` {
		t.Fatalf("get b1: %d %q %q", r.Code, r.Header().Get("Content-Type"), r.Body)
	}
	r = do(t, h, "GET", base+"/blocks/b2", "", nil)
	if r.Code != 200 || !bytes.Equal(r.Body.Bytes(), bin) {
		t.Fatalf("get b2: %d", r.Code)
	}

	// record lists both ids
	r = do(t, h, "GET", base, "", nil)
	var rec struct {
		Blocks []string `json:"blocks"`
	}
	json.Unmarshal(r.Body.Bytes(), &rec)
	if !reflect.DeepEqual(rec.Blocks, []string{"b1", "b2"}) {
		t.Fatalf("blocks=%v", rec.Blocks)
	}

	// update upserts: b1 replaced, b3 added, b2 untouched
	body, ct = multipartWithBlocks(t, `{}`, []struct{ id, ct, data string }{
		{"b1", "text/plain", "new"},
		{"b3", "text/plain", "three"},
	})
	if r := do(t, h, "PUT", base, ct, body); r.Code != 204 {
		t.Fatalf("update: %d %s", r.Code, r.Body)
	}
	if r := do(t, h, "GET", base+"/blocks/b1", "", nil); r.Body.String() != "new" || r.Header().Get("Content-Type") != "text/plain" {
		t.Fatalf("b1 not replaced: %q", r.Body)
	}
	r = do(t, h, "GET", base, "", nil)
	json.Unmarshal(r.Body.Bytes(), &rec)
	if !reflect.DeepEqual(rec.Blocks, []string{"b1", "b2", "b3"}) {
		t.Fatalf("blocks after update=%v", rec.Blocks)
	}

	// part without Content-Id falls back to the filename ("f0")
	body, ct = multipartWithBlocks(t, `{}`, []struct{ id, ct, data string }{{"", "", "x"}})
	if r := do(t, h, "PUT", base, ct, body); r.Code != 204 {
		t.Fatalf("filename fallback: %d", r.Code)
	}
	if r := do(t, h, "GET", base+"/blocks/f0", "", nil); r.Code != 200 || r.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("f0: %d %q", r.Code, r.Header().Get("Content-Type"))
	}

	// duplicate ids are rejected and nothing is written
	body, ct = multipartWithBlocks(t, `{}`, []struct{ id, ct, data string }{{"d", "", "1"}, {"d", "", "2"}})
	if r := do(t, h, "PUT", base, ct, body); r.Code != 400 {
		t.Fatalf("dup: %d", r.Code)
	}
	if r := do(t, h, "GET", base+"/blocks/d", "", nil); r.Code != 404 {
		t.Fatalf("dup block stored: %d", r.Code)
	}
}

func TestRecordService_BlockValidationAndRollback(t *testing.T) {
	ctx := context.Background()
	part := func(id string) util.BlockPart {
		return util.BlockPart{ID: id, Content: util.BlockContent{ContentType: "text/plain", Data: []byte(id)}}
	}

	t.Run("bad block type is 400", func(t *testing.T) {
		db := newErrDB()
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{}, "", "", false, "", []interface{}{"just-a-string"})
		if r.Code != 400 || len(db.m) != 0 {
			t.Fatalf("got %d keys=%v", r.Code, db.m)
		}
	})
	t.Run("empty id is 400", func(t *testing.T) {
		db := newErrDB()
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{}, "", "", false, "", []interface{}{part("")})
		if r.Code != 400 || len(db.m) != 0 {
			t.Fatalf("got %d keys=%v", r.Code, db.m)
		}
	})
	t.Run("invalid blocks do not modify an existing record", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{CallbackReference: "http://old"})
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{CallbackReference: "http://new"}, "", "", false, "", []interface{}{part("")})
		g, _ := NewRecordCRUDAPIService(db).GetMeta(ctx, tRealm, tStore, tRec, "", "", "")
		if r.Code != 400 || g.Body.(RecordMeta).CallbackReference != "http://old" {
			t.Fatalf("got %d meta=%+v", r.Code, g.Body)
		}
	})
	t.Run("block write failure on create rolls back record and blocks", func(t *testing.T) {
		db := newErrDB()
		var n int
		svc := NewRecordCRUDAPIService(&createFailsOnNth{errDB: db, failOn: 3, n: &n}) // record, b1, then b2 fails
		r, _ := svc.CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{}, "", "", false, "", []interface{}{part("b1"), part("b2")})
		if r.Code != 500 || len(db.m) != 0 {
			t.Fatalf("got %d keys=%v", r.Code, db.m)
		}
	})
	t.Run("block write failure on update is 500", func(t *testing.T) {
		db := newErrDB()
		seedRecord(t, db, RecordMeta{})
		db.m[util.CreateBlockKey(tRealm, tStore, tRec, "b1")] = "text/plain\nold"
		db.failUpdate = true // record update fails first
		r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{}, "", "", false, "", []interface{}{part("b1")})
		if r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
}

// createFailsOnNth fails the failOn-th Create call.
type createFailsOnNth struct {
	*errDB
	failOn int
	n      *int
}

func (c *createFailsOnNth) Create(ctx context.Context, k string, v any, ttl time.Duration) error {
	*c.n++
	if *c.n == c.failOn {
		return errBoom
	}
	return c.errDB.Create(ctx, k, v, ttl)
}

// The block index must stay in step with blocks written/deleted through either API, because
// GetRecord and DeleteRecord rely on it instead of scanning the keyspace.
func TestBlockIndexMaintainedAcrossAPIs(t *testing.T) {
	ctx := context.Background()
	db := newErrDB()
	recSvc, blkSvc := NewRecordCRUDAPIService(db), NewBlockCRUDAPIService(db)
	idx := util.CreateBlockIndexKey(tRealm, tStore, tRec)
	ids := func() []string { m, _ := db.fakeDB.SetMembers(ctx, idx); sort.Strings(m); return m }

	// record PUT with a block indexes it
	part := util.BlockPart{ID: "r1", Content: util.BlockContent{ContentType: "text/plain", Data: []byte("x")}}
	recSvc.CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{}, "", "", false, "", []interface{}{part})
	// Block API create and update index too
	blkSvc.CreateOrModifyBlock(ctx, tRealm, tStore, tRec, "b1", &BlockContent{ContentType: "t/x", Data: []byte("1")}, false, "", "", "")
	if !reflect.DeepEqual(ids(), []string{"b1", "r1"}) {
		t.Fatalf("index after creates = %v", ids())
	}
	// an update re-indexes a block that predates the index
	db.m[util.CreateBlockKey(tRealm, tStore, tRec, "legacy")] = "t/x\nold"
	blkSvc.CreateOrModifyBlock(ctx, tRealm, tStore, tRec, "legacy", &BlockContent{ContentType: "t/x", Data: []byte("new")}, false, "", "", "")
	if !reflect.DeepEqual(ids(), []string{"b1", "legacy", "r1"}) {
		t.Fatalf("index after legacy update = %v", ids())
	}

	// GetRecord reflects the index
	g, _ := recSvc.GetRecord(ctx, tRealm, tStore, tRec, "", "", "")
	if !reflect.DeepEqual(g.Body.(Record).Blocks, []interface{}{"b1", "legacy", "r1"}) {
		t.Fatalf("GetRecord blocks = %v", g.Body.(Record).Blocks)
	}

	// Block API delete unindexes
	blkSvc.DeleteBlock(ctx, tRealm, tStore, tRec, "b1", false, "", "")
	if !reflect.DeepEqual(ids(), []string{"legacy", "r1"}) {
		t.Fatalf("index after block delete = %v", ids())
	}

	// record delete removes blocks and the index itself
	recSvc.DeleteRecord(ctx, tRealm, tStore, tRec, "", false, "")
	if len(db.m) != 0 || len(db.sets) != 0 {
		t.Fatalf("leftovers: keys=%v sets=%v", db.m, db.sets)
	}
}

func TestRecordService_IndexWriteFailureOnCreateRollsBack(t *testing.T) {
	ctx := context.Background()
	db := newErrDB()
	db.failSets = true
	part := util.BlockPart{ID: "b", Content: util.BlockContent{Data: []byte("x")}}
	r, _ := NewRecordCRUDAPIService(db).CreateOrModifyRecord(ctx, tRealm, tStore, tRec, RecordMeta{}, "", "", false, "", []interface{}{part})
	if r.Code != 500 || len(db.m) != 0 {
		t.Fatalf("got %d keys=%v", r.Code, db.m)
	}
}
