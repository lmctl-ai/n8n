package host

import (
	"bytes"
	"encoding/json"
	"net/http"

	lmcode "github.com/lmctl-ai/lmcode-go"
)

// Config contains the preloaded, trusted inputs available to LMCode.
type Config struct {
	Workflow    *lmcode.JSON
	Connector   *lmcode.JSON
	Credentials map[string]map[string]string
	BaseURL     string
}

type handler struct {
	workflow    *lmcode.JSON
	connector   *lmcode.JSON
	credentials map[string]map[string]string
	baseURL     string
	client      *http.Client
}

// NewRegistry registers exactly the host functions and methods declared by Schema.
func NewRegistry(config Config) (*lmcode.Registry, error) {
	h := &handler{
		workflow:    config.Workflow,
		connector:   config.Connector,
		credentials: config.Credentials,
		baseURL:     config.BaseURL,
		client:      newHTTPClient(),
	}
	reg := lmcode.NewRegistry()
	functions := []struct {
		name    string
		params  []string
		returns string
		fn      lmcode.Function
	}{
		{"input", []string{"string"}, "Json", h.input},
		{"credential", []string{"string", "string"}, "string", h.credential},
		{"base_url", []string{"string"}, "string", h.baseURLFor},
		{"json_put", []string{"Json", "string", "Json"}, "Json", h.jsonPut},
		{"http_post_json", []string{"string", "Json"}, "HttpResponse", h.httpPostJSON},
		{"workflow_fail", []string{"string"}, "void", func(args []lmcode.Value) (lmcode.Value, error) {
			return lmcode.None(), lmcode.NewHostError(args[0].Str)
		}},
	}
	for _, function := range functions {
		if err := reg.Function(function.name, function.params, function.returns, function.fn); err != nil {
			return nil, err
		}
	}
	methods := []struct {
		name    string
		returns string
		fn      lmcode.Method
	}{
		{"status", "int", httpResponseStatus},
		{"json", "Json", httpResponseJSON},
	}
	for _, method := range methods {
		if err := reg.Method("HttpResponse", method.name, nil, method.returns, method.fn); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

func (h *handler) input(args []lmcode.Value) (lmcode.Value, error) {
	var document *lmcode.JSON
	switch args[0].Str {
	case "workflow":
		document = h.workflow
	case "connector":
		document = h.connector
	default:
		return lmcode.None(), lmcode.NewHostError("input: expected workflow or connector")
	}
	if document == nil {
		return lmcode.None(), lmcode.NewHostError("input: document is unavailable")
	}
	return lmcode.Object("Json", document), nil
}

func (h *handler) credential(args []lmcode.Value) (lmcode.Value, error) {
	value, err := h.credentialValue(args[0].Str, args[1].Str)
	if err != nil {
		return lmcode.None(), err
	}
	return lmcode.String(value), nil
}

func (h *handler) credentialValue(alias, field string) (string, error) {
	fields, ok := h.credentials[alias]
	if !ok {
		return "", lmcode.NewHostError("credential: missing or empty credential")
	}
	value, ok := fields[field]
	if !ok || value == "" {
		return "", lmcode.NewHostError("credential: missing or empty credential")
	}
	return value, nil
}

func (h *handler) baseURLFor(args []lmcode.Value) (lmcode.Value, error) {
	if h.baseURL != "" {
		return lmcode.String(h.baseURL), nil
	}
	return lmcode.String(args[0].Str), nil
}

func (h *handler) jsonPut(args []lmcode.Value) (lmcode.Value, error) {
	document, ok := args[0].Obj.(*lmcode.JSON)
	if !ok || document == nil {
		return lmcode.None(), lmcode.NewHostError("json_put: expected Json document")
	}
	value, ok := args[2].Obj.(*lmcode.JSON)
	if !ok || value == nil {
		return lmcode.None(), lmcode.NewHostError("json_put: expected Json value")
	}
	text, err := putJSON(document.ToJSON(), args[1].Str, value.ToJSON())
	if err != nil {
		return lmcode.None(), err
	}
	parsed, err := lmcode.ParseJSON(text)
	if err != nil {
		return lmcode.None(), lmcode.NewHostError("json_put: invalid JSON result")
	}
	return lmcode.Object("Json", parsed), nil
}

func putJSON(document, key, value string) (string, error) {
	trimmed := bytes.TrimSpace([]byte(document))
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) || !json.Valid([]byte(value)) {
		return "", lmcode.NewHostError("json_put: invalid JSON object or value")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return "", lmcode.NewHostError("json_put: invalid JSON object")
	}
	object[key] = json.RawMessage(bytes.Clone([]byte(value)))
	encoded, err := json.Marshal(object)
	if err != nil {
		return "", lmcode.NewHostError("json_put: cannot encode JSON object")
	}
	return string(encoded), nil
}
