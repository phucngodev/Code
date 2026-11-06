#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Builds, signs, notarizes and packages a Code release per CPU
architecture. Nothing leaves this Mac except the notarization upload to
Apple, unless --publish is given.

  uv run scripts/release.py 0.1.0            build dist/Code-0.1.0-<arch>.dmg for arm64 and x86_64
  uv run scripts/release.py 0.1.0 --no-wait  the same, but return while Apple is still
                                             notarizing; rerun to check again
  uv run scripts/release.py 0.1.1-rc.1       a release candidate: a GitHub pre-release
  uv run scripts/release.py 0.1.0 --publish  publish exactly those files: push the branch
                                             and tag, and create the GitHub release

Needs: a Developer ID Application certificate for TEAM, the NOTARY_PROFILE
notarytool keychain profile, create-dmg, and gh.
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

TEAM = "Y2X9JDX474"
REPO = "phucngodev/code"
NOTARY_PROFILE = "code-notary"
CHANGELOG = "https://getcode.app/changelog/"
ARCHS = ("arm64", "x86_64")

ROOT = Path(__file__).resolve().parent.parent
BUILD = ROOT / "build" / "release"
DIST = ROOT / "dist"


class ReleaseError(Exception):
    pass


def step(message):
    print(f"\n==> {message}", flush=True)


def run(*args, check=True, capture=False, quiet=False):
    result = subprocess.run(
        [str(arg) for arg in args],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE if capture or quiet else None,
        stderr=subprocess.DEVNULL if quiet else None,
    )
    if check and result.returncode != 0:
        raise ReleaseError(f"{args[0]} failed (exit {result.returncode}): {' '.join(map(str, args))}")
    return result


def output(*args):
    return run(*args, capture=True).stdout.strip()


def succeeds(*args):
    return run(*args, check=False, quiet=True).returncode == 0


# --- Decisions, kept free of side effects so they can be tested ---


def tag_action(tag, tag_commit, head, pushed):
    """A tag that was never pushed can follow HEAD when a failed build is
    fixed and rerun; a pushed tag is never moved."""
    if tag_commit is None:
        return "create"
    if tag_commit == head:
        return "keep"
    if pushed:
        raise ReleaseError(f"{tag} is already pushed at another commit")
    return "move"


def notarization_result(text):
    """Apple's verdict and submission id, or ("unknown", None) when notarytool
    gave no readable answer."""
    try:
        data = json.loads(text)
        return data.get("status", "unknown"), data.get("id")
    except (json.JSONDecodeError, AttributeError):
        return "unknown", None


VERSION = re.compile(r"\d+\.\d+\.\d+(-rc\.\d+)?")


def is_prerelease(version):
    return "-" in version


def can_resume(state, version, tag_commit):
    """A build in dist/ is picked up again only if it is this version, built
    from the commit the tag still points at."""
    return bool(state) and state.get("version") == version and tag_commit is not None \
        and state.get("commit") == tag_commit


def developer_id(find_identity_output, team):
    match = re.search(rf'"(Developer ID Application: .*\({team}\))"', find_identity_output)
    return match.group(1) if match else None


# --- Steps ---


class Release:
    def __init__(self, version):
        self.version = version
        self.tag = f"v{version}"
        self.state_file = DIST / "release.json"
        self.page_url = f"{CHANGELOG}{version}/"
        # GitHub never counts a pre-release as the latest release, so a
        # candidate reaches only the people who download it.
        self.prerelease = is_prerelease(version)

    def app(self, arch):
        """The app this architecture built, under its own derived data."""
        return BUILD / arch / "Build" / "Products" / "Release" / "Code.app"

    def dmg(self, arch):
        return DIST / f"Code-{self.version}-{arch}.dmg"

    def dmgs_built(self):
        return all(self.dmg(arch).exists() for arch in ARCHS)

    def state(self):
        try:
            return json.loads(self.state_file.read_text())
        except (OSError, json.JSONDecodeError):
            return None

    def save_state(self, **fields):
        self.state_file.write_text(json.dumps({**(self.state() or {}), **fields}, indent=2) + "\n")

    def release_exists(self):
        return succeeds("gh", "release", "view", self.tag, "--repo", REPO)

    def tag_commit(self):
        result = run("git", "rev-parse", "-q", "--verify", f"refs/tags/{self.tag}^{{commit}}", check=False, capture=True)
        return result.stdout.strip() if result.returncode == 0 else None

    def head(self):
        return output("git", "rev-parse", "HEAD")

    def prepare_tag(self):
        commit = self.tag_commit()
        pushed = commit is not None and succeeds("git", "ls-remote", "--exit-code", "--tags", "origin", f"refs/tags/{self.tag}")
        action = tag_action(self.tag, commit, self.head(), pushed)
        if action == "create":
            run("git", "tag", "-a", self.tag, "-m", f"Code {self.version}")
            print(f"tagged HEAD as {self.tag} (local only until --publish)")
        elif action == "move":
            run("git", "tag", "-f", "-a", self.tag, "-m", f"Code {self.version}", quiet=True)
            print(f"moved the unpublished tag {self.tag} to HEAD")

    def check(self):
        step("Checking the tree and tools")
        if output("git", "status", "--porcelain"):
            raise ReleaseError("commit or stash your changes first")
        identity = developer_id(output("security", "find-identity", "-v", "-p", "codesigning"), TEAM)
        if not identity:
            raise ReleaseError(f"no Developer ID Application certificate for team {TEAM}")
        if not shutil.which("create-dmg"):
            raise ReleaseError("create-dmg not found (npm install --global create-dmg)")
        if not shutil.which("gh"):
            raise ReleaseError("gh not found")
        if not succeeds("xcrun", "notarytool", "history", "--keychain-profile", NOTARY_PROFILE):
            raise ReleaseError(f"notarytool profile {NOTARY_PROFILE} is missing")
        if self.release_exists():
            raise ReleaseError(f"release {self.tag} already exists on GitHub; published releases are never replaced")
        stamp = ROOT / "Vendor" / "GhosttyKit.stamp"
        if not stamp.exists() or stamp.read_text().strip() != output(ROOT / "scripts" / "ghosttykit-stamp.sh"):
            raise ReleaseError("libghostty is out of date or unrecorded; rebuild it with scripts/build-ghosttykit.sh")
        self.prepare_tag()
        print(f"signing as: {identity}")
        return identity

    def build(self):
        shutil.rmtree(BUILD, ignore_errors=True)
        shutil.rmtree(DIST, ignore_errors=True)
        DIST.mkdir(parents=True)
        run("xcodegen", "generate", "--quiet")
        numbers = set()
        for arch in ARCHS:
            step(f"Building Release ({arch})")
            run("xcodebuild", "-scheme", "code", "-configuration", "Release",
                "-derivedDataPath", BUILD / arch, f"ARCHS={arch}", "ONLY_ACTIVE_ARCH=NO", "build", "-quiet")
            app = self.app(arch)
            lipo = output("lipo", "-archs", app / "Contents/MacOS/Code").split()
            # A fat binary would carry the other chip's code and undo the split.
            if lipo != [arch]:
                raise ReleaseError(f"the {arch} app is built for {lipo}, expected just {arch}")
            plist = app / "Contents" / "Info.plist"
            built = output("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString", plist)
            numbers.add(output("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleVersion", plist))
            if built != self.version:
                raise ReleaseError(f"the {arch} app says {built}, expected {self.version}")
            print(f"Code {built} ({arch})")
        if len(numbers) != 1:
            raise ReleaseError(f"the builds disagree on the build number: {sorted(numbers)}")
        return int(numbers.pop())

    def sign(self, identity):
        step("Signing")

        def sign(*args):
            run("codesign", "--force", "--sign", identity, "--options", "runtime", "--timestamp", *args)

        for arch in ARCHS:
            app = self.app(arch)
            # The helper inside the app is signed before the app that encloses it.
            sign(app / "Contents" / "Helpers" / "cde")
            sign("--entitlements", ROOT / "Resources" / "Code.entitlements", app)
            run("codesign", "--verify", "--deep", "--strict", app)
            entitlements = run("codesign", "-d", "--entitlements", "-", app, quiet=True).stdout
            if "get-task-allow" in entitlements:
                raise ReleaseError(f"the {arch} app carries get-task-allow, which notarization rejects")

    def package(self, identity):
        step("Packaging")
        for arch in ARCHS:
            run("create-dmg", self.app(arch), DIST, "--overwrite", f"--identity={identity}", quiet=True)
            (DIST / f"Code {self.version}.dmg").rename(self.dmg(arch))
            print(self.dmg(arch))

    def notarize(self, wait):
        results = [self.notarize_arch(arch, wait) for arch in ARCHS]
        return all(results)

    def notarize_arch(self, arch, wait):
        """Submits one architecture and returns whether it is done.

        The submission id is saved before waiting, so a wait cut short by
        sleep or a lost connection resumes on the next run; Apple carries on
        with the submission either way. Without waiting, Apple is asked once
        and False means it is still working."""
        step(f"Notarizing {arch} (usually minutes, but a new account's first uploads can take hours)")
        dmg = self.dmg(arch)
        submissions = (self.state() or {}).get("submissions", {})
        submission = submissions.get(arch)
        if submission:
            print(f"resuming submission {submission}")
        else:
            submitted = run("xcrun", "notarytool", "submit", dmg, "--keychain-profile", NOTARY_PROFILE,
                            "--output-format", "json", capture=True, check=False)
            submission = notarization_result(submitted.stdout)[1]
            if not submission:
                raise ReleaseError(f"notarytool did not accept the {arch} upload")
            self.save_state(submissions={**submissions, arch: submission})
            print(f"submitted {submission}")
        if wait:
            print(f"waiting for Apple; if this stops, rerun: uv run scripts/release.py {self.version}")
        answer = run("xcrun", "notarytool", "wait" if wait else "info", submission,
                     "--keychain-profile", NOTARY_PROFILE, "--output-format", "json", capture=True, check=False)
        report = DIST / f"notarization-{arch}.json"
        report.write_text(answer.stdout)
        status = notarization_result(answer.stdout)[0]
        if status == "In Progress" and not wait:
            return False
        if status == "unknown":
            raise ReleaseError(f"lost track of the {arch} submission {submission} while waiting; "
                               f"rerun uv run scripts/release.py {self.version} to keep waiting")
        if status != "Accepted":
            log = DIST / f"notarization-{arch}-log.json"
            run("xcrun", "notarytool", "log", submission, "--keychain-profile", NOTARY_PROFILE, log, check=False)
            self.state_file.unlink(missing_ok=True)
            raise ReleaseError(f"the {arch} notarization was {status}; see {report} and {log}")
        run("xcrun", "stapler", "staple", dmg)
        run("spctl", "--assess", "--type", "open", "--context", "context:primary-signature", dmg)
        print(f"{arch} notarized and stapled")
        return True

    def make(self, wait=True):
        state = self.state()
        if can_resume(state, self.version, self.tag_commit()) and self.dmgs_built():
            if self.release_exists():
                raise ReleaseError(f"release {self.tag} already exists on GitHub")
            step(f"Resuming the {self.version} build from {state['commit'][:10]}")
        else:
            identity = self.check()
            build_number = self.build()
            self.sign(identity)
            self.package(identity)
            self.save_state(version=self.version, commit=self.tag_commit(), build=build_number)
        if not self.notarize(wait):
            step(f"Apple is still notarizing. Check again with: uv run scripts/release.py {self.version} --no-wait")
            return
        step(f"Done. Review dist/, then run: uv run scripts/release.py {self.version} --publish")

    def publish(self):
        step(f"Publishing {self.tag}")
        if not self.dmgs_built():
            raise ReleaseError(f"build the release first: uv run scripts/release.py {self.version}")
        if self.release_exists():
            raise ReleaseError(f"release {self.tag} already exists on GitHub")
        tag_commit = self.tag_commit()
        if not can_resume(self.state(), self.version, tag_commit):
            raise ReleaseError(f"dist/ was not built from {self.tag}; build the release again")
        if not succeeds("git", "merge-base", "--is-ancestor", tag_commit, "HEAD"):
            raise ReleaseError(f"{self.tag} is not on the current branch")
        for arch in ARCHS:
            if not succeeds("xcrun", "stapler", "validate", self.dmg(arch)):
                raise ReleaseError(f"{self.dmg(arch)} is not notarized")
        branch = output("git", "branch", "--show-current")
        if not branch:
            raise ReleaseError("check out the branch being released")
        run("git", "push", "origin", branch, self.tag)
        archives = [self.dmg(arch) for arch in ARCHS]
        if self.prerelease:
            run("gh", "release", "create", self.tag, *archives, "--prerelease", "--repo", REPO,
                "--title", f"Code {self.version}", "--notes", "A release candidate for testing.")
        else:
            run("gh", "release", "create", self.tag, *archives, "--repo", REPO,
                "--title", f"Code {self.version}", "--notes", f"Release notes: {self.page_url}")
        print(f"published https://github.com/{REPO}/releases/tag/{self.tag}")


def main():
    parser = argparse.ArgumentParser(description="Build, or publish, a Code release.")
    parser.add_argument("version", help="X.Y.Z, or X.Y.Z-rc.N for a release candidate")
    parser.add_argument("--publish", action="store_true", help="publish the release already built in dist/")
    parser.add_argument("--no-wait", action="store_true", help="return while Apple is still notarizing")
    args = parser.parse_args()
    if not VERSION.fullmatch(args.version):
        parser.error("version must be X.Y.Z or X.Y.Z-rc.N")

    os.environ.setdefault("DEVELOPER_DIR", "/Applications/Xcode.app/Contents/Developer")
    sys.stdout.reconfigure(line_buffering=True)
    release = Release(args.version)
    try:
        release.publish() if args.publish else release.make(wait=not args.no_wait)
    except ReleaseError as error:
        sys.exit(f"error: {error}")


if __name__ == "__main__":
    main()
