package host

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	lmcode "github.com/lmctl-ai/lmcode-go"
)

const maxHTTPBody = 1 << 20

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type httpResponse struct {
	status int64
	body   *lmcode.JSON
}

func (h *handler) httpPostJSON(args []lmcode.Value) (lmcode.Value, error) {
	document, ok := args[1].Obj.(*lmcode.JSON)
	if !ok || document == nil {
		return lmcode.None(), lmcode.NewHostError("http_post_json: expected Json body")
	}
	response, err := h.postJSON(args[0].Str, document)
	if err != nil {
		return lmcode.None(), err
	}
	return lmcode.Object("HttpResponse", response), nil
}

func (h *handler) postJSON(rawURL string, document *lmcode.JSON) (*httpResponse, error) {
	if !validHTTPURL(rawURL) {
		return nil, lmcode.NewHostError("http_post_json: invalid URL")
	}
	body := []byte(document.ToJSON())
	if len(body) > maxHTTPBody {
		return nil, lmcode.NewHostError("http_post_json: request body exceeds 1 MiB")
	}
	request, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, lmcode.NewHostError("http_post_json: invalid URL")
	}
	request.Header.Set("Content-Type", "application/json")
	client := h.client
	if client == nil {
		client = newHTTPClient()
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, lmcode.NewHostError("http_post_json: request failed")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxHTTPBody+1))
	if err != nil {
		return nil, lmcode.NewHostError("http_post_json: response read failed")
	}
	if len(responseBody) > maxHTTPBody {
		return nil, lmcode.NewHostError("http_post_json: response body exceeds 1 MiB")
	}
	parsed, err := lmcode.ParseJSON(string(responseBody))
	if err != nil {
		return nil, lmcode.NewHostError("http_post_json: invalid JSON response")
	}
	return &httpResponse{status: int64(response.StatusCode), body: parsed}, nil
}

func validHTTPURL(raw string) bool {
	if raw == "" || !utf8.ValidString(raw) || strings.ContainsAny(raw, "?#") {
		return false
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" && parsed.RawFragment == ""
}

func httpResponseStatus(self any, _ []lmcode.Value) (lmcode.Value, error) {
	response, ok := self.(*httpResponse)
	if !ok || response == nil {
		return lmcode.None(), lmcode.NewHostError("HttpResponse.status: invalid receiver")
	}
	return lmcode.Int(response.status), nil
}

func httpResponseJSON(self any, _ []lmcode.Value) (lmcode.Value, error) {
	response, ok := self.(*httpResponse)
	if !ok || response == nil || response.body == nil {
		return lmcode.None(), lmcode.NewHostError("HttpResponse.json: invalid receiver")
	}
	return lmcode.Object("Json", response.body), nil
}
