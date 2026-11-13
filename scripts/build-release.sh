#!/usr/bin/env bash
# Builds Code as a separate app for each platform and architecture, each with
# its own libghostty-vt. macOS apps are signed and turned into disk images
# (macOS only); Linux apps are the executable with its packages, cross-compiled
# from any machine.
#
# libghostty-vt is prebuilt once with scripts/build-libghostty.sh (which needs
# Zig) and committed under resources/<os>-<arch>/, so a release copies the
# prebuilt library for its target instead of building it. Set
# CODE_BUILD_LIBGHOSTTY=1 to build a missing one instead of failing.
#
#   scripts/build-release.sh                    # all four
#   scripts/build-release.sh darwin/arm64
#   scripts/build-release.sh darwin/arm64 darwin/amd64
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64)
fi

ref="$(tr -d '[:space:]' < ghostty.version)"

# libghostty_file prints the prebuilt library of a target, or nothing.
libghostty_file() {
  local os="$1" arch="$2"
  ls "resources/$os-$arch/libghostty-vt-local-$os-$arch".* 2>/dev/null | head -1
}

# ensure_libghostty fails when a target's prebuilt library is not in the tree,
# unless CODE_BUILD_LIBGHOSTTY asks for it to be built.
ensure_libghostty() {
  local os="$1" arch="$2" target="$os/$arch"
  [ -n "$(libghostty_file "$os" "$arch")" ] && return
  if [ "${CODE_BUILD_LIBGHOSTTY:-}" = "1" ]; then
    ./scripts/build-libghostty.sh "$ref" "$target"
    return
  fi
  echo "no prebuilt libghostty-vt for $target in resources/$os-$arch/" >&2
  echo "run ./scripts/build-libghostty.sh $ref $target, then commit resources/$os-$arch/" >&2
  exit 1
}

# copy_libghostty puts the target's prebuilt library beside a built app or
# executable, so the release packages it even if mygo does not copy the
# platform resources itself.
copy_libghostty() {
  local os="$1" arch="$2" dest="$3" lib
  lib="$(libghostty_file "$os" "$arch")"
  [ -n "$lib" ] || return
  mkdir -p "$dest"
  cp -f "$lib" "$dest/" || true
}

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
  ensure_libghostty "$os" "$arch"
done

for target in "${targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  dir="build/${target//\//-}"
  case "$target" in
    darwin/*)
      # package.sh copies the prebuilt library into the app, strips MyGo's and
      # signs it, so the copy must happen before signing (inside package.sh).
      ./scripts/package.sh -platform "$target"
      ;;
    linux/*)
      go tool mygo build -platform "$target"
      copy_libghostty "$os" "$arch" "$dir"
      # mygo build bundles its own pinned libghostty-vt beside the executable
      # and inside the .deb and .tar.gz; Code ships only the library
      # scripts/build-libghostty.sh built, so strip MyGo's everywhere.
      go run ./internal/packageclean "$dir"
      ;;
  esac
done

echo
echo "Built:"
ls -d build/*/ 2>/dev/null | sed 's/^/  /'
