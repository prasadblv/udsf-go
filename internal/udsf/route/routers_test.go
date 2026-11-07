package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRouter struct{ routes []Route }

func (f fakeRouter) Routes() Routes {
	m := Routes{}
	for _, r := range f.routes {
		m[r.Name] = r
	}
	return m
}

func (f fakeRouter) OrderedRoutes() []Route { return f.routes }

func handler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestNewRouterNoRouters(t *testing.T) {
	if w := do(NewRouter(), "GET", "/x"); w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", w.Code)
	}
}

func TestNewRouterRegistersRoutes(t *testing.T) {
	r := NewRouter(fakeRouter{routes: []Route{
		{Name: "get", Method: "GET", Pattern: "/things/{id}", HandlerFunc: handler("get")},
		{Name: "put", Method: "PUT", Pattern: "/things/{id}", HandlerFunc: handler("put")},
	}})

	if w := do(r, "GET", "/things/1"); w.Code != 200 || w.Body.String() != "get" {
		t.Errorf("GET: %d %q", w.Code, w.Body.String())
	}
	if w := do(r, "PUT", "/things/1"); w.Code != 200 || w.Body.String() != "put" {
		t.Errorf("PUT: %d %q", w.Code, w.Body.String())
	}
	if w := do(r, "DELETE", "/things/1"); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE: %d, want 405", w.Code)
	}
	if w := do(r, "GET", "/unknown"); w.Code != http.StatusNotFound {
		t.Errorf("unknown path: %d, want 404", w.Code)
	}
}

func TestNewRouterMultipleRouters(t *testing.T) {
	r := NewRouter(
		fakeRouter{routes: []Route{{Name: "a", Method: "GET", Pattern: "/a", HandlerFunc: handler("A")}}},
		fakeRouter{routes: []Route{{Name: "b", Method: "GET", Pattern: "/b", HandlerFunc: handler("B")}}},
	)
	if w := do(r, "GET", "/a"); w.Body.String() != "A" {
		t.Errorf("/a: %q", w.Body.String())
	}
	if w := do(r, "GET", "/b"); w.Body.String() != "B" {
		t.Errorf("/b: %q", w.Body.String())
	}
}

func TestNewRouterNamedRoutesAndPathVars(t *testing.T) {
	var got string
	r := NewRouter(fakeRouter{routes: []Route{{
		Name: "byID", Method: "GET", Pattern: "/things/{id}",
		HandlerFunc: func(w http.ResponseWriter, req *http.Request) {
			got = req.URL.Path
		},
	}}})

	if r.Get("byID") == nil {
		t.Fatal("route should be registered by name")
	}
	u, err := r.Get("byID").URL("id", "42")
	if err != nil || u.Path != "/things/42" {
		t.Errorf("url build: %v %v", u, err)
	}
	do(r, "GET", "/things/42")
	if got != "/things/42" {
		t.Errorf("handler got %q", got)
	}
}

func TestNewRouterStrictSlash(t *testing.T) {
	r := NewRouter(fakeRouter{routes: []Route{
		{Name: "x", Method: "GET", Pattern: "/x/", HandlerFunc: handler("x")},
	}})
	w := do(r, "GET", "/x")
	if w.Code != http.StatusMovedPermanently || !strings.HasSuffix(w.Header().Get("Location"), "/x/") {
		t.Errorf("expected redirect to /x/, got %d %v", w.Code, w.Header())
	}
}

func TestFakeRouterImplementsRouter(t *testing.T) {
	var _ Router = fakeRouter{}
	f := fakeRouter{routes: []Route{{Name: "n"}}}
	if _, ok := f.Routes()["n"]; !ok {
		t.Error("Routes map should be keyed by name")
	}
}
