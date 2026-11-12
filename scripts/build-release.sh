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

# The app, its packages and the About window carry the release version, which
# comes from the git tag (or VERSION). A local run restores mygo.json on exit.
version_backup="$(mktemp)"
cp mygo.json "$version_backup"
trap 'mv -f "$version_backup" mygo.json' EXIT
./scripts/stamp-version.sh

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
    linux/*)
      go tool mygo build -platform "$target"
      # mygo build bundles its own pinned libghostty-vt beside the executable
      # and inside the .deb and .tar.gz; Code ships only the library
      # scripts/build-libghostty.sh built, so strip MyGo's everywhere.
      go run ./internal/packageclean "build/${target//\//-}"
      ;;
  esac
done

echo
echo "Built:"
ls -d build/*/ 2>/dev/null | sed 's/^/  /'
