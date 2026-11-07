package util

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func intPtr(i int) *int { return &i }

func TestIsZeroValue(t *testing.T) {
	type s struct{ A int }
	tests := []struct {
		name string
		v    interface{}
		want bool
	}{
		{"nil", nil, true},
		{"zero int", 0, true},
		{"non-zero int", 1, false},
		{"empty string", "", true},
		{"string", "a", false},
		{"zero struct", s{}, true},
		{"struct", s{1}, false},
		{"nil slice", []int(nil), true},
		{"empty slice", []int{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsZeroValue(tt.v); got != tt.want {
				t.Errorf("got %v", got)
			}
		})
	}
}

type item struct{ N int }

func TestAssertRecurseInterfaceRequired(t *testing.T) {
	var seen []int
	cb := func(i item) error { seen = append(seen, i.N); return nil }

	err := AssertRecurseInterfaceRequired([][]item{{{1}, {2}}, {{3}}}, cb)
	if err != nil || !reflect.DeepEqual(seen, []int{1, 2, 3}) {
		t.Errorf("err=%v seen=%v", err, seen)
	}

	want := errors.New("stop")
	seen = nil
	err = AssertRecurseInterfaceRequired([]item{{1}, {2}}, func(i item) error {
		seen = append(seen, i.N)
		return want
	})
	if err != want || len(seen) != 1 {
		t.Errorf("callback error not propagated: %v %v", err, seen)
	}

	if err := AssertRecurseInterfaceRequired([]struct{ X int }{{1}}, cb); !errors.Is(err, ErrTypeAssertionError) {
		t.Errorf("expected type assertion error, got %v", err)
	}

	if err := AssertRecurseInterfaceRequired("not a struct", cb); err != nil {
		t.Errorf("non struct/slice should be ignored: %v", err)
	}
}

func TestEncodeJSONResponse(t *testing.T) {
	t.Run("json default status", func(t *testing.T) {
		w := httptest.NewRecorder()
		if err := EncodeJSONResponse(map[string]int{"a": 1}, nil, w); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || w.Header().Get("Content-Type") != "application/json; charset=UTF-8" ||
			strings.TrimSpace(w.Body.String()) != `{"a":1}` {
			t.Errorf("%d %v %q", w.Code, w.Header(), w.Body.String())
		}
	})
	t.Run("custom status nil body", func(t *testing.T) {
		w := httptest.NewRecorder()
		if err := EncodeJSONResponse(nil, intPtr(204), w); err != nil {
			t.Fatal(err)
		}
		if w.Code != 204 || w.Body.Len() != 0 {
			t.Errorf("%d %q", w.Code, w.Body.String())
		}
	})
	t.Run("block content", func(t *testing.T) {
		w := httptest.NewRecorder()
		_ = EncodeJSONResponse(&BlockContent{ContentType: "text/plain", Data: []byte("x")}, intPtr(201), w)
		if w.Code != 201 || w.Header().Get("Content-Type") != "text/plain" || w.Body.String() != "x" {
			t.Errorf("%d %v %q", w.Code, w.Header(), w.Body.String())
		}
	})
	t.Run("block list", func(t *testing.T) {
		w := httptest.NewRecorder()
		_ = EncodeJSONResponse(&BlockListMultipart{Parts: []BlockPart{{ID: "a"}}}, nil, w)
		if !strings.HasPrefix(w.Header().Get("Content-Type"), "multipart/parallel; boundary=") {
			t.Errorf("%v", w.Header())
		}
	})
	t.Run("file", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "f*.txt")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		f.WriteString("hello file")
		f.Seek(0, io.SeekStart)
		w := httptest.NewRecorder()
		if err := EncodeJSONResponse(f, nil, w); err != nil {
			t.Fatal(err)
		}
		if w.Body.String() != "hello file" || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") ||
			!strings.Contains(w.Header().Get("Content-Disposition"), f.Name()) {
			t.Errorf("%v %q", w.Header(), w.Body.String())
		}
		// with explicit status
		f.Seek(0, io.SeekStart)
		w = httptest.NewRecorder()
		_ = EncodeJSONResponse(f, intPtr(202), w)
		if w.Code != 202 {
			t.Errorf("status %d", w.Code)
		}
	})
	t.Run("unencodable", func(t *testing.T) {
		w := httptest.NewRecorder()
		if err := EncodeJSONResponse(make(chan int), nil, w); err == nil {
			t.Error("expected json error")
		}
	})
}

func multipartRequest(t *testing.T, key string, files map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, content := range files {
		fw, err := mw.CreateFormFile(key, name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(content))
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func TestReadFormFileToTempFile(t *testing.T) {
	r := multipartRequest(t, "file", map[string]string{"a.txt": "content"})
	f, err := ReadFormFileToTempFile(r, "file")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "content" {
		t.Errorf("got %q (file should be rewound)", data)
	}

	if _, err := ReadFormFileToTempFile(multipartRequest(t, "file", nil), "file"); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadFormFilesToTempFiles(t *testing.T) {
	r := multipartRequest(t, "files", map[string]string{"a.txt": "A"})
	files, err := ReadFormFilesToTempFiles(r, "files")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files", len(files))
	}
	defer os.Remove(files[0].Name())
	defer files[0].Close()
	if data, _ := io.ReadAll(files[0]); string(data) != "A" {
		t.Errorf("got %q", data)
	}

	files, err = ReadFormFilesToTempFiles(multipartRequest(t, "files", map[string]string{"a": "A"}), "other")
	if err != nil || len(files) != 0 {
		t.Errorf("unknown key: %v %v", files, err)
	}

	// not multipart
	if _, err := ReadFormFilesToTempFiles(httptest.NewRequest("POST", "/", strings.NewReader("x")), "files"); err == nil {
		t.Error("expected error for non-multipart request")
	}
}

func TestParseTime(t *testing.T) {
	if got, err := parseTime(""); err != nil || !got.IsZero() {
		t.Errorf("empty: %v %v", got, err)
	}
	got, err := parseTime("2024-01-02T03:04:05Z")
	if err != nil || !got.Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("%v %v", got, err)
	}
	if _, err := parseTime("bad"); err == nil {
		t.Error("expected error")
	}
}

func TestParseTimes(t *testing.T) {
	got, err := parseTimes("2024-01-02T03:04:05Z,2025-01-02T03:04:05Z")
	if err != nil || len(got) != 2 || got[1].Year() != 2025 {
		t.Errorf("%v %v", got, err)
	}
	if _, err := parseTimes("2024-01-02T03:04:05Z,bad"); err == nil {
		t.Error("expected error")
	}
}

func TestParsePrimitives(t *testing.T) {
	if v, err := parseFloat64(""); v != 0 || err != nil {
		t.Error("float64 empty")
	}
	if v, err := parseFloat64("1.5"); v != 1.5 || err != nil {
		t.Error("float64")
	}
	if _, err := parseFloat64("x"); err == nil {
		t.Error("float64 err")
	}
	if v, err := parseFloat32(""); v != 0 || err != nil {
		t.Error("float32 empty")
	}
	if v, err := parseFloat32("2.5"); v != 2.5 || err != nil {
		t.Error("float32")
	}
	if _, err := parseFloat32("x"); err == nil {
		t.Error("float32 err")
	}
	if v, err := parseInt64(""); v != 0 || err != nil {
		t.Error("int64 empty")
	}
	if v, err := parseInt64("9999999999"); v != 9999999999 || err != nil {
		t.Error("int64")
	}
	if _, err := parseInt64("1.5"); err == nil {
		t.Error("int64 err")
	}
	if v, err := parseInt32(""); v != 0 || err != nil {
		t.Error("int32 empty")
	}
	if v, err := ParseInt32("-42"); v != -42 || err != nil {
		t.Error("int32")
	}
	if _, err := ParseInt32("9999999999"); err == nil {
		t.Error("int32 overflow should error")
	}
	if v, err := ParseBool(""); v || err != nil {
		t.Error("bool empty")
	}
	if v, err := ParseBool("true"); !v || err != nil {
		t.Error("bool")
	}
	if _, err := ParseBool("maybe"); err == nil {
		t.Error("bool err")
	}
}

func TestOperations(t *testing.T) {
	req := WithRequire(parseInt32)
	if _, _, err := req(""); err == nil || err.Error() != ErrMsgRequiredMissing {
		t.Errorf("require missing: %v", err)
	}
	if v, def, err := req("7"); v != 7 || def || err != nil {
		t.Errorf("require: %v %v %v", v, def, err)
	}

	dp := WithDefaultOrParse(int32(5), parseInt32)
	if v, def, err := dp(""); v != 5 || !def || err != nil {
		t.Errorf("default: %v %v %v", v, def, err)
	}
	if v, def, err := dp("9"); v != 9 || def || err != nil {
		t.Errorf("parse: %v %v %v", v, def, err)
	}

	p := WithParse(parseInt32)
	if _, _, err := p("x"); err == nil {
		t.Error("expected parse error")
	}
	if v, _, err := p("3"); v != 3 || err != nil {
		t.Error("parse ok")
	}
}

func TestConstraints(t *testing.T) {
	min, max := WithMinimum(int32(5)), WithMaximum(int32(10))
	if err := min(5); err != nil {
		t.Error("min boundary")
	}
	if err := min(4); err == nil || err.Error() != ErrMsgMinValueConstraint {
		t.Errorf("min: %v", err)
	}
	if err := max(10); err != nil {
		t.Error("max boundary")
	}
	if err := max(11); err == nil || err.Error() != ErrMsgMaxValueConstraint {
		t.Errorf("max: %v", err)
	}
}

func TestParseNumericParameter(t *testing.T) {
	min, max := WithMinimum(int32(1)), WithMaximum(int32(10))

	if v, err := ParseNumericParameter("5", WithRequire(parseInt32), min, max); v != 5 || err != nil {
		t.Errorf("%v %v", v, err)
	}
	if _, err := ParseNumericParameter("", WithRequire(parseInt32)); err == nil {
		t.Error("required missing")
	}
	if _, err := ParseNumericParameter("0", WithRequire(parseInt32), min); err == nil {
		t.Error("below min")
	}
	if _, err := ParseNumericParameter("11", WithRequire(parseInt32), min, max); err == nil {
		t.Error("above max")
	}
	if _, err := ParseNumericParameter("x", WithParse(parseInt32)); err == nil {
		t.Error("parse error")
	}
	// default values skip constraint checks
	if v, err := ParseNumericParameter("", WithDefaultOrParse(int32(100), parseInt32), max); v != 100 || err != nil {
		t.Errorf("default should bypass checks: %v %v", v, err)
	}
}

func TestParseBoolParameter(t *testing.T) {
	if v, err := ParseBoolParameter("true", WithParse(parseBool)); !v || err != nil {
		t.Error("true")
	}
	if _, err := ParseBoolParameter("", WithRequire(parseBool)); err == nil {
		t.Error("required")
	}
	if v, err := ParseBoolParameter("", WithDefaultOrParse(true, parseBool)); !v || err != nil {
		t.Error("default")
	}
}

func TestParseNumericArrayParameter(t *testing.T) {
	op := WithParse(parseInt32)
	max := WithMaximum(int32(5))

	if v, err := parseNumericArrayParameter("1,2,3", ",", false, op, max); err != nil || !reflect.DeepEqual(v, []int32{1, 2, 3}) {
		t.Errorf("%v %v", v, err)
	}
	if v, err := parseNumericArrayParameter("", ",", false, op); v != nil || err != nil {
		t.Errorf("optional empty: %v %v", v, err)
	}
	if _, err := parseNumericArrayParameter("", ",", true, op); err == nil {
		t.Error("required empty")
	}
	if _, err := parseNumericArrayParameter("1,x", ",", false, op); err == nil {
		t.Error("parse error")
	}
	if _, err := parseNumericArrayParameter("1,9", ",", false, op, max); err == nil {
		t.Error("constraint error")
	}
	if v, err := parseNumericArrayParameter("1;2", ";", false, op); err != nil || len(v) != 2 {
		t.Errorf("custom delimiter: %v %v", v, err)
	}
}

func TestParseQuery(t *testing.T) {
	v, err := ParseQuery("a=1&b=2&a=3")
	if err != nil || !reflect.DeepEqual(v["a"], []string{"1", "3"}) || v.Get("b") != "2" {
		t.Errorf("%v %v", v, err)
	}
	if _, err := ParseQuery("a=%zz"); err == nil {
		t.Error("expected error for malformed query")
	}
}
