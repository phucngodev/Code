#!/bin/bash
# Builds Code for arm64 and x86_64, one DMG each, without notarizing.
# For a notarized, publishable release use scripts/release.py.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD="$ROOT/build/release"
DIST="$ROOT/dist"
ARCHS=(arm64 x86_64)

command -v xcodegen >/dev/null || { echo "xcodegen not found; brew install xcodegen" >&2; exit 1; }

mkdir -p "$DIST"
(cd "$ROOT" && xcodegen generate --quiet)

VERSION=
for arch in "${ARCHS[@]}"; do
  echo "==> Building $arch"
  # Xcode can run its own Info.plist processing after the version-stamping
  # phase, so a reused derived data folder would un-stamp the version.
  rm -rf "$BUILD/$arch"
  # A CI runner has no signing identity and turns signing off through these.
  xcodebuild -scheme code -configuration Release \
    -derivedDataPath "$BUILD/$arch" \
    ARCHS="$arch" ONLY_ACTIVE_ARCH=NO \
    "CODE_SIGNING_ALLOWED=${CODE_SIGNING_ALLOWED:-YES}" \
    "CODE_SIGNING_REQUIRED=${CODE_SIGNING_REQUIRED:-YES}" \
    build -quiet

  app="$BUILD/$arch/Build/Products/Release/Code.app"

  # A fat app would carry the other chip's code and defeat the split.
  archs="$(lipo -archs "$app/Contents/MacOS/Code")"
  if [ "$archs" != "$arch" ]; then
    echo "the $arch app is built for $archs" >&2
    exit 1
  fi

  # Both DMGs name one version, so the two builds must agree on it.
  built="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")"
  if [ -n "$VERSION" ] && [ "$VERSION" != "$built" ]; then
    echo "$arch is version $built, but the first build was $VERSION" >&2
    exit 1
  fi
  VERSION="$built"
done

for arch in "${ARCHS[@]}"; do
  echo "==> Packaging $arch"
  app="$BUILD/$arch/Build/Products/Release/Code.app"
  dmg="$DIST/Code-$VERSION-$arch.dmg"

  # An Applications link beside the app makes the image drag-to-install like a
  # downloaded one. ditto copies the bundle so its signature stays valid.
  stage="$(mktemp -d)"
  ditto "$app" "$stage/Code.app"
  ln -s /Applications "$stage/Applications"
  hdiutil create -quiet -volname "Code" -srcfolder "$stage" -ov -format UDZO "$dmg"
  rm -rf "$stage"
  echo "    $dmg"
done
