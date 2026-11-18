package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	lmcode "github.com/lmctl-ai/lmcode-go"
)

func TestJSONPutPreservesScalarTypes(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{`null`, `null`},
		{`true`, `true`},
		{`false`, `false`},
		{`42`, `42`},
		{`9007199254740993`, `9007199254740993`},
		{`1.25`, `1.25`},
		{`"text"`, `"text"`},
		{`[1,false,"x"]`, `[1,false,"x"]`},
		{`{"nested":null}`, `{"nested":null}`},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			result, err := putJSON(`{"keep":9007199254740993}`, "added", test.value)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal([]byte(result), &object); err != nil {
				t.Fatal(err)
			}
			if got := string(object["added"]); got != test.want {
				t.Fatalf("added value = %s; want %s", got, test.want)
			}
			if got := string(object["keep"]); got != `9007199254740993` {
				t.Fatalf("existing number changed to %s", got)
			}
		})
	}
}

func TestJSONPutReturnsNewDocument(t *testing.T) {
	registry, err := NewRegistry(Config{})
	if err != nil {
		t.Fatal(err)
	}
	document, err := lmcode.ParseJSON(`{"old":1}`)
	if err != nil {
		t.Fatal(err)
	}
	value, err := lmcode.ParseJSON(`false`)
	if err != nil {
		t.Fatal(err)
	}
	before := document.ToJSON()
	result, err := registry.FindFunction("json_put").Fn([]lmcode.Value{
		lmcode.Object("Json", document), lmcode.String("new"), lmcode.Object("Json", value),
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, ok := result.Obj.(*lmcode.JSON)
	if !ok || updated.ToJSON() != `{"new":false,"old":1}` {
		t.Fatalf("unexpected json_put result: %#v", result)
	}
	if document.ToJSON() != before {
		t.Fatalf("json_put mutated its input: %s", document.ToJSON())
	}
	if _, err := putJSON(`[1,2]`, "key", `true`); err == nil {
		t.Fatal("json_put accepted a non-object document")
	}
}

func TestCredentialErrorsDoNotLeakSecrets(t *testing.T) {
	const secret = "123456:super-secret-token"
	h := &handler{credentials: map[string]map[string]string{
		"working":             {"token": secret},
		"empty-private-alias": {"token": ""},
	}}
	if got, err := h.credentialValue("working", "token"); err != nil || got != secret {
		t.Fatalf("valid credential = %q, %v", got, err)
	}
	for _, test := range [][2]string{{"missing-private-alias", "token"}, {"working", "missing-private-field"}, {"empty-private-alias", "token"}} {
		_, err := h.credentialValue(test[0], test[1])
		if err == nil {
			t.Fatal("missing credential returned no error")
		}
		var native *lmcode.Error
		if !errors.As(err, &native) || native.Code != lmcode.RUN_HOST {
			t.Fatalf("credential error = %#v", err)
		}
		for _, private := range []string{secret, test[0], test[1]} {
			if strings.Contains(err.Error(), private) {
				t.Fatalf("credential error leaked %q: %v", private, err)
			}
		}
	}
	if _, err := parseCredentials([]byte(`{"working":{"token":"` + secret + `"`)); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("malformed credentials error = %v", err)
	}
}

func TestSchemaAndRegistryConsistency(t *testing.T) {
	var schema struct {
		Functions map[string][]struct {
			Params  []string `json:"params"`
			Returns string   `json:"returns"`
		} `json:"functions"`
		Types map[string]struct {
			Methods map[string][]struct {
				Params  []string `json:"params"`
				Returns string   `json:"returns"`
			} `json:"methods"`
		} `json:"types"`
	}
	if err := json.Unmarshal(Schema, &schema); err != nil {
		t.Fatal(err)
	}
	var expected []string
	for name, overloads := range schema.Functions {
		for _, overload := range overloads {
			expected = append(expected, "fn "+name+lmcode.Signature{Params: overload.Params, Returns: overload.Returns}.Text())
		}
	}
	for typeName, typeSchema := range schema.Types {
		for name, overloads := range typeSchema.Methods {
			for _, overload := range overloads {
				expected = append(expected, "method "+typeName+"."+name+lmcode.Signature{Params: overload.Params, Returns: overload.Returns}.Text())
			}
		}
	}
	sort.Strings(expected)
	registry, err := NewRegistry(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.HostSignatures(); !slices.Equal(got, expected) {
		t.Fatalf("registry signatures:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
	contract, err := lmcode.ResultContractFromSchema(Schema)
	if err != nil {
		t.Fatal(err)
	}
	if contract.Type() != "Json" || contract.Nullable() {
		t.Fatalf("result contract = %s, nullable %t", contract.Type(), contract.Nullable())
	}
	if len(schema.Functions) != 6 || len(schema.Types["HttpResponse"].Methods) != 2 {
		t.Fatal(fmt.Sprintf("unexpected schema shape: %d functions", len(schema.Functions)))
	}
}
