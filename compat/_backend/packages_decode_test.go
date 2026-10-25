package integrations

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Copied into the backend's integrations package by the compat job of
// fuseone-io/fusescope-integrations; FUSESCOPE_INTEGRATION_PACKAGES holds the
// output of `fsint render`.
func renderedPackages(t *testing.T) map[string]IntegrationDetails {
	t.Helper()
	dir := os.Getenv("FUSESCOPE_INTEGRATION_PACKAGES")
	if dir == "" {
		t.Skip("FUSESCOPE_INTEGRATION_PACKAGES is not set")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no rendered packages in %s: %v", dir, err)
	}

	out := map[string]IntegrationDetails{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var details IntegrationDetails
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&details); err != nil {
			t.Fatalf("%s: the backend rejects this package: %v", filepath.Base(file), err)
		}
		if err := validateIntegration(details); err != nil {
			t.Fatalf("%s: %v", filepath.Base(file), err)
		}
		out[details.Id] = details
	}
	return out
}

func TestIntegrationPackagesDecode(t *testing.T) {
	for id, details := range renderedPackages(t) {
		if details.ConnectionTests == nil {
			t.Errorf("%s: connection_tests is required", id)
		}
	}
}
