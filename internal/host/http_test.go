package host

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	lmcode "github.com/lmctl-ai/lmcode-go"
)

func testJSON(t *testing.T, raw string) *lmcode.JSON {
	t.Helper()
	document, err := lmcode.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func TestHTTPPostJSONSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		if string(body) != `{"chat_id":41,"text":"hello"}` {
			t.Errorf("request body = %s", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"ok":true,"result":{"id":7}}`)
	}))
	defer server.Close()
	h := &handler{client: newHTTPClient()}
	response, err := h.postJSON(server.URL+"/botsecret/sendMessage", testJSON(t, `{"chat_id":41,"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	if response.status != http.StatusCreated || response.body.ToJSON() != `{"ok":true,"result":{"id":7}}` {
		t.Fatalf("response = %#v", response)
	}
	status, err := httpResponseStatus(response, nil)
	if err != nil || status.Int != http.StatusCreated {
		t.Fatalf("status method = %#v, %v", status, err)
	}
	body, err := httpResponseJSON(response, nil)
	if err != nil || body.Obj.(*lmcode.JSON) != response.body {
		t.Fatalf("json method = %#v, %v", body, err)
	}
	if h.client.Timeout != 10*time.Second {
		t.Fatalf("HTTP timeout = %s", h.client.Timeout)
	}
}

func TestHTTPPostJSONReturnsNon2xxJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"ok":false,"description":"limited"}`)
	}))
	defer server.Close()
	h := &handler{client: newHTTPClient()}
	response, err := h.postJSON(server.URL+"/botsecret/sendMessage", testJSON(t, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if response.status != http.StatusTooManyRequests || response.body.ToJSON() != `{"ok":false,"description":"limited"}` {
		t.Fatalf("non-2xx response = %#v", response)
	}
}

func TestHTTPPostJSONRejectsInvalidResponseJSONWithoutLeakingURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `not JSON`)
	}))
	defer server.Close()
	rawURL := server.URL + "/bot-super-secret-token/sendMessage"
	h := &handler{client: newHTTPClient()}
	_, err := h.postJSON(rawURL, testJSON(t, `{}`))
	if err == nil || err.Error() != "http_post_json: invalid JSON response" {
		t.Fatalf("invalid JSON error = %v", err)
	}
	if strings.Contains(err.Error(), rawURL) || strings.Contains(err.Error(), "super-secret-token") {
		t.Fatalf("error leaked URL: %v", err)
	}
}

func TestHTTPPostJSONBodyLimits(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			requests.Add(1)
		}))
		defer server.Close()
		rawURL := server.URL + "/bot-super-secret-token"
		h := &handler{client: newHTTPClient()}
		_, err := h.postJSON(rawURL, testJSON(t, `"`+strings.Repeat("x", maxHTTPBody)+`"`))
		if err == nil || err.Error() != "http_post_json: request body exceeds 1 MiB" {
			t.Fatalf("oversize request error = %v", err)
		}
		if requests.Load() != 0 {
			t.Fatal("oversize request reached the server")
		}
		if strings.Contains(err.Error(), "super-secret-token") {
			t.Fatalf("error leaked URL: %v", err)
		}
	})
	t.Run("response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `"`+strings.Repeat("x", maxHTTPBody)+`"`)
		}))
		defer server.Close()
		rawURL := server.URL + "/bot-super-secret-token"
		h := &handler{client: newHTTPClient()}
		_, err := h.postJSON(rawURL, testJSON(t, `{}`))
		if err == nil || err.Error() != "http_post_json: response body exceeds 1 MiB" {
			t.Fatalf("oversize response error = %v", err)
		}
		if strings.Contains(err.Error(), "super-secret-token") {
			t.Fatalf("error leaked URL: %v", err)
		}
	})
}

func TestHTTPPostJSONDoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/followed")
		w.WriteHeader(http.StatusFound)
		_, _ = io.WriteString(w, `{"ok":false,"redirect":true}`)
	})
	mux.HandleFunc("/followed", func(w http.ResponseWriter, _ *http.Request) {
		followed.Store(true)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	h := &handler{client: newHTTPClient()}
	response, err := h.postJSON(server.URL+"/start", testJSON(t, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if response.status != http.StatusFound || response.body.ToJSON() != `{"ok":false,"redirect":true}` {
		t.Fatalf("redirect response = %#v", response)
	}
	if followed.Load() {
		t.Fatal("redirect target was requested")
	}
}

func TestHTTPPostJSONRejectsUnsafeURLShapes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	userinfo := strings.Replace(server.URL, "://", "://user:secret@", 1) + "/botsecret"
	urls := []string{
		"",
		"://malformed",
		"ftp://example.com/resource",
		userinfo,
		server.URL + "/botsecret?query=secret",
		server.URL + "/botsecret?",
		server.URL + "/botsecret#fragment",
		server.URL + "/botsecret#",
	}
	h := &handler{client: newHTTPClient()}
	for _, rawURL := range urls {
		_, err := h.postJSON(rawURL, testJSON(t, `{}`))
		if err == nil || err.Error() != "http_post_json: invalid URL" {
			t.Errorf("URL error = %v", err)
		}
		if err != nil && (rawURL != "" && strings.Contains(err.Error(), rawURL) || strings.Contains(err.Error(), "secret")) {
			t.Errorf("URL error leaked sensitive text: %v", err)
		}
	}
}
