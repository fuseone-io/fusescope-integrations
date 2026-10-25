package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode"
)

// Details is an integration as GET /api/v1/integrations/{id} serves it, before
// the installation is attached. JSON field names and null/[] distinctions match
// the backend's IntegrationDetails.
type Details struct {
	ID              string             `json:"id"`
	Title           string             `json:"title"`
	Description     string             `json:"description"`
	Author          Author             `json:"author"`
	Icon            string             `json:"icon"`
	Categories      []string           `json:"categories"`
	Overview        string             `json:"overview"`
	Configuration   []ConfigStep       `json:"configuration"`
	DataCollected   DataCollected      `json:"data_collected"`
	Assets          Assets             `json:"assets"`
	ConnectionTests *RenderedConnTests `json:"connection_tests"`
}

type ConfigStep struct {
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

type DataCollected struct {
	Logs    []CollectedLogAttribute `json:"logs"`
	Metrics []CollectedMetric       `json:"metrics"`
}

type CollectedLogAttribute struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type CollectedMetric struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
}

type Assets struct {
	Logs       LogAssets         `json:"logs"`
	Dashboards []Dashboard       `json:"dashboards"`
	Alerts     []json.RawMessage `json:"alerts"`
}

type LogAssets struct {
	Pipelines []json.RawMessage `json:"pipelines"`
}

type Dashboard struct {
	ID         string          `json:"id"`
	Definition json.RawMessage `json:"definition"`
}

type RenderedConnTests struct {
	Logs    *LogsConnectionTest `json:"logs"`
	Metrics []string            `json:"metrics"`
}

// Render inlines the files a package references: markdown as text, JSON as
// objects and the icon as a data URI.
func Render(p *Package) (*Details, error) {
	m := p.Manifest
	d := &Details{
		ID:          m.ID,
		Title:       m.Title,
		Description: m.Description,
		Author:      m.Author,
		Categories:  m.Categories,
	}

	var err error
	if d.Icon, err = readDataURI(p.FS, m.Icon); err != nil {
		return nil, err
	}
	if d.Overview, err = readText(p.FS, m.Overview); err != nil {
		return nil, err
	}
	d.Configuration = make([]ConfigStep, 0, len(m.Configuration))
	for _, step := range m.Configuration {
		text, err := readText(p.FS, step.Instructions)
		if err != nil {
			return nil, err
		}
		d.Configuration = append(d.Configuration, ConfigStep{Title: step.Title, Instructions: text})
	}
	if err := readStrictJSON(p.FS, m.DataCollected, &d.DataCollected); err != nil {
		return nil, err
	}

	if d.Assets.Logs.Pipelines, err = readObjects(p.FS, m.Assets.Logs.Pipelines); err != nil {
		return nil, err
	}
	if d.Assets.Alerts, err = readObjects(p.FS, m.Assets.Alerts); err != nil {
		return nil, err
	}
	if m.Assets.Dashboards != nil {
		d.Assets.Dashboards = make([]Dashboard, 0, len(m.Assets.Dashboards))
		for _, ref := range m.Assets.Dashboards {
			raw, err := readObject(p.FS, ref.Definition)
			if err != nil {
				return nil, err
			}
			d.Assets.Dashboards = append(d.Assets.Dashboards, Dashboard{ID: ref.ID, Definition: raw})
		}
	}

	d.ConnectionTests = &RenderedConnTests{Logs: m.ConnectionTests.Logs}
	if len(d.DataCollected.Metrics) > 0 {
		for _, metric := range d.DataCollected.Metrics {
			d.ConnectionTests.Metrics = append(d.ConnectionTests.Metrics, PromMetricName(metric.Name))
		}
	}
	return d, nil
}

func readText(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readDataURI(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	var mime string
	switch path.Ext(name) {
	case ".svg":
		mime = "image/svg+xml"
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	default:
		return "", fmt.Errorf("%s: unsupported icon type", name)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

func readStrictJSON(fsys fs.FS, name string, v any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func readObject(fsys fs.FS, name string) (json.RawMessage, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, fmt.Errorf("%s: must hold one JSON object: %w", name, err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, b); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return compact.Bytes(), nil
}

func readObjects(fsys fs.FS, names []string) ([]json.RawMessage, error) {
	if names == nil {
		return nil, nil
	}
	out := make([]json.RawMessage, 0, len(names))
	for _, name := range names {
		raw, err := readObject(fsys, name)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

// PromMetricName mirrors the backend's toPromMetricName: non-alphanumerics
// become underscores and a leading digit or underscore gets a "key" prefix.
func PromMetricName(s string) string {
	if s == "" {
		return s
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	if s[0] == '_' {
		s = "key" + s
	}
	return s
}
