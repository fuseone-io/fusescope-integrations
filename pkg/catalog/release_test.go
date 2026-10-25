package catalog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func releaseFS(t *testing.T, r *Release) fstest.MapFS {
	t.Helper()
	raw, err := json.Marshal(r.Index)
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{IndexFile: {Data: raw}}
	for name, archive := range r.Archives {
		fsys[name] = &fstest.MapFile{Data: archive}
	}
	return fsys
}

func TestReadRelease(t *testing.T) {
	good := releaseOf(t, testPackage(t, "builtin-mysql", "1.0.1", "v1\n"))
	const pkg = "builtin-mysql-1.0.1.tar.gz"

	tests := []struct {
		name   string
		mutate func(fstest.MapFS)
		err    string
	}{
		{name: "valid release"},
		{name: "missing index", mutate: func(f fstest.MapFS) { delete(f, IndexFile) }, err: IndexFile},
		{name: "missing package", mutate: func(f fstest.MapFS) { delete(f, pkg) }, err: pkg},
		{name: "tampered package", mutate: func(f fstest.MapFS) {
			f[pkg] = &fstest.MapFile{Data: append(bytes.Clone(f[pkg].Data), 0)}
		}, err: "the signed index says"},
		{name: "unsupported catalog version", mutate: func(f fstest.MapFS) {
			f[IndexFile] = &fstest.MapFile{Data: bytes.Replace(f[IndexFile].Data, []byte(`"catalog_version":1`), []byte(`"catalog_version":2`), 1)}
		}, err: "catalog_version 2"},
		{name: "unknown index field", mutate: func(f fstest.MapFS) {
			f[IndexFile] = &fstest.MapFile{Data: bytes.Replace(f[IndexFile].Data, []byte(`"catalog_version"`), []byte(`"extra":1,"catalog_version"`), 1)}
		}, err: "extra"},
		{name: "package name not derived from id and version", mutate: func(f fstest.MapFS) {
			f[IndexFile] = &fstest.MapFile{Data: bytes.ReplaceAll(f[IndexFile].Data, []byte(pkg), []byte("../x.tar.gz"))}
		}, err: "../x.tar.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := releaseFS(t, good)
			if tt.mutate != nil {
				tt.mutate(fsys)
			}
			r, err := ReadRelease(fsys)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Index.Integrations) != 1 || !bytes.Equal(r.Archives[pkg], good.Archives[pkg]) {
				t.Fatalf("release %+v", r.Index)
			}
		})
	}
}
