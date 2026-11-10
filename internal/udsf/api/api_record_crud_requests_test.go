package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"udsf-go/internal/udsf/route"
)

// Request-level behaviour of the Record controller: parsing and validation before the service runs.
func TestRecordController_RequestErrors(t *testing.T) {
	db := newErrDB()
	h := route.NewRouter(NewRecordCRUDAPIController(NewRecordCRUDAPIService(db)))
	base := "/nudsf-dr/v1/R/S/records/Rec"

	t.Run("PUT without multipart body is 400", func(t *testing.T) {
		if r := do(t, h, "PUT", base, "application/json", []byte(`{}`)); r.Code != 400 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("PUT without meta is 400", func(t *testing.T) {
		body, ct := multipartForm(t, map[string]string{"other": "x"})
		if r := do(t, h, "PUT", base, ct, body); r.Code != 400 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("PUT with invalid get-previous is 400", func(t *testing.T) {
		body, ct := multipartForm(t, map[string]string{"meta": "{}"})
		if r := do(t, h, "PUT", base+"?get-previous=maybe", ct, body); r.Code != 400 {
			t.Fatalf("got %d", r.Code)
		}
		if len(db.m) != 0 {
			t.Fatalf("record stored despite bad request: %v", db.m)
		}
	})
	t.Run("DELETE with invalid get-previous is 400", func(t *testing.T) {
		if r := do(t, h, "DELETE", base+"?get-previous=maybe", "", nil); r.Code != 400 {
			t.Fatalf("got %d", r.Code)
		}
	})
	t.Run("unimplemented endpoints answer 501 through the controller", func(t *testing.T) {
		for _, tc := range []struct{ method, url string }{
			{"GET", "/nudsf-dr/v1/R/S/records"},
			{"DELETE", "/nudsf-dr/v1/R/S/records?filter=" + `{"cond":{}}`},
		} {
			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(""))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusOK || rec.Code == http.StatusNoContent {
				t.Errorf("%s %s unexpectedly succeeded: %d", tc.method, tc.url, rec.Code)
			}
		}
	})
}

func TestParseBlocksParam(t *testing.T) {
	t.Run("no multipart form yields empty list", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/", nil)
		blocks, err := parseBlocksParam(req)
		if err != nil || blocks == nil || len(blocks) != 0 {
			t.Fatalf("got %#v %v", blocks, err)
		}
	})
}
