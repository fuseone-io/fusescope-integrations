package integrations_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fuseone-io/fusescope-integrations/pkg/docs"
	"github.com/fuseone-io/fusescope-integrations/pkg/integration"
)

func loadRepo(t *testing.T) []*integration.Package {
	t.Helper()
	pkgs, err := integration.LoadAll(os.DirFS("integrations"), ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages under integrations/")
	}
	return pkgs
}

func TestPackagesAreValid(t *testing.T) {
	for _, p := range integration.ValidateAll(loadRepo(t)) {
		t.Error(p)
	}
}

func TestPackagesHaveCollectorSnippets(t *testing.T) {
	total := 0
	for _, p := range loadRepo(t) {
		d, err := integration.Render(p)
		if err != nil {
			t.Fatal(err)
		}
		snippets, err := integration.CollectorSnippets(p, d)
		if err != nil {
			t.Fatal(err)
		}
		total += len(snippets)
	}
	if total == 0 {
		t.Fatal("no collector config snippets found")
	}
}

func TestClickHouseQueryLogsNamesItsCollector(t *testing.T) {
	var clickhouse *integration.Package
	for _, p := range loadRepo(t) {
		if p.Manifest.ID == "builtin-clickhouse" {
			clickhouse = p
		}
	}
	if clickhouse == nil {
		t.Fatal("builtin-clickhouse not found")
	}
	d, err := integration.Render(clickhouse)
	if err != nil {
		t.Fatal(err)
	}
	var instructions string
	for _, step := range d.Configuration {
		if step.Title == "Collect Query Logs" {
			instructions = step.Instructions
		}
	}
	for _, expected := range []string{"clickhousesystemtablesreceiver", "fusescope-otel-collector", "otelcol-contrib", "sqlqueryreceiver"} {
		if !strings.Contains(instructions, expected) {
			t.Errorf("query logs step must mention %q", expected)
		}
	}
}

func TestDocsAreUpToDate(t *testing.T) {
	files, err := docs.Generate(loadRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("docs", name))
		if err != nil || string(got) != want {
			t.Errorf("docs/%s is out of date; run `go run ./cmd/fsint docs`", name)
		}
	}
}
