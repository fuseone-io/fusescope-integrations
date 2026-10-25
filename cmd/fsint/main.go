// Command fsint validates integration packages, renders them as the backend
// serves them, extracts collector snippets, builds the catalog and generates docs.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/fuseone-io/fusescope-integrations/pkg/catalog"
	"github.com/fuseone-io/fusescope-integrations/pkg/docs"
	"github.com/fuseone-io/fusescope-integrations/pkg/integration"
)

const usage = `usage: fsint <command> [flags]

commands:
  validate   check every package (schema, files, dashboards, snippets, brand)
  render     write each package as the API serves it, <out>/<id>.json
  snippets   write full collector configs and their env, <out>/<distribution>/<name>.{yaml,env}
  catalog    build index.json and the tarballs: the tree's package versions
             plus, with -previous, every version the previous release holds
  docs       generate docs/, or with -check fail when it is out of date
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	commands := map[string]func([]string) error{
		"validate": validate,
		"render":   render,
		"snippets": snippets,
		"catalog":  buildCatalog,
		"docs":     generateDocs,
	}
	run, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func load(fs *flag.FlagSet, args []string) ([]*integration.Package, error) {
	root := fs.String("root", "integrations", "directory holding one package per subdirectory")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return integration.LoadAll(os.DirFS(*root), ".")
}

func validate(args []string) error {
	pkgs, err := load(flag.NewFlagSet("validate", flag.ExitOnError), args)
	if err != nil {
		return err
	}
	problems := integration.ValidateAll(pkgs)
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problem(s) in %d package(s)", len(problems), len(pkgs))
	}
	fmt.Printf("%d packages valid\n", len(pkgs))
	return nil
}

func render(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	out := fs.String("out", "rendered", "output directory")
	pkgs, err := load(fs, args)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	for _, p := range pkgs {
		d, err := integration.Render(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Dir, err)
		}
		var raw bytes.Buffer
		enc := json.NewEncoder(&raw)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(d); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(*out, d.ID+".json"), raw.Bytes(), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("rendered %d packages into %s\n", len(pkgs), *out)
	return nil
}

func snippets(args []string) error {
	fs := flag.NewFlagSet("snippets", flag.ExitOnError)
	out := fs.String("out", "snippets", "output directory")
	pkgs, err := load(fs, args)
	if err != nil {
		return err
	}
	count := map[string]int{}
	for _, p := range pkgs {
		d, err := integration.Render(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Dir, err)
		}
		list, err := integration.CollectorSnippets(p, d)
		if err != nil {
			return err
		}
		for _, s := range list {
			dir := filepath.Join(*out, s.Distribution)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, s.Name+".yaml"), []byte(s.YAML), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, s.Name+".env"), []byte(s.EnvFile()), 0o644); err != nil {
				return err
			}
			count[s.Distribution]++
		}
	}
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s: %d snippets\n", k, count[k])
	}
	return nil
}

func buildCatalog(args []string) error {
	fs := flag.NewFlagSet("catalog", flag.ExitOnError)
	out := fs.String("out", "dist", "output directory")
	repo := fs.String("repository", "fuseone-io/fusescope-integrations", "source repository")
	ref := fs.String("ref", "", "git ref the catalog is built from")
	commit := fs.String("commit", "", "git commit the catalog is built from")
	previous := fs.String("previous", "", "directory holding the previous release, its signature already verified; its versions are carried over")
	retiredFile := fs.String("retired", catalog.RetiredFile, "package versions to drop from the catalog")
	pkgs, err := load(fs, args)
	if err != nil {
		return err
	}
	if problems := integration.ValidateAll(pkgs); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		return errors.New("refusing to build a catalog from invalid packages")
	}
	retired, err := catalog.ReadRetired(*retiredFile)
	if err != nil {
		return err
	}
	var release *catalog.Release
	if *previous != "" {
		if release, err = catalog.ReadRelease(os.DirFS(*previous)); err != nil {
			return fmt.Errorf("previous release: %w", err)
		}
	}
	items, err := catalog.Plan(pkgs, release, retired)
	if err != nil {
		return err
	}
	if _, err := catalog.Build(items, catalog.Source{Repository: *repo, Ref: *ref, Commit: *commit}, *out); err != nil {
		return err
	}
	for _, it := range items {
		origin := "new"
		if it.Carried {
			origin = "released"
		}
		fmt.Printf("%s %s %s %s %s\n", it.Entry.ID, it.Entry.Version, it.Entry.SHA256, it.Entry.Package, origin)
	}
	return nil
}

func generateDocs(args []string) error {
	fs := flag.NewFlagSet("docs", flag.ExitOnError)
	out := fs.String("out", "docs", "output directory")
	check := fs.Bool("check", false, "fail when the docs differ from the packages instead of writing them")
	pkgs, err := load(fs, args)
	if err != nil {
		return err
	}
	files, err := docs.Generate(pkgs)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var stale []string
	for _, name := range names {
		target := filepath.Join(*out, name)
		if *check {
			current, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(current, []byte(files[name])) {
				stale = append(stale, target)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(files[name]), 0o644); err != nil {
			return err
		}
	}
	if *check {
		stale = append(stale, orphans(filepath.Join(*out, "integrations"), files)...)
		if len(stale) > 0 {
			for _, s := range stale {
				fmt.Println(s)
			}
			return errors.New("docs are out of date; run `go run ./cmd/fsint docs`")
		}
		fmt.Println("docs up to date")
	}
	return nil
}

func orphans(dir string, files map[string]string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if _, ok := files[filepath.ToSlash(filepath.Join("integrations", e.Name()))]; !ok {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}
