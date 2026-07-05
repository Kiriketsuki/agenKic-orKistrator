# Vendored addon: godot-xterm

`godot/addons/godot_xterm/` is a native GDExtension providing an in-editor
Terminal control and a bidirectional PTY node, used by T10's raw-terminal
panel mode for a live shell into an agent's tmux session. It is **not
committed to this repository** — prebuilt native binaries do not belong in
source control, and godot-xterm has no stable v1 release, so pinning to an
exact commit is required for reproducibility.

## Pin

| | |
|---|---|
| Repo | https://github.com/lihop/godot-xterm |
| Tag | `v4.0.3` |
| Commit | `e65b9d1d2a5982c721aeb7ddff8e5b9876e53ec6` |
| Minimum Godot | 4.3+ |

A future bump to a newer godot-xterm release must re-verify the `PTY.fork()`
signature, the `terminal_path` auto-wire behavior, and the `PTY.IPCSIGNAL_*`
constants against that release's source — none of these are guaranteed
API-stable pre-1.0.

## Platform support

godot-xterm's PTY is Linux/macOS only. On Windows (or wherever the extension
is absent), the project runs cleanly and falls back to a read-only ANSI
terminal view fed from `BridgeManager` output — see
`godot/scripts/panels/terminal_view.gd`.

## Install

Run the installer from the repo root:

```bash
godot/addons/install_godot_xterm.sh
```

This downloads the pinned release asset (`godot-xterm-v4.0.3.zip`, which
ships prebuilt native libraries — no `scons` build required) and extracts
`addons/godot_xterm/` into place at `godot/addons/godot_xterm/`.

To verify it took effect, open the project in the Godot editor and confirm
`ClassDB.class_exists(&"Terminal")` and `ClassDB.class_exists(&"PTY")` both
report `true` (e.g. from the Remote tab of the debugger, or a throwaway
`print()` in any `_ready()`).

## Runtime safety

All godot-xterm usage in this codebase is behind a capability check —
`ClassDB.class_exists(&"Terminal")` / `ClassDB.class_exists(&"PTY")` plus an
`OS.get_name()` check — and instantiated via `ClassDB.instantiate(&"...")`
rather than a typed `Terminal.new()`/`PTY.new()` reference, so every script
that touches these classes still parses and the project runs cleanly with
the addon absent (falling back to the read-only view, identical to the
Windows path). Do not introduce a typed `Terminal`/`PTY` variable or `as
Terminal` cast anywhere — it would break loading when the addon isn't
installed.
