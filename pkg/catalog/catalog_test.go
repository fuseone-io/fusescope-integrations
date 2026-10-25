package catalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/fuseone-io/fusescope-integrations/pkg/integration"
)

func demoPackage(t *testing.T) *integration.Package {
	t.Helper()
	fsys := fstest.MapFS{
		"manifest.yaml": {Data: []byte(`manifest_version: 1
id: builtin-demo
version: 1.2.3
fusescope: ">=0.154.0"
title: Demo
description: Demo
author: {name: FuseScope, email: "", homepage: ""}
icon: icon.svg
categories: [Database]
signals: [logs]
overview: overview.md
configuration:
  - title: Prerequisites
    instructions: config/prerequisites.md
assets:
  logs:
    pipelines: []
data_collected: data_collected.json
connection_tests:
  logs: {attribute_key: source, attribute_value: demo}
`)},
		"icon.svg":                 {Data: []byte("<svg/>")},
		"overview.md":              {Data: []byte("overview\n")},
		"config/prerequisites.md":  {Data: []byte("prereqs\n")},
		"data_collected.json":      {Data: []byte(`{"logs":[{"name":"T","path":"timestamp","type":"timestamp"}],"metrics":[]}`)},
		"unreferenced-but-ignored": {Data: []byte("x")},
	}
	p, err := integration.Load(fsys, "demo")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestArchiveIsReproducibleAndHoldsOnlyReferencedFiles(t *testing.T) {
	p := demoPackage(t)
	a, err := Archive(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Archive(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two builds of the same package differ")
	}

	gz, err := gzip.NewReader(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.ModTime.Unix() != 0 || hdr.Uid != 0 || hdr.Mode != 0o644 {
			t.Errorf("%s: header not normalised: %+v", hdr.Name, hdr)
		}
		names = append(names, hdr.Name)
	}
	want := []string{"manifest.yaml", "icon.svg", "overview.md", "data_collected.json", "config/prerequisites.md"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func TestBuildWritesIndexWithChecksums(t *testing.T) {
	out := t.TempDir()
	items, err := Plan([]*integration.Package{demoPackage(t)}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	index, err := Build(items, Source{Repository: "r", Ref: "v1", Commit: "c"}, out)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(out, IndexFile))
	if err != nil {
		t.Fatal(err)
	}
	var onDisk Index
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.CatalogVersion != Version || len(onDisk.Integrations) != 1 {
		t.Fatalf("index %+v", onDisk)
	}
	e := onDisk.Integrations[0]
	if e.ID != "builtin-demo" || e.Version != "1.2.3" || e.ManifestVersion != 1 || e.Package != "builtin-demo-1.2.3.tar.gz" || e.FuseScope != ">=0.154.0" {
		t.Fatalf("entry %+v", e)
	}
	archive, err := os.ReadFile(filepath.Join(out, e.Package))
	if err != nil {
		t.Fatal(err)
	}
	if e.Size != len(archive) || e.SHA256 != index.Integrations[0].SHA256 || len(e.SHA256) != 64 {
		t.Fatalf("checksum/size mismatch: %+v", e)
	}
}
