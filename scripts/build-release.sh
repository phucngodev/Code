#!/usr/bin/env bash
# Builds Code as a separate app for each platform and architecture, each with
# its own libghostty-vt. macOS apps are signed and turned into disk images
# (macOS only); Linux apps are the executable with its packages, cross-compiled
# from any machine.
#
#   scripts/build-release.sh                    # all four
#   scripts/build-release.sh darwin/arm64
#   scripts/build-release.sh darwin/arm64 darwin/amd64
#
# Needs Zig (the version Ghostty asks for) and git to build libghostty-vt the
# first time; see scripts/build-libghostty.sh. The results are under build/.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64)
fi

ref="$(tr -d '[:space:]' < ghostty.version)"

for target in "${targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  case "$os/$arch" in
    darwin/arm64|darwin/amd64|linux/amd64|linux/arm64) ;;
    *) echo "unsupported target $target (want darwin/arm64, darwin/amd64, linux/amd64 or linux/arm64)" >&2; exit 1 ;;
  esac
  # Build the matching libghostty-vt when it is not there yet.
  if ! compgen -G "resources/$os-$arch/libghostty-vt-local-$os-$arch.*" >/dev/null; then
    ./scripts/build-libghostty.sh "$ref" "$target"
  fi
done

for target in "${targets[@]}"; do
  case "$target" in
    darwin/*) ./scripts/package.sh -platform "$target" ;;
    linux/*)  go tool mygo build -platform "$target" ;;
  esac
done

echo
echo "Built:"
ls -d build/*/ 2>/dev/null | sed 's/^/  /'
