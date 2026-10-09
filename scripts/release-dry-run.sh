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

bash "$repo_root/scripts/package.sh" "$version" "$output_dir"
