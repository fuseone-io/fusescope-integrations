package integration

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Exporter is the only exporter a collector snippet may use, sending to FuseScope.
const Exporter = "otlp/fusescope"

// ExporterEndpoint is the endpoint every snippet's exporter must read from the environment.
const ExporterEndpoint = "${env:OTLP_DESTINATION_ENDPOINT}"

var (
	forbiddenBrand = []string{"SigNoz", "signoz.io", "signoz.cloud"}
	// Ingestion keys and test fixtures that must not leak into user instructions.
	forbiddenInSnippets = []string{"signoz-access-token", "SIGNOZ_INGESTION_KEY", "pgtestdb"}

	yamlBlockPattern = regexp.MustCompile("(?s)```yaml\n(.*?)```")
)

// CheckBrand reports user-facing text that names SigNoz or links to its sites.
func CheckBrand(d *Details) []Problem {
	texts := []struct{ field, text string }{
		{"title", d.Title},
		{"description", d.Description},
		{"author", d.Author.Name + " " + d.Author.Email + " " + d.Author.HomePage},
		{"overview", d.Overview},
	}
	for _, step := range d.Configuration {
		texts = append(texts, struct{ field, text string }{"configuration/" + step.Title, step.Instructions})
	}

	var problems []Problem
	for _, t := range texts {
		for _, forbidden := range forbiddenBrand {
			if strings.Contains(t.text, forbidden) {
				problems = append(problems, Problem{File: t.field, Message: fmt.Sprintf("mentions %q", forbidden)})
			}
		}
	}
	return problems
}

type cloudWatchExporterSnippet struct {
	Metrics []struct {
		Namespace  string `yaml:"aws_namespace"`
		MetricName string `yaml:"aws_metric_name"`
	} `yaml:"metrics"`
}

// CollectorConfig is the part of a collector config snippet the checks read.
type CollectorConfig struct {
	Receivers  map[string]any `yaml:"receivers"`
	Processors map[string]any `yaml:"processors"`
	Exporters  map[string]struct {
		Endpoint string         `yaml:"endpoint"`
		Headers  map[string]any `yaml:"headers"`
	} `yaml:"exporters"`
	Service struct {
		Pipelines map[string]struct {
			Receivers  []string `yaml:"receivers"`
			Processors []string `yaml:"processors"`
			Exporters  []string `yaml:"exporters"`
		} `yaml:"pipelines"`
	} `yaml:"service"`
}

// Snippet is a fenced yaml block of a configuration step.
type Snippet struct {
	Step  string
	Index int
	YAML  string
}

// YAMLSnippets returns the yaml blocks of every configuration step, in order.
func YAMLSnippets(d *Details) []Snippet {
	var out []Snippet
	for _, step := range d.Configuration {
		for i, match := range yamlBlockPattern.FindAllStringSubmatch(step.Instructions, -1) {
			out = append(out, Snippet{Step: step.Title, Index: i, YAML: match[1]})
		}
	}
	return out
}

// IsCollectorConfig reports whether a snippet is a full collector config, i.e. has pipelines.
func IsCollectorConfig(s Snippet) (CollectorConfig, bool, error) {
	var config CollectorConfig
	if err := yaml.Unmarshal([]byte(s.YAML), &config); err != nil {
		return config, false, err
	}
	return config, len(config.Service.Pipelines) > 0, nil
}

// CheckSnippets ports the backend's collector config test: snippets must send
// only through otlp/fusescope, carry no ingestion key and be self-consistent.
func CheckSnippets(d *Details) []Problem {
	var problems []Problem
	add := func(file, format string, args ...any) {
		problems = append(problems, Problem{File: file, Message: fmt.Sprintf(format, args...)})
	}

	for _, step := range d.Configuration {
		for _, forbidden := range forbiddenInSnippets {
			if strings.Contains(step.Instructions, forbidden) {
				add("configuration/"+step.Title, "mentions %q", forbidden)
			}
		}
	}

	for _, s := range YAMLSnippets(d) {
		file := fmt.Sprintf("configuration/%s#yaml-%d", s.Step, s.Index)

		var cloudWatch cloudWatchExporterSnippet
		if yaml.Unmarshal([]byte(s.YAML), &cloudWatch) == nil {
			seen := map[string]bool{}
			for _, metric := range cloudWatch.Metrics {
				key := metric.Namespace + "/" + metric.MetricName
				if seen[key] {
					add(file, "CloudWatch metric %s listed twice", key)
				}
				seen[key] = true
			}
		}

		config, isConfig, err := IsCollectorConfig(s)
		if err != nil {
			add(file, "invalid YAML: %v", err)
			continue
		}
		if !isConfig {
			continue
		}
		if len(config.Exporters) != 1 {
			add(file, "must define exactly one exporter, %s; found %d", Exporter, len(config.Exporters))
		}
		exporter, ok := config.Exporters[Exporter]
		if !ok {
			add(file, "the only exporter must be %s", Exporter)
		} else {
			if exporter.Endpoint != ExporterEndpoint {
				add(file, "%s endpoint must be %s, got %q", Exporter, ExporterEndpoint, exporter.Endpoint)
			}
			if len(exporter.Headers) > 0 {
				add(file, "%s must not set headers", Exporter)
			}
		}
		for name, pipeline := range config.Service.Pipelines {
			if len(pipeline.Exporters) != 1 || pipeline.Exporters[0] != Exporter {
				add(file, "pipeline %s must export only to %s", name, Exporter)
			}
			for _, r := range pipeline.Receivers {
				if _, ok := config.Receivers[r]; !ok {
					add(file, "pipeline %s uses undefined receiver %s", name, r)
				}
			}
			for _, p := range pipeline.Processors {
				if _, ok := config.Processors[p]; !ok {
					add(file, "pipeline %s uses undefined processor %s", name, p)
				}
			}
		}
	}
	return problems
}

var svgForbidden = []struct {
	pattern *regexp.Regexp
	message string
}{
	{regexp.MustCompile(`(?i)<script`), "must not contain scripts"},
	{regexp.MustCompile(`(?i)<foreignObject`), "must not contain foreignObject"},
	{regexp.MustCompile(`(?i)\son[a-z]+\s*=`), "must not contain event handlers"},
	{regexp.MustCompile(`(?i)javascript:`), "must not contain javascript: URLs"},
	{regexp.MustCompile(`(?i)(href|src)\s*=\s*["']\s*(https?:)?//`), "must not reference external resources"},
}

// CheckSVG keeps icons static: no scripts, handlers or external references.
func CheckSVG(raw []byte) []string {
	var problems []string
	if !bytes.Contains(raw, []byte("<svg")) {
		problems = append(problems, "is not an SVG document")
	}
	for _, f := range svgForbidden {
		if f.pattern.Match(raw) {
			problems = append(problems, f.message)
		}
	}
	return problems
}
