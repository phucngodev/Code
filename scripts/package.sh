#!/usr/bin/env bash
# Builds Code without MyGo's bundled libghostty-vt, which Code does not use:
# main.go points the terminal plugin at the library scripts/build-libghostty.sh
# built into resources/. Removing the unused copy keeps the app smaller and
# leaves one libghostty in it. The app is re-signed and a disk image made, as
# mygo build does.
#
#   scripts/package.sh                                  # darwin/arm64
#   scripts/package.sh -platform darwin/universal
#
# Build the library first with scripts/build-libghostty.sh for each platform
# you package. https://developer.apple.com signing and disk images need macOS.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

command -v codesign >/dev/null || { echo "codesign is required (macOS)" >&2; exit 1; }

# Build the app bundles, without disk images.
platforms=()
previous=""
for argument in "$@"; do
  if [ "$previous" = "-platform" ]; then
    IFS=',' read -ra parts <<< "$argument"
    for part in "${parts[@]}"; do platforms+=("build/${part//\//-}"); done
  fi
  previous="$argument"
done

go tool mygo build -skip-dmg "$@"

slug="$(sed -n 's/.*"slug": *"\([^"]*\)".*/\1/p' mygo.json | head -1)"
version="$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' mygo.json | head -1)"
slug="${slug:-code}"
version="${version:-0.0.0}"

# Only the app bundles this run built, so packaging one platform does not
# touch another's.
apps=()
if [ ${#platforms[@]} -eq 0 ]; then
  apps=(build/*/Code.app)
else
  for platform in "${platforms[@]}"; do apps+=("$platform/Code.app"); done
fi

for app in "${apps[@]}"; do
  [ -d "$app" ] || continue
  # The name MyGo's pinned library takes; Code's own has -local-<os>-<arch>.
  rm -f "$app/Contents/Resources/libghostty-vt.dylib"
  if [ -e "$app/Contents/Resources/libghostty-vt.dylib" ]; then
    echo "MyGo's libghostty-vt is still in $app" >&2
    exit 1
  fi
  if ! compgen -G "$app/Contents/Resources/libghostty-vt-local-*" >/dev/null; then
    echo "$app has no libghostty-vt-local-*; build it with ./scripts/build-libghostty.sh first" >&2
    exit 1
  fi
  codesign --force --deep -s - "$app"
  echo "stripped MyGo's library and signed $app"

  case "$app" in
    */darwin-*) suffix="$(basename "$(dirname "$app")")"; suffix="${suffix#darwin-}" ;;
    *) continue ;;
  esac
  dmg="build/$(basename "$(dirname "$app")")/${slug}-${version}-${suffix}.dmg"
  staging="$(mktemp -d)"
  cp -R "$app" "$staging/"
  ln -s /Applications "$staging/Applications"
  rm -f "$dmg"
  hdiutil create -volname "$slug" -srcfolder "$staging" -ov -format UDZO "$dmg" >/dev/null
  rm -rf "$staging"
  echo "created $dmg"
done
