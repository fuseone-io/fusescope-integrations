package integration

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// fuseScopeOnlyComponents ship in fusescope-otel-collector but not in otelcol-contrib.
var fuseScopeOnlyComponents = []string{"clickhousesystemtablesreceiver"}

var (
	envReference   = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)
	exportLine     = regexp.MustCompile(`(?m)^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	angleBracketed = regexp.MustCompile(`<[^<>]*>`)
	stepSlug       = regexp.MustCompile(`[^a-z0-9]+`)
)

// CollectorSnippet is a full collector config from a configuration step, with
// the environment it needs to pass `otelcol validate`.
type CollectorSnippet struct {
	Name string
	YAML string
	Env  map[string]string
	// Distribution is "contrib", or "fusescope" when the snippet needs
	// components only fusescope-otel-collector ships.
	Distribution string
}

// CollectorSnippets returns the package's full collector configs. Variables
// take the values the instructions export, with <placeholders> filled in.
func CollectorSnippets(p *Package, d *Details) ([]CollectorSnippet, error) {
	exported := map[string]string{}
	for _, step := range d.Configuration {
		for _, m := range exportLine.FindAllStringSubmatch(step.Instructions, -1) {
			exported[m[1]] = exportValue(m[1], m[2])
		}
	}

	var out []CollectorSnippet
	for _, s := range YAMLSnippets(d) {
		config, isConfig, err := IsCollectorConfig(s)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", p.Dir, s.Step, err)
		}
		if !isConfig {
			continue
		}

		env := map[string]string{}
		for _, m := range envReference.FindAllStringSubmatch(s.YAML, -1) {
			value, ok := exported[m[1]]
			if !ok || value == "" {
				value = placeholderFor(m[1])
			}
			env[m[1]] = value
		}

		distribution := "contrib"
		for name := range config.Receivers {
			for _, c := range fuseScopeOnlyComponents {
				if strings.SplitN(name, "/", 2)[0] == c {
					distribution = "fusescope"
				}
			}
		}

		slug := strings.Trim(stepSlug.ReplaceAllString(strings.ToLower(s.Step), "-"), "-")
		out = append(out, CollectorSnippet{
			Name:         fmt.Sprintf("%s--%s--%d", p.Dir, slug, s.Index),
			YAML:         s.YAML,
			Env:          env,
			Distribution: distribution,
		})
	}
	return out, nil
}

func exportValue(name, raw string) string {
	v := strings.TrimSpace(raw)
	if i := strings.Index(v, " #"); i >= 0 && !strings.HasPrefix(v, `"`) {
		v = strings.TrimSpace(v[:i])
	}
	v = strings.Trim(v, `"'`)
	if angleBracketed.FindString(v) == v && v != "" {
		return placeholderFor(name)
	}
	return angleBracketed.ReplaceAllString(v, "placeholder")
}

// placeholderFor stands in for a value the user fills in. Receivers parse
// endpoints as host:port, so a bare <host> placeholder would fail validation.
func placeholderFor(name string) string {
	if strings.HasSuffix(name, "_ENDPOINT") {
		return "localhost:1"
	}
	return "placeholder"
}

// EnvFile renders env as KEY=value lines, sorted, for `docker run --env-file`.
func (s CollectorSnippet) EnvFile() string {
	keys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, s.Env[k])
	}
	return b.String()
}
