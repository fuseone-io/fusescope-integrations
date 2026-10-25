package catalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"

	"github.com/Masterminds/semver/v3"
)

const maxArchiveBytes = 16 << 20

var packageNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*\.tar\.gz$`)

// Release is a published catalog: its index and its package archives, by file name.
type Release struct {
	Index    Index
	Archives map[string][]byte
}

// ReadRelease reads a release whose index.json signature has already been
// verified, and checks every package against the checksum and size the index holds.
func ReadRelease(fsys fs.FS) (*Release, error) {
	raw, err := fs.ReadFile(fsys, IndexFile)
	if err != nil {
		return nil, err
	}
	var index Index
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&index); err != nil {
		return nil, fmt.Errorf("%s: %w", IndexFile, err)
	}
	if index.CatalogVersion != Version {
		return nil, fmt.Errorf("%s: unsupported catalog_version %d", IndexFile, index.CatalogVersion)
	}

	r := &Release{Index: index, Archives: map[string][]byte{}}
	for _, e := range index.Integrations {
		if _, err := semver.StrictNewVersion(e.Version); err != nil {
			return nil, fmt.Errorf("%s %s: version is not semver: %w", e.ID, e.Version, err)
		}
		if !packageNamePattern.MatchString(e.Package) || e.Package != packageFileName(e.ID, e.Version) {
			return nil, fmt.Errorf("%s %s: invalid package file name %q", e.ID, e.Version, e.Package)
		}
		archive, err := fs.ReadFile(fsys, e.Package)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(archive)
		if got := hex.EncodeToString(sum[:]); got != e.SHA256 || len(archive) != e.Size {
			return nil, fmt.Errorf("%s: sha256 %s and size %d, the signed index says %s and %d", e.Package, got, len(archive), e.SHA256, e.Size)
		}
		r.Archives[e.Package] = archive
	}
	return r, nil
}

// unpack returns the regular files of a package archive by name.
func unpack(archive []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("%s: not a regular file", hdr.Name)
		}
		if _, dup := files[hdr.Name]; dup {
			return nil, fmt.Errorf("%s: listed twice", hdr.Name)
		}
		total += hdr.Size
		if total > maxArchiveBytes {
			return nil, fmt.Errorf("larger than %d bytes unpacked", maxArchiveBytes)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[hdr.Name] = content
	}
}
