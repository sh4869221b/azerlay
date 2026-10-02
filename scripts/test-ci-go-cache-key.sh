#!/usr/bin/env bash
# Contract tests use synthetic tool output; native behavior is checked by CI.
set -euo pipefail
script=$(cd "$(dirname "$0")" && pwd)/ci-go-cache-key.sh
fixture=$(mktemp -d)
trap 'rm -rf -- "$fixture"' EXIT
mkdir "$fixture/bin" "$fixture/repo"
cat > "$fixture/bin/go" <<'MOCK'
#!/usr/bin/env bash
[[ ${MOCK_FAIL:-} != go ]] || exit 17
printf '%s\n' "go=${MOCK_GO:-1}; settings=${MOCK_SETTINGS:-1}; command=$*"
MOCK
cat > "$fixture/bin/pacman" <<'MOCK'
#!/usr/bin/env bash
[[ ${MOCK_FAIL:-} != pacman ]] || exit 18
printf 'glibc 1\ngtk4 1\npango %s\n' "${MOCK_NATIVE:-1}"
MOCK
cat > "$fixture/bin/pkg-config" <<'MOCK'
#!/usr/bin/env bash
[[ ${MOCK_FAIL:-} != pkg-config ]] || exit 19
printf 'native-flags %s\n' "$*"
MOCK
chmod +x "$fixture/bin/"*
export PATH="$fixture/bin:$PATH"
cd "$fixture/repo"
git init -q
printf 'module synthetic\n' > go.mod
printf 'synthetic dependency\n' > go.sum
printf 'package synthetic\n' > source.go
git add .
key() { bash "$script" "$fixture/manifest"; }
original=$(key)
[[ $original == "$(key)" ]]
printf '// source-only change\n' >> source.go
git add source.go
changed=$(key)
[[ $(grep '^prefix=' <<< "$original") == "$(grep '^prefix=' <<< "$changed")" ]]
[[ $(grep '^key=' <<< "$original") != "$(grep '^key=' <<< "$changed")" ]]
# A dependency change must not restore across the old dependency boundary.
printf 'new dependency\n' >> go.sum
dependencies=$(key)
[[ $(grep '^prefix=' <<< "$changed") != "$(grep '^prefix=' <<< "$dependencies")" ]]
# Full native inventory includes transitive dependencies such as Pango.
native=$(MOCK_NATIVE=2 key)
toolchain=$(MOCK_GO=2 key)
settings=$(MOCK_SETTINGS=2 key)
for result in "$native" "$toolchain" "$settings"; do
  [[ $(grep '^prefix=' <<< "$dependencies") != "$(grep '^prefix=' <<< "$result")" ]]
done
# Tracked non-Go assets also rotate the generation, without widening fallback.
printf 'synthetic embedded asset\n' > asset.txt
git add asset.txt
asset=$(key)
[[ $(grep '^prefix=' <<< "$dependencies") == "$(grep '^prefix=' <<< "$asset")" ]]
[[ $(grep '^key=' <<< "$dependencies") != "$(grep '^key=' <<< "$asset")" ]]
[[ $asset == "$(key)" ]]
# Never publish a usable key from a partial/failed fingerprint.
for tool in pacman go pkg-config; do
  if output=$(MOCK_FAIL=$tool key); then
    printf 'Expected %s failure to reject the fingerprint\n' "$tool" >&2
    exit 1
  fi
  [[ -z $output ]]
done
printf 'Go cache key contract tests passed\n' 
