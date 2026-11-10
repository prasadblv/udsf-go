package api

import (
	"context"
	"net/http"
	"testing"

	"udsf-go/internal/udsf/util"
)

func blockSvc(db *errDB) *BlockCRUDAPIService { return NewBlockCRUDAPIService(db) }

func TestBlockService_GetBlock(t *testing.T) {
	ctx := context.Background()
	key := util.CreateBlockKey(tRealm, tStore, tRec, "b")

	t.Run("missing is 404", func(t *testing.T) {
		r, _ := blockSvc(newErrDB()).GetBlock(ctx, tRealm, tStore, tRec, "b", "", "", "")
		if r.Code != 404 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("db error is 500", func(t *testing.T) {
		db := newErrDB()
		db.failGet = true
		if r, _ := blockSvc(db).GetBlock(ctx, tRealm, tStore, tRec, "b", "", "", ""); r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("corrupt stored block is 500", func(t *testing.T) {
		db := newErrDB()
		db.m[key] = "no-newline-header"
		if r, _ := blockSvc(db).GetBlock(ctx, tRealm, tStore, tRec, "b", "", "", ""); r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("returns stored content", func(t *testing.T) {
		db := newErrDB()
		db.m[key] = "text/plain\nhi"
		r, _ := blockSvc(db).GetBlock(ctx, tRealm, tStore, tRec, "b", "", "", "")
		bc, ok := r.Body.(*BlockContent)
		if r.Code != 200 || !ok || bc.ContentType != "text/plain" || string(bc.Data) != "hi" {
			t.Fatalf("got %d %+v", r.Code, r.Body)
		}
	})
}

func TestBlockService_GetBlockList(t *testing.T) {
	ctx := context.Background()

	t.Run("index read error is 500", func(t *testing.T) {
		db := newErrDB()
		db.failSets = true
		if r, _ := blockSvc(db).GetBlockList(ctx, tRealm, tStore, tRec, ""); r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("corrupt block is 500", func(t *testing.T) {
		db := newErrDB()
		seedBlock(db, tRec, "b", "garbage")
		if r, _ := blockSvc(db).GetBlockList(ctx, tRealm, tStore, tRec, ""); r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("only this record's blocks, sorted by id", func(t *testing.T) {
		db := newErrDB()
		seedBlock(db, tRec, "b2", "t/x\n2")
		seedBlock(db, tRec, "b1", "t/x\n1")
		seedBlock(db, "Other", "b3", "t/x\n3")
		r, _ := blockSvc(db).GetBlockList(ctx, tRealm, tStore, tRec, "")
		l, ok := r.Body.(*util.BlockListMultipart)
		if r.Code != 200 || !ok || len(l.Parts) != 2 || l.Parts[0].ID != "b1" || l.Parts[1].ID != "b2" {
			t.Fatalf("got %d %+v", r.Code, r.Body)
		}
	})
}

func TestBlockService_GetBlockList_StaleIndexEntryIsSkipped(t *testing.T) {
	db := newErrDB()
	seedBlock(db, tRec, "live", "t/x\n1")
	db.fakeDB.SetAdd(context.Background(), util.CreateBlockIndexKey(tRealm, tStore, tRec), "ghost") // no such block
	r, _ := blockSvc(db).GetBlockList(context.Background(), tRealm, tStore, tRec, "")
	l, ok := r.Body.(*util.BlockListMultipart)
	if r.Code != 200 || !ok || len(l.Parts) != 1 || l.Parts[0].ID != "live" {
		t.Fatalf("got %d %+v", r.Code, r.Body)
	}
}

func TestBlockService_GetBlockList_ReadError(t *testing.T) {
	db := newErrDB()
	seedBlock(db, tRec, "b", "t/x\n1")
	db.failGetMany = true
	if r, _ := blockSvc(db).GetBlockList(context.Background(), tRealm, tStore, tRec, ""); r.Code != 500 {
		t.Fatalf("got %d", r.Code)
	}
}

func TestBlockService_CreateOrModify(t *testing.T) {
	ctx := context.Background()
	body := &BlockContent{ContentType: "text/plain", Data: []byte("x")}
	call := func(db *errDB, b *BlockContent, prev bool) ImplResponse {
		r, _ := blockSvc(db).CreateOrModifyBlock(ctx, tRealm, tStore, tRec, "b", b, prev, "", "", "")
		return r
	}

	if r := call(newErrDB(), nil, false); r.Code != http.StatusBadRequest {
		t.Fatalf("nil body: %d", r.Code)
	}
	t.Run("create db error is 500", func(t *testing.T) {
		db := newErrDB()
		db.failCreate = true
		if r := call(db, body, false); r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("initial read error is 500", func(t *testing.T) {
		db := newErrDB()
		db.failGet = true
		if r := call(db, body, false); r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("update error is 500", func(t *testing.T) {
		db := newErrDB()
		db.m[util.CreateBlockKey(tRealm, tStore, tRec, "b")] = "t/x\nold"
		db.failUpdate = true
		if r := call(db, body, false); r.Code != 500 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("get-previous with corrupt previous falls back to 204", func(t *testing.T) {
		db := newErrDB()
		db.m[util.CreateBlockKey(tRealm, tStore, tRec, "b")] = "garbage"
		if r := call(db, body, true); r.Code != 204 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("empty content type defaults to octet-stream", func(t *testing.T) {
		db := newErrDB()
		call(db, &BlockContent{Data: []byte("x")}, false)
		if got := db.m[util.CreateBlockKey(tRealm, tStore, tRec, "b")]; got != "application/octet-stream\nx" {
			t.Fatalf("stored %q", got)
		}
	})
}

func TestBlockService_Delete(t *testing.T) {
	ctx := context.Background()
	key := util.CreateBlockKey(tRealm, tStore, tRec, "b")

	if r, _ := blockSvc(newErrDB()).DeleteBlock(ctx, tRealm, tStore, tRec, "b", false, "", ""); r.Code != 404 {
		t.Fatalf("missing: %d", r.Code)
	}
	db := newErrDB()
	db.m[key] = "t/x\n1"
	db.failDelete = true
	if r, _ := blockSvc(db).DeleteBlock(ctx, tRealm, tStore, tRec, "b", false, "", ""); r.Code != 500 {
		t.Fatalf("db error: %d", r.Code)
	}
	db.failDelete = false
	if r, _ := blockSvc(db).DeleteBlock(ctx, tRealm, tStore, tRec, "b", false, "", ""); r.Code != 204 {
		t.Fatalf("delete: %d", r.Code)
	}
}
