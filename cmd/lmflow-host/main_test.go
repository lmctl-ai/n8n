package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/lmctl-ai/n8n/internal/host"
)

func TestSchemaInspectionUsesExactEmbeddedBytes(t *testing.T) {
	t.Setenv("LMCODE_INVOCATION_FILE", t.TempDir()+"/must-not-be-read")
	var output bytes.Buffer
	if status := run([]string{"--lmcode-schema", "--json"}, &output); status != 0 {
		t.Fatalf("status = %d", status)
	}
	expected, err := os.ReadFile("../../internal/host/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), expected) || !bytes.Equal(output.Bytes(), host.Schema) {
		t.Fatal("schema inspection output differs from schema.json")
	}
}

func TestSignatureInspection(t *testing.T) {
	t.Setenv("LMCODE_INVOCATION_FILE", t.TempDir()+"/must-not-be-read")
	registry, err := host.NewRegistry(host.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if status := run([]string{"--lmcode-signatures"}, &output); status != 0 {
		t.Fatalf("status = %d", status)
	}
	expected := strings.Join(registry.Signatures(), "\n") + "\n"
	if output.String() != expected {
		t.Fatal("signature inspection output differs from registry")
	}
}

func TestExecutionErrorsHaveOneTerminalEnvelope(t *testing.T) {
	var output bytes.Buffer
	if status := run(nil, &output); status != 2 {
		t.Fatalf("status = %d; output = %q", status, output.String())
	}
	if strings.Count(output.String(), "LMCODE_RETURN_BEGIN") != 1 || strings.Count(output.String(), "LMCODE_RETURN_END") != 1 {
		t.Fatalf("invalid terminal envelope: %q", output.String())
	}
}
