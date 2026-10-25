// Package catalog builds the release catalog: one tarball per package and an
// index.json listing them with their checksums and compatibility.
package catalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fuseone-io/fusescope-integrations/pkg/integration"
)

// Version is the format of index.json.
const Version = 1

// IndexFile is the catalog index's file name in a release.
const IndexFile = "index.json"

type Index struct {
	CatalogVersion int     `json:"catalog_version"`
	Source         Source  `json:"source"`
	Integrations   []Entry `json:"integrations"`
}

type Source struct {
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Commit     string `json:"commit"`
}

type Entry struct {
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	ManifestVersion int      `json:"manifest_version"`
	FuseScope       string   `json:"fusescope"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Categories      []string `json:"categories"`
	Signals         []string `json:"signals"`
	Package         string   `json:"package"`
	SHA256          string   `json:"sha256"`
	Size            int      `json:"size"`
}

// PackageFileName is the tarball name of a package version.
func PackageFileName(m integration.Manifest) string {
	return fmt.Sprintf("%s-%s.tar.gz", m.ID, m.Version)
}

// Build writes a tarball per package and index.json into outDir.
func Build(pkgs []*integration.Package, source Source, outDir string) (*Index, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	index := &Index{CatalogVersion: Version, Source: source, Integrations: []Entry{}}
	for _, p := range pkgs {
		archive, err := Archive(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Dir, err)
		}
		name := PackageFileName(p.Manifest)
		if err := os.WriteFile(filepath.Join(outDir, name), archive, 0o644); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(archive)
		m := p.Manifest
		index.Integrations = append(index.Integrations, Entry{
			ID:              m.ID,
			Version:         m.Version,
			ManifestVersion: m.ManifestVersion,
			FuseScope:       m.FuseScope,
			Title:           m.Title,
			Description:     m.Description,
			Categories:      m.Categories,
			Signals:         m.Signals,
			Package:         name,
			SHA256:          hex.EncodeToString(sum[:]),
			Size:            len(archive),
		})
	}

	var raw bytes.Buffer
	enc := json.NewEncoder(&raw)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(index); err != nil {
		return nil, err
	}
	return index, os.WriteFile(filepath.Join(outDir, IndexFile), raw.Bytes(), 0o644)
}

// Archive packs the files the manifest references into a reproducible
// tar.gz: fixed order, modes, owners and timestamps.
func Archive(p *integration.Package) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(gz)
	epoch := time.Unix(0, 0).UTC()

	for _, name := range p.Files() {
		content, err := fs.ReadFile(p.FS, name)
		if err != nil {
			return nil, err
		}
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			ModTime:  epoch,
			Typeflag: tar.TypeReg,
			Format:   tar.FormatPAX,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(content); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
