# FuseScope integrations

Integration packages for [FuseScope](https://github.com/fuseone-io/fusescope), the
OpenTelemetry-native observability platform. Each package tells a user how to
send one technology's telemetry to FuseScope and ships what FuseScope installs
for it.

A package is **data only**: markdown, JSON, YAML and an icon. No code runs from
a package. The design is recorded in the backend's
[ADR 0005](https://github.com/fuseone-io/fusescope/blob/main/docs/adr/0005-integration-packages.md).

Catalog: [docs/README.md](docs/README.md).

## What an integration is

What the **Integrations** page shows and installs:

- an overview and the configuration steps: prerequisites and the collector config that collects metrics and logs, sending through the `otlp/fusescope` exporter;
- the data it collects: log attributes and metrics, also used to detect incoming data (connection status);
- assets installed with it: dashboards, log pipelines and alert rules.

## Layout

```
integrations/<dir>/
  manifest.yaml               what the package is, and which files it uses
  overview.md
  config/*.md                 one file per configuration step
  assets/dashboards/*.json    dashboards (v6 format)
  assets/pipelines/*.json     log pipelines, one per file
  assets/alerts/*.json        alert rules, one per file
  data_collected.json         {"logs": [...], "metrics": [...]}
  icon.svg                    or icon.png / icon.jpg
schema/manifest.schema.json   JSON Schema of manifest.yaml
cmd/fsint                     validate, render, snippets, catalog, docs
docs/                         generated reference pages
```

Every file in a package must be referenced by its manifest, and every referenced file must exist.

## Manifest

```yaml
manifest_version: 1
id: builtin-redis
version: 1.0.0
fusescope: ">=0.154.0"
title: "Redis"
description: "Monitor redis with metrics and logs"
author:
  name: "FuseScope"
  email: ""
  homepage: "https://github.com/fuseone-io/fusescope"
icon: icon.svg
categories:
  - "Database"
signals:
  - logs
  - metrics
overview: overview.md
configuration:
  - title: "Prerequisites"
    instructions: config/prerequisites.md
assets:
  logs:
    pipelines: []
  dashboards:
    - id: overview
      definition: assets/dashboards/overview.json
  alerts: []
data_collected: data_collected.json
connection_tests:
  logs:
    attribute_key: "source"
    attribute_value: "redis"
```

| Field | Meaning |
|---|---|
| `manifest_version` | Format of the manifest, `1`. The backend reads the current and the previous format. |
| `id` | `<namespace>-<name>`. The id the API serves and each installation stores; it never changes. This catalog uses the `builtin` namespace. |
| `version` | Package version (semver). Bump it on every change: a released version is immutable. |
| `fusescope` | FuseScope versions the package works with, a semver constraint. |
| `title`, `description`, `author`, `categories` | Shown on the Integrations page. |
| `icon` | `icon.svg`, `icon.png` or `icon.jpg`. Served as a data URI. SVGs must not hold scripts, event handlers or external references. |
| `signals` | `logs`, `metrics`, `traces`. Must match the non-empty lists in `data_collected.json`. |
| `overview` | `overview.md`. |
| `configuration` | Ordered steps: a title and a markdown file under `config/`. |
| `assets.logs.pipelines` | Log pipeline files. |
| `assets.dashboards` | Dashboards installed with the integration. `id` is stable: it names the installed dashboard. |
| `assets.alerts` | Alert rule files. |
| `data_collected` | `data_collected.json`. |
| `connection_tests.logs` | The log attribute that marks this integration's logs. Metrics are detected by the names in `data_collected.json`. |

For `dashboards`, `pipelines` and `alerts`, an omitted list and `[]` are served differently (`null` and `[]`); keep what the package had.

## Adding an integration

1. Create `integrations/<dir>/` with a manifest, in the `builtin` namespace, starting at `version: 1.0.0`.
2. Write the steps. Collector configs go in fenced `yaml` blocks and must:
   - export only through `otlp/fusescope`, with `endpoint: ${env:OTLP_DESTINATION_ENDPOINT}` and no headers (no ingestion keys);
   - define every receiver and processor their pipelines use;
   - pass `otelcol-contrib validate`, with the values the step `export`s in a `bash` block. A `<placeholder>` value is fine.

   Configs that need components only [fusescope-otel-collector](https://github.com/fuseone-io/fusescope-otel-collector) ships are listed in CI but not validated.
3. Use **FuseScope** in user-facing text; no SigNoz wording or `signoz.io` links.
4. Run the checks and regenerate the docs:

   ```sh
   go run ./cmd/fsint validate
   go run ./cmd/fsint docs
   go test ./...
   ```

5. Open a pull request.

To change an integration, edit its files and bump `version`.

## Checks

CI ([`ci.yaml`](.github/workflows/ci.yaml)) runs on every pull request:

| Job | What |
|---|---|
| `go` | gofmt, `go mod tidy`, vet, tests |
| `packages` | `fsint validate`: schema, files, dashboards, brand, snippets, icons; generated docs in sync |
| `collector-snippets` | `otelcol-contrib validate` on every collector config, in the contrib image |
| `backend-compat` | loads the catalog built from the branch with the loader of `fusescope` `main`, as the backend does at startup, and checks that every package version the backend already embeds renders unchanged. |
| `licenses` | Go dependencies are MIT, BSD, ISC or Apache-2.0 |
| `catalog-dry-run` | builds the catalog and uploads it as an artifact |

A released package version is immutable: changing its files without bumping `version` fails `backend-compat` once the backend embeds that version. The backend picks up a new catalog release with `make integrations-catalog CATALOG_VERSION=vX.Y.Z`.

## Catalog and releases

`fsint catalog` writes, for each package, a reproducible `<id>-<version>.tar.gz` holding the files its manifest references, plus `index.json`:

```json
{
  "catalog_version": 1,
  "source": { "repository": "fuseone-io/fusescope-integrations", "ref": "v1.0.0", "commit": "…" },
  "integrations": [
    {
      "id": "builtin-redis", "version": "1.0.0", "manifest_version": 1, "fusescope": ">=0.154.0",
      "title": "Redis", "description": "…", "categories": ["Database"], "signals": ["logs", "metrics"],
      "package": "builtin-redis-1.0.0.tar.gz", "sha256": "…", "size": 7419
    }
  ]
}
```

Pushing a `v*` tag runs [`release.yaml`](.github/workflows/release.yaml):

1. builds the catalog;
2. signs `index.json` with cosign keyless (GitHub OIDC), writing `index.json.sigstore.json`; the index holds each package's sha256, so the signature covers the packages;
3. uploads `index.json`, the bundle and the tarballs to the GitHub release;
4. pushes them as an OCI artifact, `ghcr.io/fuseone-io/fusescope-integrations-catalog:<version>`, and signs it.

Run manually on a branch, the workflow only builds the catalog.

To verify a release:

```sh
cosign verify-blob index.json \
  --bundle index.json.sigstore.json \
  --certificate-identity "https://github.com/fuseone-io/fusescope-integrations/.github/workflows/release.yaml@refs/tags/v1.0.0" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum -c <(jq -r '.integrations[] | "\(.sha256)  \(.package)"' index.json)
```

## Licence

[MIT](LICENSE). The packages were moved from the FuseScope backend, a fork of SigNoz; see [NOTICE](NOTICE).
