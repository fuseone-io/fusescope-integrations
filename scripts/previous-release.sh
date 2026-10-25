#!/usr/bin/env bash
# usage: previous-release.sh <dir> [<tag>]
#
# Downloads into <dir> the latest catalog release older than <tag> (by default
# the latest release) and verifies index.json against the release workflow of
# that release's tag. fsint checks the packages against the verified index.
set -euo pipefail

dir=${1:?usage: previous-release.sh <dir> [<tag>]}
current=${2:-}
repo=${GITHUB_REPOSITORY:-fuseone-io/fusescope-integrations}

tags=$(gh release list -R "$repo" --exclude-drafts --exclude-pre-releases --limit 1000 \
  --json tagName --jq '.[].tagName' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | grep -vxF "${current:-none}" || true)
if [ -n "$current" ]; then
  previous=$(printf '%s\n%s\n' "$tags" "$current" | sed '/^$/d' | sort -V | grep -xF -B1 "$current" | grep -vxF "$current" || true)
else
  previous=$(printf '%s\n' "$tags" | sed '/^$/d' | sort -V | tail -n1)
fi
if [ -z "$previous" ]; then
  echo "no release of $repo before ${current:-now}" >&2
  exit 1
fi

mkdir -p "$dir"
gh release download "$previous" -R "$repo" -D "$dir" --clobber \
  -p index.json -p index.json.sigstore.json -p '*.tar.gz'
cosign verify-blob "$dir/index.json" \
  --bundle "$dir/index.json.sigstore.json" \
  --certificate-identity "https://github.com/$repo/.github/workflows/release.yaml@refs/tags/$previous" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com >&2
echo "$previous"
