package host

import (
	"encoding/json"
	"os"

	lmcode "github.com/lmctl-ai/lmcode-go"
)

// LoadConfig reads execution inputs without including their contents in errors.
func LoadConfig(workflowPath, connectorPath, credentialsPath, baseURL string) (Config, error) {
	workflow, err := loadJSON(workflowPath, "workflow")
	if err != nil {
		return Config{}, err
	}
	connector, err := loadJSON(connectorPath, "connector")
	if err != nil {
		return Config{}, err
	}
	raw, err := os.ReadFile(credentialsPath)
	if err != nil {
		return Config{}, lmcode.NewHostError("cannot read credentials JSON")
	}
	credentials, err := parseCredentials(raw)
	if err != nil {
		return Config{}, err
	}
	return Config{Workflow: workflow, Connector: connector, Credentials: credentials, BaseURL: baseURL}, nil
}

func loadJSON(path, name string) (*lmcode.JSON, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, lmcode.NewHostError("cannot read " + name + " JSON")
	}
	document, err := lmcode.ParseJSON(string(raw))
	if err != nil {
		return nil, lmcode.NewHostError("invalid " + name + " JSON")
	}
	return document, nil
}

func parseCredentials(raw []byte) (map[string]map[string]string, error) {
	var credentials map[string]map[string]string
	if err := json.Unmarshal(raw, &credentials); err != nil || credentials == nil {
		return nil, lmcode.NewHostError("invalid credentials JSON")
	}
	return credentials, nil
}
