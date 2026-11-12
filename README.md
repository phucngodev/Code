# Code (cross-platform)

Code is a terminal built on Ghostty's emulator, with split panes, workspaces
and agent status. It is written in Go with mygo UI [MyGo](https://github.com/egoist/mygo)'s
**native UI** — no web view, no Electron. It builds and runs on **macOS**
(arm64 and amd64) and **Linux** (amd64 and arm64).

## What is ported

| Area | |
|---|---|
| Workspaces | sidebar list, accents, rename, reorder-ready model, per-workspace tabs |
| Tabs | strip in the top bar, select, close, new |
| Split panes | right/down, nested, drag-resized dividers, zoom, percentage ratios |
| Pane focus | click, or ⌥⌘ + arrow keys |
| Terminals | mygo `terminal` view: colors, Unicode, scrollback, mouse, clipboard, find |
| Theme | Ghostty's `theme`, `font-family` list, `font-size`, `font-feature`, `adjust-cell-height`, `window-padding-*`, `macos-option-as-alt`; the chrome follows the terminal colors, and Code ships `modus` (the default), `everforest`, `gruvbox`, `kanagawa`, `nord` and `rosepine` |
| Chrome | the tab strip and actions sit in the drawn title bar, under the window controls, with room left for the traffic lights |
| Agents | Claude Code, Codex, OpenCode, Pi and PiG report through Code's `cde` helper: PATH shims for the hook-based agents, Code-installed extensions for Pi and PiG, the full hook translation, status dots, unread counts, desktop notifications and a Dock badge |
| Settings | a window over `~/.config/code/config` in Ghostty's format |
| Command palette | ⌘P, lists workspaces, tabs and panes, recent first |
| Session | windows (size, position, sidebar state), workspaces, tabs and split layouts restored on relaunch |
| Menus | Code / File / Edit / View / Workspace / Window / Help |
| Keyboard shortcuts | listed in the Help window |

On macOS, Code sets its own `AppleFontSmoothing` default to 0 before the first
frame, so text is drawn thin rather than thickened by the system's Core Text
font smoothing, as the Swift app's is. Set `CODE_FONT_SMOOTHING=1` to keep the
system's look.

## What is not ported yet

These are deliberately out of this pass:

- alert **sounds** (the Settings controls persist but are not played),
- secure keyboard entry, About/Licenses windows, translucent sidebar material.

## Requirements

- [Go](https://go.dev/dl/) 1.27 or later.
- Native UI needs nothing more; Linux builds need GTK 3.
- The emulator library downloads once on first run (packaged builds embed it).

## Run and build

```sh
go run .                 # run the development app
go test ./...            # the model, config, agent and view tests
go tool mygo dev         # dev with auto-restart (after `go get -tool github.com/egoist/mygo/cmd/mygo`)
go tool mygo build       # package for this machine under build/<os>-<arch>/
```

The MyGo module is pinned in `go.mod`; `mygo.json` names the app **Code**
(identifier `dev.phucngo.code`). To build and install a packaged app for a
platform, use `scripts/build-release.sh <platform>`; on Linux its
`build/<os>-<arch>/install.sh` installs it for the current user (see below).

## The libghostty version

MyGo's terminal plugin pins its own `libghostty-vt` revision (a Ghostty
`main` commit). Code instead builds the library it wants and points the plugin
at it, so it can track Ghostty releases — the macOS app is on **v1.4.0**.

```sh
scripts/build-libghostty.sh                  # host platform, v1.4.0
scripts/build-libghostty.sh v1.5.0           # another tag or commit
scripts/build-libghostty.sh v1.5.0 all        # darwin arm64+amd64, linux amd64+arm64
```

It builds `libghostty-vt` with Zig (the version Ghostty asks for), installs it
in this directory's `resources/` as `libghostty-vt-local-<os>-<arch>.<ext>`,
and records the ref in `resources/libghostty-vt-local.version`. `main.go` sets
`MYGO_GHOSTTY_VT` to that file, which the terminal plugin loads first, so both
`go run .` and a packaged app use it. `MYGO_GHOSTTY_VT` set yourself still wins.

Set `GHOSTTY_SRC` to build from a checkout you already have at the tag,
instead of cloning one into a cache. The built libraries are git-ignored —
run the script once after cloning, or Code falls back to MyGo's pinned
revision. The revision to use is kept in `ghostty.version`.

`mygo build` also bundles MyGo's own pinned `libghostty-vt`, which Code does
not use. `scripts/package.sh` builds the app without a disk image, removes
that copy, re-signs and makes the disk image, so a shipped app holds only the
library Code uses:

```sh
scripts/build-libghostty.sh all
scripts/package.sh -platform darwin/universal
```

## Build every platform

Each platform and architecture gets its own app, carrying only its own
`libghostty-vt`, so no download holds two chips' worth. `scripts/build-release.sh`
builds and packages them:

```sh
scripts/build-release.sh                       # all four
scripts/build-release.sh darwin/arm64          # or darwin/amd64, linux/amd64, linux/arm64
```

macOS apps are signed and turned into disk images; Linux apps are the
executable with its `.deb` and `.tar.gz`. On Linux the shared library is
stripped (`internal/elfstrip`) after Zig builds it, since Zig keeps its debug
sections otherwise.

## Install on Linux

The Linux build folder holds `install.sh`, `code.desktop`, the icons and the
archive. To install Code for the current user (no root), run the script from
that folder:

```sh
sh install.sh
```

It installs the app in `~/.local/code.app`, writes
`~/.local/share/applications/code.desktop` (with the app's absolute path and
icon, so Code shows in the applications menu), links `~/.local/bin/code`, and
refreshes the desktop database. `sh install.sh --uninstall` removes it.

On Arch Linux the same folder holds a `PKGBUILD`, so `makepkg -si` installs
Code with pacman into `/opt/code` and `/usr/share/applications`. This app uses
native UI, so its packages need GTK alone, not WebKitGTK.

## Layout

```
main.go                 the app entry point
cmd/cde                 reports agent events to a running Code
internal/config         the Ghostty-format config document and typed settings
internal/model          pane tree, workspaces, tabs, the saved session
internal/agent          the event protocol, session store, socket listener
internal/history        the agents' saved sessions, read from disk
internal/app            the application, windows, views, menus and dialogs
```

The model and config packages know nothing about terminals or widgets, so
they are unit-tested without a window. The views are tested with `ui.NewTester`.

## MyGo docs

MyGo's documentation is checked in under `mygo-docs/`: `ui/` for the native UI
components (indexed by `ui/README.md`), `plugins/terminal.md` for the terminal,
and the top-level guides for menus, windows and architecture.

## Where Code keeps its files

| | |
|---|---|
| Config | `~/.config/code/config` (`$CODE_CONFIG` overrides it) |
| Themes | `~/.config/code/themes` |
| Session | `~/.config/code/session.json` |
| Agent socket | `~/.config/code/agent.<pid>.sock` |

The config file uses Ghostty's syntax, so existing Ghostty themes and options
carry over, and the first launch imports Ghostty's own config.

## Agent status

Code supports **Claude Code**, **Codex**, **OpenCode**, **Pi** and **PiG**. Each
shell Code starts gets Code's shim directory in front of `PATH`, so typing an
agent's name runs it through `cde launch`, which starts the real binary with
Code's hooks or environment added, without touching the agent's own config:

- **Claude** gets Code's hooks through `--settings`, merged with any the user
  passed.
- **Codex** gets them through `-c` overrides, plus the notification settings
  Code reads to tell when it is waiting.
- **OpenCode** gets `OPENCODE_CONFIG_CONTENT` naming Code's plugins (v1 and
  v2).
- **Pi** and **PiG** run no hooks; Code installs its extension into their
  config folders at launch when those exist.

A hook calls `cde hook <agent>`, which reads the agent's JSON and sends an
event over Code's Unix socket. OpenCode's plugins and Pi's and PiG's
extensions call `cde event` directly. The app binary also acts as `cde`, so a
single build serves both.

```sh
cde event working   --agent claude --detail "Read src/main.go"
cde event needsInput --agent claude --detail "Approve this command?"
cde ping
```

The event kinds are `sessionStarted`, `turnStarted`, `working`, `needsInput`,
`turnEnded`, `sessionEnded` and `attention`. Start an agent with
`CODE_AGENTS_DISABLED=1` to run it without Code's hooks for one session.
