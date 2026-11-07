package util

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestBlockContentMarshal(t *testing.T) {
	got := BlockContent{ContentType: "text/plain", Data: []byte("hi")}.Marshal()
	if string(got) != "text/plain\nhi" {
		t.Errorf("got %q", got)
	}
	got = BlockContent{Data: []byte("x")}.Marshal()
	if string(got) != DefaultBlockContentType+"\nx" {
		t.Errorf("default content type not applied: %q", got)
	}
}

func TestUnmarshalBlockContent(t *testing.T) {
	b, err := UnmarshalBlockContent("text/plain\nhello\nworld")
	if err != nil {
		t.Fatal(err)
	}
	if b.ContentType != "text/plain" || string(b.Data) != "hello\nworld" {
		t.Errorf("got %+v", b)
	}
	b, err = UnmarshalBlockContent("a/b\n")
	if err != nil || b.ContentType != "a/b" || len(b.Data) != 0 {
		t.Errorf("empty data: %+v %v", b, err)
	}
	if _, err := UnmarshalBlockContent("no-header"); err == nil {
		t.Error("expected error without newline")
	}
}

func TestBlockContentRoundTrip(t *testing.T) {
	in := BlockContent{ContentType: "application/x-bin", Data: []byte{0, 1, '\n', 255}}
	out, err := UnmarshalBlockContent(string(in.Marshal()))
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != in.ContentType || !bytes.Equal(out.Data, in.Data) {
		t.Errorf("round trip mismatch: %+v", out)
	}
}

func TestWriteBlockContent(t *testing.T) {
	w := httptest.NewRecorder()
	if err := writeBlockContent(w, 200, &BlockContent{ContentType: "image/png", Data: []byte("png")}); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Body.String() != "png" {
		t.Errorf("got %d %v %q", w.Code, w.Header(), w.Body.String())
	}

	w = httptest.NewRecorder()
	_ = writeBlockContent(w, 201, &BlockContent{Data: []byte("d")})
	if w.Code != 201 || w.Header().Get("Content-Type") != DefaultBlockContentType {
		t.Errorf("default type not used: %d %v", w.Code, w.Header())
	}
}

func TestWriteBlockListMultipart(t *testing.T) {
	list := &BlockListMultipart{Parts: []BlockPart{
		{ID: "b1", Content: BlockContent{ContentType: "text/plain", Data: []byte("one")}},
		{ID: "b2", Content: BlockContent{Data: []byte("two")}},
	}}
	w := httptest.NewRecorder()
	if err := writeBlockListMultipart(w, 200, list); err != nil {
		t.Fatal(err)
	}
	mt, params, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	if err != nil || mt != "multipart/parallel" {
		t.Fatalf("content type %q err %v", mt, err)
	}
	mr := multipart.NewReader(w.Body, params["boundary"])
	want := []struct{ id, ct, body string }{
		{"b1", "text/plain", "one"},
		{"b2", DefaultBlockContentType, "two"},
	}
	for _, e := range want {
		p, err := mr.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(p)
		if p.Header.Get("Content-Id") != e.id || p.Header.Get("Content-Type") != e.ct ||
			p.Header.Get("Content-Transfer-Encoding") != "binary" || string(data) != e.body {
			t.Errorf("part mismatch for %s: %v %q", e.id, p.Header, data)
		}
	}
	if _, err := mr.NextPart(); err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestWriteBlockListMultipartEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	if err := writeBlockListMultipart(w, 200, &BlockListMultipart{}); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 {
		t.Errorf("status %d", w.Code)
	}
}
