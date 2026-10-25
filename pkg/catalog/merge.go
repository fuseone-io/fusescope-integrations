package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.yaml.in/yaml/v3"

	"github.com/fuseone-io/fusescope-integrations/pkg/integration"
)

// RetiredFile lists the package versions dropped from the catalog.
const RetiredFile = "retired.yaml"

// Item is one package version of the catalog being built.
type Item struct {
	Entry   Entry
	Archive []byte
	// Carried is set when the archive is the previous release's, byte for byte.
	Carried bool
}

// Retired holds the retired package versions, as <id>@<version>.
type Retired map[string]bool

func key(id, version string) string { return id + "@" + version }

// Plan merges the packages of the tree with the previous release. Every
// released version is kept unless retired, and the tree may not change one.
// Items are sorted by id, then semver.
func Plan(pkgs []*integration.Package, previous *Release, retired Retired) ([]Item, error) {
	released := map[string]Entry{}
	if previous != nil {
		for _, e := range previous.Index.Integrations {
			k := key(e.ID, e.Version)
			if _, dup := released[k]; dup {
				return nil, fmt.Errorf("%s is listed twice in the previous release", k)
			}
			released[k] = e
		}
	}

	var items []Item
	var errs []error
	inTree := map[string]bool{}
	for _, p := range pkgs {
		k := key(p.Manifest.ID, p.Manifest.Version)
		if inTree[k] {
			errs = append(errs, fmt.Errorf("%s is defined twice in the tree", k))
			continue
		}
		inTree[k] = true
		if retired[k] {
			errs = append(errs, fmt.Errorf("%s is retired in %s but still in the tree", k, RetiredFile))
			continue
		}
		if e, ok := released[k]; ok {
			archive := previous.Archives[e.Package]
			if err := sameFiles(p, archive); err != nil {
				errs = append(errs, fmt.Errorf("%s differs from the released package: %w; released versions are immutable, bump its version", k, err))
				continue
			}
			items = append(items, Item{Entry: e, Archive: archive, Carried: true})
			continue
		}
		archive, err := Archive(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Dir, err))
			continue
		}
		items = append(items, Item{Entry: newEntry(p.Manifest, archive), Archive: archive})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	for k, e := range released {
		if !inTree[k] && !retired[k] {
			items = append(items, Item{Entry: e, Archive: previous.Archives[e.Package], Carried: true})
		}
	}
	return items, sortItems(items)
}

func sortItems(items []Item) error {
	versions := make(map[string]*semver.Version, len(items))
	for _, it := range items {
		v, err := semver.StrictNewVersion(it.Entry.Version)
		if err != nil {
			return fmt.Errorf("%s: %w", key(it.Entry.ID, it.Entry.Version), err)
		}
		versions[key(it.Entry.ID, it.Entry.Version)] = v
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i].Entry, items[j].Entry
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return versions[key(a.ID, a.Version)].LessThan(versions[key(b.ID, b.Version)])
	})
	return nil
}

// sameFiles reports the files that differ between the package and a released archive of it.
func sameFiles(p *integration.Package, archive []byte) error {
	released, err := unpack(archive)
	if err != nil {
		return err
	}
	var differ []string
	inTree := map[string]bool{}
	for _, name := range p.Files() {
		inTree[name] = true
		content, err := fs.ReadFile(p.FS, name)
		if err != nil {
			return err
		}
		if old, ok := released[name]; !ok || !bytes.Equal(old, content) {
			differ = append(differ, name)
		}
	}
	for name := range released {
		if !inTree[name] {
			differ = append(differ, name)
		}
	}
	if len(differ) > 0 {
		sort.Strings(differ)
		return errors.New(strings.Join(differ, ", "))
	}
	return nil
}

// ReadRetired reads the retired list at path.
func ReadRetired(path string) (Retired, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r, err := ParseRetired(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

func ParseRetired(raw []byte) (Retired, error) {
	var doc struct {
		Retired []string `yaml:"retired"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	r := Retired{}
	for _, k := range doc.Retired {
		id, version, ok := strings.Cut(k, "@")
		if !ok || id == "" {
			return nil, fmt.Errorf("%q is not <id>@<version>", k)
		}
		if _, err := semver.StrictNewVersion(version); err != nil {
			return nil, fmt.Errorf("%q: version is not semver: %w", k, err)
		}
		if r[k] {
			return nil, fmt.Errorf("%q is listed twice", k)
		}
		r[k] = true
	}
	return r, nil
}
