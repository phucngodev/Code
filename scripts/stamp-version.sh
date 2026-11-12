#!/usr/bin/env bash
# Writes the release version into mygo.json, which mygo build copies into the
# app bundle (CFBundleShortVersionString), the disk image names, the Linux
# .deb/.tar.gz and the update archives, so the About window and the file names
# match the git tag. The version is the argument, else VERSION, else the
# GitHub tag (GITHUB_REF_NAME), else `git describe --tags`, with a leading "v"
# dropped. A dirty working tree makes a "-dirty" suffix.
#
#   scripts/stamp-version.sh              # from the git tag or the environment
#   scripts/stamp-version.sh 2.2.0
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

version="${1:-${VERSION:-${GITHUB_REF_NAME:-}}}"
if [ -z "$version" ]; then
  version="$(git describe --tags --always --dirty 2>/dev/null || true)"
fi
version="${version#v}"
if [ -z "$version" ] || [ "$version" = "unknown" ]; then
  echo "could not determine a version; pass one as an argument" >&2
  exit 1
fi

python3 - "$root/mygo.json" "$version" <<'PY'
import json
import sys

path, version = sys.argv[1], sys.argv[2]
with open(path) as f:
    data = json.load(f)
data["version"] = version
with open(path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
PY

echo "stamped mygo.json version = $version"
