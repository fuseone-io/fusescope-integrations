package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

const validManifest = `manifest_version: 1
id: builtin-demo
version: 1.0.0
fusescope: ">=0.154.0"
title: Demo
description: Monitor demo with metrics and logs
author:
  name: FuseScope
  email: ""
  homepage: https://github.com/fuseone-io/fusescope
icon: icon.svg
categories:
  - Database
signals:
  - logs
  - metrics
overview: overview.md
configuration:
  - title: Collect Metrics
    instructions: config/collect-metrics.md
assets:
  logs:
    pipelines: []
  dashboards:
    - id: overview
      definition: assets/dashboards/overview.json
  alerts: []
data_collected: data_collected.json
connection_tests:
  logs:
    attribute_key: source
    attribute_value: demo
`

const validSnippet = "```yaml\n" + `receivers:
  demo:
    endpoint: ${env:DEMO_ENDPOINT}
processors:
  batch: {}
exporters:
  otlp/fusescope:
    endpoint: ${env:OTLP_DESTINATION_ENDPOINT}
service:
  pipelines:
    metrics/demo:
      receivers: [demo]
      processors: [batch]
      exporters: [otlp/fusescope]
` + "```\n\n```bash\nexport DEMO_ENDPOINT=\"<demo-host>\"\nexport OTLP_DESTINATION_ENDPOINT=\"<host>:4317\"\n```\n"

func validFS() fstest.MapFS {
	return fstest.MapFS{
		"manifest.yaml":                   {Data: []byte(validManifest)},
		"icon.svg":                        {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h1v1z"/></svg>`)},
		"overview.md":                     {Data: []byte("### Demo\n")},
		"config/collect-metrics.md":       {Data: []byte(validSnippet)},
		"assets/dashboards/overview.json": {Data: []byte(`{"schemaVersion":"v6","image":"","name":"","generateName":true,"tags":[],"spec":{"display":{"name":"Demo"},"panels":{}}}`)},
		"data_collected.json": {Data: []byte(`{"logs":[{"name":"Timestamp","path":"timestamp","type":"timestamp"}],` +
			`"metrics":[{"name":"demo.requests","type":"Sum","unit":"number","description":"Requests"}]}`)},
	}
}

// withAlerts sets the manifest's version and its alerts to the given YAML
// list items; none leaves an empty list.
func withAlerts(manifest string, version int, items string) string {
	manifest = strings.Replace(manifest, "manifest_version: 1", fmt.Sprintf("manifest_version: %d", version), 1)
	if items == "" {
		return manifest
	}
	return strings.Replace(manifest, "  alerts: []\n", "  alerts:\n"+items, 1)
}

const validAlert = `{"alert": "Demo down", "version": "v5"}`

func mustLoad(t *testing.T, fsys fstest.MapFS) *Package {
	t.Helper()
	p, err := Load(fsys, "demo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return p
}

func TestLoadSchema(t *testing.T) {
	testCases := []struct {
		name    string
		edit    func(string) string
		wantErr string
	}{
		{name: "Valid", edit: func(s string) string { return s }},
		{name: "UnknownField", edit: func(s string) string { return s + "extra: 1\n" }, wantErr: "schema"},
		{name: "MissingVersion", edit: func(s string) string { return strings.Replace(s, "version: 1.0.0\n", "", 1) }, wantErr: "schema"},
		{name: "FutureManifestVersion", edit: func(s string) string { return strings.Replace(s, "manifest_version: 1", "manifest_version: 3", 1) }, wantErr: "schema"},
		{name: "V1AlertFile", edit: func(s string) string { return withAlerts(s, 1, "    - assets/alerts/a.json\n") }},
		{name: "V1AlertWithID", edit: func(s string) string { return withAlerts(s, 1, "    - {id: a, definition: assets/alerts/a.json}\n") }, wantErr: "schema"},
		{name: "V2AlertWithID", edit: func(s string) string { return withAlerts(s, 2, "    - {id: a, definition: assets/alerts/a.json}\n") }},
		{name: "V2NoAlerts", edit: func(s string) string { return withAlerts(s, 2, "") }},
		{name: "V2AlertFile", edit: func(s string) string { return withAlerts(s, 2, "    - assets/alerts/a.json\n") }, wantErr: "schema"},
		{name: "V2AlertWithoutID", edit: func(s string) string { return withAlerts(s, 2, "    - {definition: assets/alerts/a.json}\n") }, wantErr: "schema"},
		{name: "V2AlertUnknownField", edit: func(s string) string {
			return withAlerts(s, 2, "    - {id: a, definition: assets/alerts/a.json, extra: 1}\n")
		}, wantErr: "schema"},
		{name: "V2AlertBadID", edit: func(s string) string { return withAlerts(s, 2, "    - {id: A b, definition: assets/alerts/a.json}\n") }, wantErr: "schema"},
		{name: "IdWithoutNamespace", edit: func(s string) string { return strings.Replace(s, "id: builtin-demo", "id: demo", 1) }, wantErr: "schema"},
		{name: "VersionWithV", edit: func(s string) string { return strings.Replace(s, "version: 1.0.0", "version: v1.0.0", 1) }, wantErr: "schema"},
		{name: "PathTraversal", edit: func(s string) string {
			return strings.Replace(s, "config/collect-metrics.md", "config/../../etc.md", 1)
		}, wantErr: "schema"},
		{name: "UnknownSignal", edit: func(s string) string { return strings.Replace(s, "  - metrics\n", "  - profiles\n", 1) }, wantErr: "schema"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := validFS()
			fsys["manifest.yaml"] = &fstest.MapFile{Data: []byte(tc.edit(validManifest))}
			_, err := Load(fsys, "demo")
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name string
		edit func(fstest.MapFS)
		want string
	}{
		{name: "Valid", edit: func(fstest.MapFS) {}},
		{name: "StrayFile", edit: func(f fstest.MapFS) { f["notes.txt"] = &fstest.MapFile{Data: []byte("x")} }, want: "notes.txt: is not referenced"},
		{name: "MissingFile", edit: func(f fstest.MapFS) { delete(f, "overview.md") }, want: "references overview.md, which does not exist"},
		{name: "SignalsMismatch", edit: func(f fstest.MapFS) {
			f["data_collected.json"] = &fstest.MapFile{Data: []byte(`{"logs":[{"name":"T","path":"timestamp","type":"timestamp"}],"metrics":[]}`)}
		}, want: "signals"},
		{name: "UnknownDataCollectedField", edit: func(f fstest.MapFS) {
			f["data_collected.json"] = &fstest.MapFile{Data: []byte(`{"logs":[],"metrics":[],"traces":[]}`)}
		}, want: "unknown field"},
		{name: "DashboardSchemaVersion", edit: func(f fstest.MapFS) {
			f["assets/dashboards/overview.json"] = &fstest.MapFile{Data: []byte(`{"schemaVersion":"v5","generateName":true,"spec":{"display":{"name":"Demo"}}}`)}
		}, want: `schemaVersion must be "v6"`},
		{name: "DashboardNotJSON", edit: func(f fstest.MapFS) {
			f["assets/dashboards/overview.json"] = &fstest.MapFile{Data: []byte(`{`)}
		}, want: "cannot render"},
		{name: "BrandName", edit: func(f fstest.MapFS) {
			f["overview.md"] = &fstest.MapFile{Data: []byte("Send data to SigNoz\n")}
		}, want: `overview: mentions "SigNoz"`},
		{name: "BrandLink", edit: func(f fstest.MapFS) {
			f["overview.md"] = &fstest.MapFile{Data: []byte("See https://signoz.io/docs\n")}
		}, want: `mentions "signoz.io"`},
		{name: "IngestionKey", edit: func(f fstest.MapFS) {
			f["config/collect-metrics.md"] = &fstest.MapFile{Data: []byte(validSnippet + "\nexport SIGNOZ_INGESTION_KEY=x\n")}
		}, want: `mentions "SIGNOZ_INGESTION_KEY"`},
		{name: "SecondExporter", edit: func(f fstest.MapFS) {
			f["config/collect-metrics.md"] = &fstest.MapFile{Data: []byte(strings.Replace(validSnippet, "exporters:\n", "exporters:\n  debug: {}\n", 1))}
		}, want: "exactly one exporter"},
		{name: "ExporterHeaders", edit: func(f fstest.MapFS) {
			f["config/collect-metrics.md"] = &fstest.MapFile{Data: []byte(strings.Replace(validSnippet,
				"    endpoint: ${env:OTLP_DESTINATION_ENDPOINT}\n",
				"    endpoint: ${env:OTLP_DESTINATION_ENDPOINT}\n    headers:\n      key: x\n", 1))}
		}, want: "must not set headers"},
		{name: "ExporterEndpoint", edit: func(f fstest.MapFS) {
			f["config/collect-metrics.md"] = &fstest.MapFile{Data: []byte(strings.Replace(validSnippet, "${env:OTLP_DESTINATION_ENDPOINT}", "ingest.example.com:443", 1))}
		}, want: "endpoint must be"},
		{name: "UndefinedProcessor", edit: func(f fstest.MapFS) {
			f["config/collect-metrics.md"] = &fstest.MapFile{Data: []byte(strings.Replace(validSnippet, "processors: [batch]", "processors: [batch, memory_limiter]", 1))}
		}, want: "undefined processor memory_limiter"},
		{name: "V2Alerts", edit: func(f fstest.MapFS) {
			f["manifest.yaml"] = &fstest.MapFile{Data: []byte(withAlerts(validManifest, 2, "    - {id: a, definition: assets/alerts/a.json}\n    - {id: b, definition: assets/alerts/b.json}\n"))}
			f["assets/alerts/a.json"] = &fstest.MapFile{Data: []byte(validAlert)}
			f["assets/alerts/b.json"] = &fstest.MapFile{Data: []byte(validAlert)}
		}},
		{name: "V2AlertIDTwice", edit: func(f fstest.MapFS) {
			f["manifest.yaml"] = &fstest.MapFile{Data: []byte(withAlerts(validManifest, 2, "    - {id: a, definition: assets/alerts/a.json}\n    - {id: a, definition: assets/alerts/b.json}\n"))}
			f["assets/alerts/a.json"] = &fstest.MapFile{Data: []byte(validAlert)}
			f["assets/alerts/b.json"] = &fstest.MapFile{Data: []byte(validAlert)}
		}, want: "alert id a is listed twice"},
		{name: "V2AlertFileMissing", edit: func(f fstest.MapFS) {
			f["manifest.yaml"] = &fstest.MapFile{Data: []byte(withAlerts(validManifest, 2, "    - {id: a, definition: assets/alerts/a.json}\n"))}
		}, want: "references assets/alerts/a.json, which does not exist"},
		{name: "V2AlertNotAnObject", edit: func(f fstest.MapFS) {
			f["manifest.yaml"] = &fstest.MapFile{Data: []byte(withAlerts(validManifest, 2, "    - {id: a, definition: assets/alerts/a.json}\n"))}
			f["assets/alerts/a.json"] = &fstest.MapFile{Data: []byte(`[]`)}
		}, want: "cannot render"},
		{name: "SVGScript", edit: func(f fstest.MapFS) {
			f["icon.svg"] = &fstest.MapFile{Data: []byte(`<svg><script>alert(1)</script></svg>`)}
		}, want: "must not contain scripts"},
		{name: "SVGExternalImage", edit: func(f fstest.MapFS) {
			f["icon.svg"] = &fstest.MapFile{Data: []byte(`<svg><image href="https://example.com/x.png"/></svg>`)}
		}, want: "external resources"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := validFS()
			tc.edit(fsys)
			problems := Validate(mustLoad(t, fsys))
			var all []string
			for _, p := range problems {
				all = append(all, p.String())
			}
			joined := strings.Join(all, "\n")
			if tc.want == "" && len(problems) > 0 {
				t.Fatalf("unexpected problems:\n%s", joined)
			}
			if tc.want != "" && !strings.Contains(joined, tc.want) {
				t.Fatalf("want a problem containing %q, got:\n%s", tc.want, joined)
			}
		})
	}
}

func TestValidateAllRejectsDuplicateIDs(t *testing.T) {
	a, b := mustLoad(t, validFS()), mustLoad(t, validFS())
	b.Dir = "demo2"
	problems := ValidateAll([]*Package{a, b})
	if len(problems) != 1 || !strings.Contains(problems[0].Message, "also used by demo") {
		t.Fatalf("got %v", problems)
	}
}

func TestRender(t *testing.T) {
	d, err := Render(mustLoad(t, validFS()))
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "builtin-demo" || d.Overview != "### Demo\n" {
		t.Fatalf("got id %q overview %q", d.ID, d.Overview)
	}
	if !strings.HasPrefix(d.Icon, "data:image/svg+xml;base64,") {
		t.Fatalf("icon %q", d.Icon)
	}
	if got := d.ConnectionTests.Metrics; len(got) != 1 || got[0] != "demo_requests" {
		t.Fatalf("connection test metrics %v", got)
	}
}

// Alerts render as the API serves them, the bare rule, whichever form the
// manifest lists them in.
func TestRenderAlerts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
		items   string
	}{
		{name: "V1", version: 1, items: "    - assets/alerts/a.json\n"},
		{name: "V2", version: 2, items: "    - {id: a, definition: assets/alerts/a.json}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := validFS()
			fsys["manifest.yaml"] = &fstest.MapFile{Data: []byte(withAlerts(validManifest, tc.version, tc.items))}
			fsys["assets/alerts/a.json"] = &fstest.MapFile{Data: []byte(validAlert)}
			d, err := Render(mustLoad(t, fsys))
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(d.Assets.Alerts)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != `[{"alert":"Demo down","version":"v5"}]` {
				t.Fatalf("alerts render as %s", got)
			}
		})
	}
}

// The API serves an omitted asset list as null and an empty one as [];
// the UI and existing installs see that difference, so rendering keeps it.
func TestRenderKeepsNullAndEmptyApart(t *testing.T) {
	testCases := []struct {
		name     string
		edit     func(string) string
		wantJSON string
	}{
		{name: "Present", edit: func(s string) string { return s }, wantJSON: `"alerts":[]`},
		{name: "Omitted", edit: func(s string) string { return strings.Replace(s, "  alerts: []\n", "", 1) }, wantJSON: `"alerts":null`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := validFS()
			fsys["manifest.yaml"] = &fstest.MapFile{Data: []byte(tc.edit(validManifest))}
			d, err := Render(mustLoad(t, fsys))
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(d.Assets)
			if !strings.Contains(string(raw), tc.wantJSON) {
				t.Fatalf("want %s in %s", tc.wantJSON, raw)
			}
			if !strings.Contains(string(raw), `"pipelines":[]`) {
				t.Fatalf("pipelines: [] must render as [], got %s", raw)
			}
		})
	}
}

func TestPromMetricName(t *testing.T) {
	testCases := map[string]string{
		"redis_cpu_time":           "redis_cpu_time",
		"mongodb.cache.operations": "mongodb_cache_operations",
		"1xx":                      "key_1xx",
		"_private":                 "key_private",
		"":                         "",
	}
	for in, want := range testCases {
		if got := PromMetricName(in); got != want {
			t.Errorf("PromMetricName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectorSnippets(t *testing.T) {
	p := mustLoad(t, validFS())
	d, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	snippets, err := CollectorSnippets(p, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(snippets) != 1 {
		t.Fatalf("got %d snippets", len(snippets))
	}
	s := snippets[0]
	if s.Name != "demo--collect-metrics--0" || s.Distribution != "contrib" {
		t.Fatalf("got %+v", s)
	}
	want := "DEMO_ENDPOINT=localhost:1\nOTLP_DESTINATION_ENDPOINT=placeholder:4317\n"
	if got := s.EnvFile(); got != want {
		t.Fatalf("env file:\n%s\nwant:\n%s", got, want)
	}
}

func TestCollectorSnippetsNeedingFuseScopeCollector(t *testing.T) {
	fsys := validFS()
	fsys["config/collect-metrics.md"] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(validSnippet, "demo", "clickhousesystemtablesreceiver/query_log"))}
	p := mustLoad(t, fsys)
	d, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	snippets, err := CollectorSnippets(p, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(snippets) != 1 || snippets[0].Distribution != "fusescope" {
		t.Fatalf("got %+v", snippets)
	}
}
