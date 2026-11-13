#!/usr/bin/env bash
# Builds libghostty-vt from a Ghostty ref (tag or commit) and installs it into
# the resources, so this Code uses that revision. main.go points MyGo's
# terminal plugin at the built library through MYGO_GHOSTTY_VT. The built
# libraries are committed under resources/<os>-<arch>/, so a release copies
# them instead of building them; run this and commit resources/ when the ref
# changes.
#
#   scripts/build-libghostty.sh                 # host platform, the pinned ref
#   scripts/build-libghostty.sh v1.5.0
#   scripts/build-libghostty.sh v1.5.0 linux/amd64
#   scripts/build-libghostty.sh v1.5.0 all      # every supported target
#
# The app supports macOS (arm64 and amd64) and Linux (amd64 and arm64). The
# ref to build by default is in ghostty.version; pass one to override it.
#
# Needs `zig` (the version Ghostty asks for) and `git`. A build of Ghostty is
# cloned into a cache; set GHOSTTY_SRC to use an existing checkout instead.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
default_ref="$(tr -d '[:space:]' < "$root/ghostty.version" 2>/dev/null || true)"
default_ref="${default_ref:-v1.4.0}"
ref="${1:-$default_ref}"
target="${2:-host}"

cache="${XDG_CACHE_HOME:-$HOME/.cache}/code/ghostty-src"
resources="$root/resources"

command -v zig >/dev/null || { echo "zig is required" >&2; exit 1; }
command -v git >/dev/null || { echo "git is required" >&2; exit 1; }

# A ref of forty hex characters is already a commit.
if [[ "$ref" =~ ^[0-9a-f]{40}$ ]]; then
  commit="$ref"
else
  commit="$(git ls-remote https://github.com/ghostty-org/ghostty "refs/tags/$ref^{}" 2>/dev/null | awk 'NR==1{print $1}')"
  if [ -z "$commit" ]; then
    commit="$(git ls-remote https://github.com/ghostty-org/ghostty "$ref" 2>/dev/null | awk 'NR==1{print $1}')"
  fi
fi
[ -n "$commit" ] || { echo "could not resolve $ref" >&2; exit 1; }

# Locate Ghostty's sources at the commit.
if [ -n "${GHOSTTY_SRC:-}" ]; then
  src="$GHOSTTY_SRC"
  have="$(git -C "$src" rev-parse HEAD 2>/dev/null || true)"
  if [ "$have" != "$commit" ]; then
    echo "warning: $src is at ${have:-unknown}, not $commit" >&2
  fi
else
  src="$cache/$commit"
  if [ ! -d "$src/.git" ]; then
    mkdir -p "$cache"
    git clone --filter=blob:none https://github.com/ghostty-org/ghostty "$src"
  fi
  # A checkout that already has the commit needs no fetch. Otherwise fetch,
  # forcing the moving `tip` tag the remote rewrites, which a plain
  # `fetch --tags` refuses to clobber.
  if ! git -C "$src" cat-file -e "$commit^{commit}" 2>/dev/null; then
    git -C "$src" fetch --tags --force origin
  fi
  git -C "$src" checkout --detach "$commit"
fi

zig_target() {
  case "$1/$2" in
    darwin/arm64)   echo aarch64-macos ;;
    darwin/amd64)   echo x86_64-macos ;;
    linux/amd64)    echo x86_64-linux-gnu.2.28 ;;
    linux/arm64)    echo aarch64-linux-gnu.2.28 ;;
    windows/amd64)  echo x86_64-windows-gnu ;;
    windows/arm64)  echo aarch64-windows-gnu ;;
    *) echo "unsupported target $1/$2" >&2; return 1 ;;
  esac
}

lib_name() {
  case "$1" in
    darwin)  echo "libghostty-vt-local-$1-$2.dylib" ;;
    windows) echo "libghostty-vt-local-$1-$2.dll" ;;
    *)       echo "libghostty-vt-local-$1-$2.so" ;;
  esac
}

built_path() {
  case "$1" in
    darwin)  echo "lib/libghostty-vt.dylib" ;;
    windows) echo "bin/ghostty-vt.dll" ;;
    *)       echo "lib/libghostty-vt.so" ;;
  esac
}

build_one() {
  local goos="$1" goarch="$2"
  local zig built name prefix
  zig="$(zig_target "$goos" "$goarch")"
  built="$(built_path "$goos")"
  name="$(lib_name "$goos" "$goarch")"
  prefix="$(mktemp -d)"
  echo "building $goos/$goarch ($zig)"
  ( cd "$src" && zig build -Demit-lib-vt -Demit-xcframework=false -Dtarget="$zig" -Doptimize=ReleaseFast -Dstrip=true --prefix "$prefix" )
  mkdir -p "$resources/$goos-$goarch"
  cp "$prefix/$built" "$resources/$goos-$goarch/$name"
  rm -rf "$prefix"
  if [ "$goos" = linux ]; then
    # Zig keeps the debug sections of a shared library even with -Dstrip=true.
    ( cd "$root" && go run ./internal/elfstrip "$resources/$goos-$goarch/$name" )
  fi
  echo "installed resources/$goos-$goarch/$name"
}

targets=()
case "$target" in
  host) targets=("$(go env GOOS)/$(go env GOARCH)") ;;
  all)  targets=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64) ;;
  */*)  targets=("$target") ;;
  *)    echo "target is host, all, or goos/goarch" >&2; exit 1 ;;
esac

for t in "${targets[@]}"; do
  build_one "${t%/*}" "${t#*/}"
done

printf '%s\n' "$ref" > "$resources/libghostty-vt-local.version"
echo "Code now uses libghostty-vt $ref ($commit)"