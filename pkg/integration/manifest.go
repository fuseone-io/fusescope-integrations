// Package integration loads, validates and renders FuseScope integration packages.
package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/fuseone-io/fusescope-integrations/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

const (
	// ManifestVersion is the manifest format this module writes and reads.
	ManifestVersion = 1
	ManifestFile    = "manifest.yaml"
	// BuiltinNamespace prefixes the ids of the packages in the official catalog.
	BuiltinNamespace = "builtin"
)

type Manifest struct {
	ManifestVersion int             `yaml:"manifest_version" json:"manifest_version"`
	ID              string          `yaml:"id" json:"id"`
	Version         string          `yaml:"version" json:"version"`
	FuseScope       string          `yaml:"fusescope" json:"fusescope"`
	Title           string          `yaml:"title" json:"title"`
	Description     string          `yaml:"description" json:"description"`
	Author          Author          `yaml:"author" json:"author"`
	Icon            string          `yaml:"icon" json:"icon"`
	Categories      []string        `yaml:"categories" json:"categories"`
	Signals         []string        `yaml:"signals" json:"signals"`
	Overview        string          `yaml:"overview" json:"overview"`
	Configuration   []ConfigStepRef `yaml:"configuration" json:"configuration"`
	Assets          AssetRefs       `yaml:"assets" json:"assets"`
	DataCollected   string          `yaml:"data_collected" json:"data_collected"`
	ConnectionTests ConnectionTests `yaml:"connection_tests" json:"connection_tests"`
}

type Author struct {
	Name     string `yaml:"name" json:"name"`
	Email    string `yaml:"email" json:"email"`
	HomePage string `yaml:"homepage" json:"homepage"`
}

type ConfigStepRef struct {
	Title        string `yaml:"title" json:"title"`
	Instructions string `yaml:"instructions" json:"instructions"`
}

type AssetRefs struct {
	Logs       LogAssetRefs   `yaml:"logs" json:"logs"`
	Dashboards []DashboardRef `yaml:"dashboards" json:"dashboards"`
	Alerts     []string       `yaml:"alerts" json:"alerts"`
}

type LogAssetRefs struct {
	Pipelines []string `yaml:"pipelines" json:"pipelines"`
}

type DashboardRef struct {
	ID         string `yaml:"id" json:"id"`
	Definition string `yaml:"definition" json:"definition"`
}

type ConnectionTests struct {
	Logs *LogsConnectionTest `yaml:"logs" json:"logs"`
}

type LogsConnectionTest struct {
	AttributeKey   string `yaml:"attribute_key" json:"attribute_key"`
	AttributeValue string `yaml:"attribute_value" json:"attribute_value"`
}

// Package is one integration: its manifest and the files of its directory.
type Package struct {
	// Dir is the package's directory name, used in messages and file names.
	Dir      string
	FS       fs.FS
	Manifest Manifest
}

// Files lists the package files the manifest references, manifest.yaml first.
func (p *Package) Files() []string {
	m := p.Manifest
	files := []string{ManifestFile, m.Icon, m.Overview, m.DataCollected}
	for _, step := range m.Configuration {
		files = append(files, step.Instructions)
	}
	files = append(files, m.Assets.Logs.Pipelines...)
	for _, d := range m.Assets.Dashboards {
		files = append(files, d.Definition)
	}
	files = append(files, m.Assets.Alerts...)
	return files
}

var manifestSchema = mustCompileSchema()

func mustCompileSchema() *jsonschema.Schema {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.Manifest))
	if err != nil {
		panic(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("manifest.schema.json", doc); err != nil {
		panic(err)
	}
	return c.MustCompile("manifest.schema.json")
}

// Load reads and schema-validates the manifest of the package rooted at fsys.
func Load(fsys fs.FS, dir string) (*Package, error) {
	raw, err := fs.ReadFile(fsys, ManifestFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}

	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", dir, ManifestFile, err)
	}
	asJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", dir, ManifestFile, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(asJSON))
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", dir, ManifestFile, err)
	}
	if err := manifestSchema.Validate(inst); err != nil {
		return nil, fmt.Errorf("%s/%s does not match the schema: %w", dir, ManifestFile, err)
	}

	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", dir, ManifestFile, err)
	}
	return &Package{Dir: dir, FS: fsys, Manifest: m}, nil
}

// LoadAll loads every package under root, sorted by directory name.
func LoadAll(fsys fs.FS, root string) ([]*Package, error) {
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var pkgs []*Package
	var errs []error
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		sub, err := fs.Sub(fsys, path.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		p, err := Load(sub, e.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, errors.Join(errs...)
}
