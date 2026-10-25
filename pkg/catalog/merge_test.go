package catalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fuseone-io/fusescope-integrations/pkg/integration"
)

func testPackage(t *testing.T, id, version, overview string) *integration.Package {
	t.Helper()
	fsys := fstest.MapFS{
		"manifest.yaml": {Data: []byte(`manifest_version: 1
id: ` + id + `
version: ` + version + `
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
		"icon.svg":                {Data: []byte("<svg/>")},
		"overview.md":             {Data: []byte(overview)},
		"config/prerequisites.md": {Data: []byte("prereqs\n")},
		"data_collected.json":     {Data: []byte(`{"logs":[{"name":"T","path":"timestamp","type":"timestamp"}],"metrics":[]}`)},
	}
	p, err := integration.Load(fsys, id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// releaseOf builds a previous release from packages, packing them without
// compression so their bytes differ from what Archive writes for the same files.
func releaseOf(t *testing.T, pkgs ...*integration.Package) *Release {
	t.Helper()
	r := &Release{Index: Index{CatalogVersion: Version}, Archives: map[string][]byte{}}
	for _, p := range pkgs {
		archive := uncompressedArchive(t, p)
		sum := sha256.Sum256(archive)
		e := newEntry(p.Manifest, archive)
		if e.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatal("newEntry checksum")
		}
		r.Index.Integrations = append(r.Index.Integrations, e)
		r.Archives[e.Package] = archive
	}
	return r
}

func uncompressedArchive(t *testing.T, p *integration.Package) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.NoCompression)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(gz)
	for _, name := range p.Files() {
		content, err := fs.ReadFile(p.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPlan(t *testing.T) {
	mysql100 := testPackage(t, "builtin-mysql", "1.0.0", "v1\n")
	mysql101 := testPackage(t, "builtin-mysql", "1.0.1", "v1\n")
	mysql110 := testPackage(t, "builtin-mysql", "1.1.0", "v2\n")
	mysql190 := testPackage(t, "builtin-mysql", "1.9.0", "v9\n")
	mysql1100 := testPackage(t, "builtin-mysql", "1.10.0", "v10\n")
	redis := testPackage(t, "builtin-redis", "1.0.0", "redis\n")
	gone := testPackage(t, "builtin-gone", "1.0.0", "gone\n")

	type want struct {
		key     string
		carried bool
	}
	tests := []struct {
		name     string
		tree     []*integration.Package
		previous *Release
		retired  Retired
		want     []want
		err      string
	}{
		{
			name: "no previous release",
			tree: []*integration.Package{redis, mysql101},
			want: []want{{"builtin-mysql@1.0.1", false}, {"builtin-redis@1.0.0", false}},
		},
		{
			name:     "unchanged version is carried over",
			tree:     []*integration.Package{mysql101, redis},
			previous: releaseOf(t, mysql101, redis),
			want:     []want{{"builtin-mysql@1.0.1", true}, {"builtin-redis@1.0.0", true}},
		},
		{
			name:     "bumped package keeps the released version",
			tree:     []*integration.Package{mysql110, redis},
			previous: releaseOf(t, mysql101, redis),
			want:     []want{{"builtin-mysql@1.0.1", true}, {"builtin-mysql@1.1.0", false}, {"builtin-redis@1.0.0", true}},
		},
		{
			name:     "package removed from the tree stays in the catalog",
			tree:     []*integration.Package{redis},
			previous: releaseOf(t, gone, redis),
			want:     []want{{"builtin-gone@1.0.0", true}, {"builtin-redis@1.0.0", true}},
		},
		{
			name:     "versions sort by semver",
			tree:     []*integration.Package{mysql1100},
			previous: releaseOf(t, mysql190, mysql100),
			want:     []want{{"builtin-mysql@1.0.0", true}, {"builtin-mysql@1.9.0", true}, {"builtin-mysql@1.10.0", false}},
		},
		{
			name:     "retired version is dropped",
			tree:     []*integration.Package{mysql110},
			previous: releaseOf(t, mysql100, mysql101),
			retired:  Retired{"builtin-mysql@1.0.0": true},
			want:     []want{{"builtin-mysql@1.0.1", true}, {"builtin-mysql@1.1.0", false}},
		},
		{
			name:    "retired version still in the tree",
			tree:    []*integration.Package{mysql101},
			retired: Retired{"builtin-mysql@1.0.1": true},
			err:     "builtin-mysql@1.0.1 is retired",
		},
		{
			name:     "released version changed in the tree",
			tree:     []*integration.Package{testPackage(t, "builtin-mysql", "1.0.1", "edited\n")},
			previous: releaseOf(t, mysql101),
			err:      "builtin-mysql@1.0.1 differs from the released package: overview.md",
		},
		{
			name: "released version has other files",
			tree: []*integration.Package{mysql101},
			previous: func() *Release {
				r := releaseOf(t, mysql101)
				var buf bytes.Buffer
				gz := gzip.NewWriter(&buf)
				tw := tar.NewWriter(gz)
				_ = tw.WriteHeader(&tar.Header{Name: "manifest.yaml", Mode: 0o644, Typeflag: tar.TypeReg})
				_ = tw.Close()
				_ = gz.Close()
				r.Archives["builtin-mysql-1.0.1.tar.gz"] = buf.Bytes()
				return r
			}(),
			err: "builtin-mysql@1.0.1 differs from the released package",
		},
		{
			name:     "released version listed twice",
			tree:     []*integration.Package{redis},
			previous: releaseOf(t, mysql101, mysql101),
			err:      "builtin-mysql@1.0.1 is listed twice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := Plan(tt.tree, tt.previous, tt.retired)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != len(tt.want) {
				t.Fatalf("got %d items, want %v", len(items), tt.want)
			}
			for i, w := range tt.want {
				it := items[i]
				if key := it.Entry.ID + "@" + it.Entry.Version; key != w.key || it.Carried != w.carried {
					t.Errorf("item %d: %s carried=%v, want %s carried=%v", i, key, it.Carried, w.key, w.carried)
				}
				sum := sha256.Sum256(it.Archive)
				if it.Entry.SHA256 != hex.EncodeToString(sum[:]) || it.Entry.Size != len(it.Archive) {
					t.Errorf("%s: entry does not describe its archive", w.key)
				}
				if it.Carried {
					prev := tt.previous.Archives[it.Entry.Package]
					if !bytes.Equal(prev, it.Archive) {
						t.Errorf("%s: carried archive is not byte for byte the released one", w.key)
					}
				}
			}
		})
	}
}

func TestParseRetired(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
		err  string
	}{
		{name: "empty list", raw: "retired: []\n"},
		{name: "empty file", raw: ""},
		{name: "entries", raw: "retired:\n  - builtin-mysql@1.0.0\n  - builtin-redis@1.0.0\n", want: []string{"builtin-mysql@1.0.0", "builtin-redis@1.0.0"}},
		{name: "missing version", raw: "retired: [builtin-mysql]\n", err: "<id>@<version>"},
		{name: "not semver", raw: "retired: [builtin-mysql@1.0]\n", err: "not semver"},
		{name: "duplicate", raw: "retired: [builtin-mysql@1.0.0, builtin-mysql@1.0.0]\n", err: "twice"},
		{name: "unknown field", raw: "retire: []\n", err: "retire"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRetired([]byte(tt.raw))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for _, k := range tt.want {
				if !got[k] {
					t.Errorf("%s missing from %v", k, got)
				}
			}
		})
	}
}
