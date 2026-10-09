#!/usr/bin/env bash
set -euo pipefail

if (($# != 2)) || [[ -z "$2" ]]; then
  printf 'Usage: %s VERSION OUTPUT_DIR\n' "$0" >&2
  exit 2
fi
version=$1
if [[ ! "$version" =~ ^[0-9]+(\.[0-9]+)+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]]; then
  printf 'VERSION must be numeric and dotted, with an optional prerelease suffix.\n' >&2
  exit 2
fi
if [[ ${GITHUB_REF:-} == refs/tags/* ]]; then
  tag=${GITHUB_REF#refs/tags/}
  if [[ ${tag#v} != "$version" ]]; then
    printf 'Tag and candidate VERSION must match after removing one leading v.\n' >&2
    exit 1
  fi
fi

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
if [[ -n $(git -C "$repo_root" status --porcelain --untracked-files=all) ]]; then
  printf 'Release candidates require a clean checkout, including untracked files.\n' >&2
  exit 1
fi
output_dir=$(realpath -m -- "$2")
if [[ "$output_dir" == "$repo_root" || "$output_dir" == "$repo_root/"* ]]; then
  printf 'OUTPUT_DIR must be outside the source tree.\n' >&2
  exit 1
fi
mkdir -p -- "$output_dir"
if [[ -n $(find "$output_dir" -mindepth 1 -maxdepth 1 -print -quit) ]]; then
  printf 'OUTPUT_DIR must be empty.\n' >&2
  exit 1
fi

syft=${SYFT:-syft}
schema=${SPDX_SCHEMA:?Set SPDX_SCHEMA to the pinned SPDX 2.3 schema file.}
source "$repo_root/.github/ci-tools.env"
if [[ $(go env GOVERSION) != go1.27.2 ]]; then
  printf 'Release candidates require Go 1.27.2.\n' >&2
  exit 1
fi
if [[ $("$syft" version -o json | python3 -c 'import json,sys; print("v" + json.load(sys.stdin)["version"])') != "$SYFT_VERSION" ]]; then
  printf 'Release candidates require Syft %s.\n' "$SYFT_VERSION" >&2
  exit 1
fi
test -f "$schema"

bash "$repo_root/scripts/package.sh" "$version" "$output_dir"
stage=$(mktemp -d /tmp/azerlay-release-XXXXXXXX)
trap 'rm -rf -- "$stage"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
name="azerlay-$version"
mkdir -- "$stage/source" "$stage/binary"
tar -xzf "$output_dir/$name-source.tar.gz" -C "$stage/source"
tar -xzf "$output_dir/$name-linux-x86_64.tar.gz" -C "$stage/binary"
for artifact in source binary; do
  "$syft" scan "dir:$stage/$artifact/$name" --source-name "$name-$artifact" --source-version "$version" \
    -o "syft-json=$stage/$artifact.syft.json" -o "spdx-json@2.3=$stage/$artifact.spdx.json"
done
"$syft" scan dir:/var/lib/pacman/local --override-default-catalogers alpm-db-cataloger \
  -o "syft-json=$stage/host.syft.json"
CGO_ENABLED=1 python3 "$repo_root/scripts/release_inventory.py" \
  --source-root "$stage/source/$name" --source-archive "$output_dir/$name-source.tar.gz" \
  --binary "$stage/binary/$name/bin/azerlay" --version "$version" \
  --syft-source "$stage/source.syft.json" --syft-binary "$stage/binary.syft.json" \
  --syft-host "$stage/host.syft.json" --output "$stage/inventory.json"
for artifact in source binary; do
  base="$name-source"
  if [[ "$artifact" == binary ]]; then
    base="$name-linux-x86_64"
  fi
  python3 "$repo_root/scripts/release_report.py" --inventory "$stage/inventory.json" \
    --syft-spdx "$stage/$artifact.spdx.json" --artifact "$artifact" \
    --sbom "$output_dir/$base.spdx.json" --report "$output_dir/$base.licenses.json" | tee "$stage/$artifact-review.txt"
  python3 -m jsonschema --instance "$output_dir/$base.spdx.json" "$schema"
  python3 "$repo_root/scripts/release_report.py" --check \
    --sbom "$output_dir/$base.spdx.json" --report "$output_dir/$base.licenses.json"
  if [[ -n ${GITHUB_STEP_SUMMARY:-} ]]; then
    cat "$stage/$artifact-review.txt" >> "$GITHUB_STEP_SUMMARY"
  fi
done
bash "$repo_root/scripts/release-notes.sh" "$version" "$output_dir/release-notes.md"
(
  cd -- "$output_dir"
  sha256sum -- "$name-source.tar.gz" "$name-linux-x86_64.tar.gz" \
    "$name-source.spdx.json" "$name-linux-x86_64.spdx.json" \
    "$name-source.licenses.json" "$name-linux-x86_64.licenses.json" release-notes.md > SHA256SUMS
)
printf 'Created release candidate in %s. Publication readiness is not established.\n' "$output_dir"
