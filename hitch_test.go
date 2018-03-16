package hitch

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/nbio/st"
)

func TestHome(t *testing.T) {
	s := newTestServer(t)
	_, res := s.request("GET", "/")
	defer res.Body.Close()
	expectHeaders(t, res)
}

func TestCopied(t *testing.T) {
	s := newCopiedTestServer(t)
	_, res := s.request("GET", "/")
	defer res.Body.Close()
	st.Expect(t, res.StatusCode, 404)

	_, res2 := s.request("GET", "/sub")
	defer res2.Body.Close()
	expectHeaders(t, res2)
	st.Expect(t, res2.StatusCode, 200)
}

func TestEcho(t *testing.T) {
	s := newTestServer(t)
	_, res := s.request("GET", "/api/echo/hip-hop")
	defer res.Body.Close()
	expectHeaders(t, res)
	body, _ := ioutil.ReadAll(res.Body)
	st.Assert(t, string(body), "hip-hop")
}

func TestRouteMiddleware(t *testing.T) {
	s := newTestServer(t)
	_, res := s.request("GET", "/route_middleware")
	defer res.Body.Close()
	expectHeaders(t, res)
	body, _ := ioutil.ReadAll(res.Body)
	st.Assert(t, string(body), "middleware1 -> middleware2 -> Hello, world! -> middleware2 -> middleware1")
}

func TestHandleIf(t *testing.T) {
	s := newTestServer2(t)
	{
		_, res := s.request("GET", "/", map[string]string{
			"x-route": "next",
		})
		defer res.Body.Close()
		expectHeaders(t, res)
		body, _ := ioutil.ReadAll(res.Body)
		st.Assert(t, string(body), "next")
	}

	{
		_, res := s.request("GET", "/", map[string]string{
			"x-route": "fallback",
		})
		defer res.Body.Close()
		expectHeaders(t, res)
		body, _ := ioutil.ReadAll(res.Body)
		st.Assert(t, string(body), "Hello, world!")
	}
}

func expectHeaders(t *testing.T, res *http.Response) {
	st.Expect(t, res.Header.Get("Content-Type"), "text/plain")
	st.Expect(t, res.Header.Get("X-Awesome"), "awesome")
}

// testServer

type testServer struct {
	*httptest.Server
	t *testing.T
}

func (s *testServer) request(method, path string, headers ...map[string]string) (*http.Request, *http.Response) {
	req, err := http.NewRequest(method, s.URL+path, nil)
	if err != nil {
		s.t.Fatal(err)
	}
	for _, header := range headers {
		for k, v := range header {
			req.Header.Set(k, v)
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	return req, res
}

func newTestServer(t *testing.T) *testServer {
	h := New()
	h.Use(logger, plaintext)
	h.UseHandler(http.HandlerFunc(awesome))
	h.HandleFunc("GET", "/", home)
	api := New()
	api.Get("/api/echo/:phrase", http.HandlerFunc(echo))
	h.Next(api.Handler())
	h.Get("/route_middleware", http.HandlerFunc(home), testMiddleware("middleware1"), testMiddleware("middleware2"))

	s := &testServer{httptest.NewServer(h.Handler()), t}
	runtime.SetFinalizer(s, func(s *testServer) { s.Server.Close() })
	return s
}

func newTestServer2(t *testing.T) *testServer {
	h := New()
	h.Use(logger, plaintext)
	h.UseHandler(http.HandlerFunc(awesome))
	h.HandleIf(func(next, fallback http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Header.Get("x-route") == "next" {
				next.ServeHTTP(w, req)
			} else {
				fallback.ServeHTTP(w, req)
			}
		})
	}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, "next")
	}))
	h.HandleFunc("GET", "/", home)

	s := &testServer{httptest.NewServer(h.Handler()), t}
	runtime.SetFinalizer(s, func(s *testServer) { s.Server.Close() })
	return s
}

func newCopiedTestServer(t *testing.T) *testServer {
	h := New()
	h.Use(logger, plaintext)
	h.UseHandler(http.HandlerFunc(awesome))
	h.HandleFunc("GET", "/", home)

	sub := New()
	sub.Use(h.Middleware...)
	// no handler on /
	sub.HandleFunc("GET", "/sub", home)

	s := &testServer{httptest.NewServer(sub.Handler()), t}
	runtime.SetFinalizer(s, func(s *testServer) { s.Server.Close() })
	return s
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Printf("%s %s\n", req.Method, req.URL.String())
		next.ServeHTTP(w, req)
	})
}

func plaintext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		next.ServeHTTP(w, req)
	})
}

func awesome(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("X-Awesome", "awesome")
}

func home(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}

func echo(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, Params(req).ByName("phrase"))
}

func testMiddleware(name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprint(w, name+" -> ")
			next.ServeHTTP(w, req)
			fmt.Fprint(w, " -> "+name)
		})
	}
}
