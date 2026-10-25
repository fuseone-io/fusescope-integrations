package integration

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Problem is one validation failure, attributed to a package and, when known, a file.
type Problem struct {
	Package string
	File    string
	Message string
}

func (p Problem) String() string {
	if p.File == "" {
		return p.Package + ": " + p.Message
	}
	return p.Package + "/" + p.File + ": " + p.Message
}

// DashboardSchemaVersion is the dashboard format the backend accepts.
const DashboardSchemaVersion = "v6"

// Validate runs every package check. Backend decoding of dashboards, pipelines
// and alerts is covered separately by the compat job, against the backend itself.
func Validate(p *Package) []Problem {
	var problems []Problem
	add := func(file, format string, args ...any) {
		problems = append(problems, Problem{Package: p.Dir, File: file, Message: fmt.Sprintf(format, args...)})
	}

	checkManifest(p, add)
	checkFiles(p, add)

	d, err := Render(p)
	if err != nil {
		add("", "cannot render: %v", err)
		return problems
	}
	checkSignals(p, d, add)
	checkDashboards(p, add)
	for _, problem := range CheckBrand(d) {
		add(problem.File, "%s", problem.Message)
	}
	for _, problem := range CheckSnippets(d) {
		add(problem.File, "%s", problem.Message)
	}
	if path.Ext(p.Manifest.Icon) == ".svg" {
		if raw, err := fs.ReadFile(p.FS, p.Manifest.Icon); err == nil {
			for _, msg := range CheckSVG(raw) {
				add(p.Manifest.Icon, "%s", msg)
			}
		}
	}
	return problems
}

// ValidateAll validates each package and the rules that span packages.
func ValidateAll(pkgs []*Package) []Problem {
	var problems []Problem
	seen := map[string]string{}
	for _, p := range pkgs {
		problems = append(problems, Validate(p)...)
		if other, dup := seen[p.Manifest.ID]; dup {
			problems = append(problems, Problem{Package: p.Dir, Message: fmt.Sprintf("id %s is also used by %s", p.Manifest.ID, other)})
		}
		seen[p.Manifest.ID] = p.Dir
	}
	return problems
}

func checkManifest(p *Package, add func(string, string, ...any)) {
	m := p.Manifest
	if !strings.HasPrefix(m.ID, BuiltinNamespace+"-") {
		add(ManifestFile, "id %q must start with %q in this catalog", m.ID, BuiltinNamespace+"-")
	}
	if _, err := semver.StrictNewVersion(m.Version); err != nil {
		add(ManifestFile, "version %q is not semver: %v", m.Version, err)
	}
	if _, err := semver.NewConstraint(m.FuseScope); err != nil {
		add(ManifestFile, "fusescope %q is not a semver constraint: %v", m.FuseScope, err)
	}

	titles := map[string]bool{}
	for _, step := range m.Configuration {
		if titles[step.Title] {
			add(ManifestFile, "configuration step %q is listed twice", step.Title)
		}
		titles[step.Title] = true
	}
	ids := map[string]bool{}
	for _, d := range m.Assets.Dashboards {
		if ids[d.ID] {
			add(ManifestFile, "multiple dashboards found with id %s", d.ID)
		}
		ids[d.ID] = true
	}
}

// checkFiles requires every referenced file to exist and every file in the
// package to be referenced, so a package is exactly what its manifest says.
func checkFiles(p *Package, add func(string, string, ...any)) {
	referenced := map[string]bool{}
	for _, f := range p.Files() {
		if referenced[f] {
			add(ManifestFile, "%s is referenced twice", f)
		}
		referenced[f] = true
		if _, err := fs.Stat(p.FS, f); err != nil {
			add(ManifestFile, "references %s, which does not exist", f)
		}
	}

	var stray []string
	_ = fs.WalkDir(p.FS, ".", func(name string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		if !referenced[name] {
			stray = append(stray, name)
		}
		return nil
	})
	sort.Strings(stray)
	for _, name := range stray {
		add(name, "is not referenced by %s", ManifestFile)
	}
}

func checkSignals(p *Package, d *Details, add func(string, string, ...any)) {
	want := []string{}
	if len(d.DataCollected.Logs) > 0 {
		want = append(want, "logs")
	}
	if len(d.DataCollected.Metrics) > 0 {
		want = append(want, "metrics")
	}
	got := slices.Clone(p.Manifest.Signals)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		add(ManifestFile, "signals %v do not match %s, which lists %v", p.Manifest.Signals, p.Manifest.DataCollected, want)
	}
}

type dashboardHeader struct {
	SchemaVersion *string `json:"schemaVersion"`
	Name          string  `json:"name"`
	GenerateName  bool    `json:"generateName"`
	Spec          *struct {
		Display *struct {
			Name string `json:"name"`
		} `json:"display"`
	} `json:"spec"`
}

// checkDashboards catches the common mistakes early; the compat job decodes
// each dashboard with the backend's own types for the full rules.
func checkDashboards(p *Package, add func(string, string, ...any)) {
	for _, ref := range p.Manifest.Assets.Dashboards {
		raw, err := fs.ReadFile(p.FS, ref.Definition)
		if err != nil {
			continue
		}
		var h dashboardHeader
		if err := json.Unmarshal(raw, &h); err != nil {
			add(ref.Definition, "invalid JSON: %v", err)
			continue
		}
		if h.SchemaVersion == nil || *h.SchemaVersion != DashboardSchemaVersion {
			add(ref.Definition, "schemaVersion must be %q", DashboardSchemaVersion)
		}
		if h.Spec == nil || h.Spec.Display == nil || h.Spec.Display.Name == "" {
			add(ref.Definition, "spec.display.name is required")
		}
		if !h.GenerateName && h.Name == "" {
			add(ref.Definition, "name is required unless generateName is true")
		}
	}
}
