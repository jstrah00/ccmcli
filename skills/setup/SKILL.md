---
name: setup
description: Install and configure ccm, the tmux popup for Claude Code sessions. Use when the user asks to install, set up, configure, update, fix or uninstall ccm / ccmcli, its C-g popup, or its desktop notifications.
---

# Set up ccm

ccm is a tmux popup (opened with `C-g`) that lists every Claude Code session with its live status, plus desktop notifications that jump to the right tmux pane when clicked. It needs Claude Code hooks in `settings.json` and a few lines in `tmux.conf`; `ccm setup` writes both, `ccm doctor` checks them.

Run the steps in order. Ask before installing packages or editing files outside what `ccm setup` touches. Show the user what you're about to change.

## 1. Prerequisites

Run `uname -s`, `tmux -V` and `go version`.

- **tmux 3.2 or newer** (ccm uses `display-popup`). ccm only lists sessions running inside tmux.
- **Go 1.24 or newer** to build ccm.
- **macOS:** `terminal-notifier` makes clicking a notification jump to the pane. Without it ccm falls back to `osascript`: the banner shows, the click does nothing useful. Check with `command -v terminal-notifier || ls /opt/homebrew/bin/terminal-notifier`.
- **Linux:** `notify-send` (package `libnotify-bin` on Debian/Ubuntu, `libnotify` on Fedora/Arch).

If something is missing, offer the install command (`brew install tmux go terminal-notifier` on macOS; the distro's package manager on Linux) and run it only after the user agrees.

## 2. Install the binary

```sh
go install github.com/jstrah00/ccmcli/cmd/ccm@latest
```

The binary lands in `$(go env GOBIN)` or, if that's empty, `$(go env GOPATH)/bin`. Use that absolute path for the rest of the steps (call it `CCM`), since it may not be on `PATH`.

## 3. Hooks and the `C-g` shortcut

Check whether `C-g` is already taken: `tmux list-keys -T root C-g 2>/dev/null` and `grep -n 'C-g' ~/.config/tmux/tmux.conf ~/.tmux.conf 2>/dev/null`. If it's bound to something other than ccm, tell the user and ask which key they want instead (pass it with `--key`, e.g. `--key M-g`). Otherwise use the default `C-g`.

Preview, show the output to the user, then apply:

```sh
"$CCM" setup --dry-run
"$CCM" setup
```

`ccm setup`:
- adds 8 hooks to `~/.claude/settings.json` (or `$CLAUDE_CONFIG_DIR/settings.json`) pointing at this binary, keeps every other hook and setting, and saves a backup as `settings.json.bak-ccm`;
- adds a block between `# >>> ccm >>>` and `# <<< ccm <<<` to `~/.config/tmux/tmux.conf` (or `~/.tmux.conf`) that binds `C-g` (without the tmux prefix) to the popup and marks sessions as seen when you switch windows; it reloads tmux if it's running;
- is safe to run again (after updating ccm, too).

If it reports ccm lines outside the block (from an older manual install), offer to remove those lines.

## 4. Check and test notifications

```sh
"$CCM" doctor --notify
```

Fix every `✘`. For `!` lines:
- **duplicate notifications:** other hooks (e.g. a custom `notify.sh`) also notify on Stop/Notification. Offer to remove them from `settings.json` so the user doesn't get two alerts; don't remove them without asking.
- **terminal app (macOS):** ccm couldn't tell which terminal to bring to the front. Ask which terminal they use and set `"terminal_bundle"` (e.g. `com.googlecode.iterm2`, `com.apple.Terminal`, `com.mitchellh.ghostty`, `net.kovidgoyal.kitty`, `com.github.wez.wezterm`, `org.alacritty`) in `~/.config/ccm/config.json`:
  ```json
  {"notify": {"terminal_bundle": "com.googlecode.iterm2"}}
  ```

Ask the user whether the test banner showed up. On macOS, the first one may need permission: **System Settings → Notifications → terminal-notifier → Allow notifications**, and "Alerts" style if they want it to stay on screen. Then have them click it: it should bring the terminal to the front and switch tmux to the pane where they ran `doctor`.

## 5. Finish

Tell the user:
- **Restart the Claude Code sessions that are already open.** Hooks load when a session starts; sessions opened before setup show `?` until then.
- Inside tmux, **press `C-g`** (or the key they chose) to open ccm. `Enter` or a number jumps to a session, `c` shows closed sessions, `q` closes.
- Notifications fire when a session needs input, or when a turn that took 10 s or more finishes while they're looking elsewhere. Tune them in `~/.config/ccm/config.json`:
  ```json
  {"notify": {"banner": true, "sound": true, "min_turn_seconds": 10}}
  ```

## Uninstall

Remove the block between `# >>> ccm >>>` and `# <<< ccm <<<` from the tmux config, delete the hooks whose command ends in `ccm hook <event>` from `settings.json`, then `rm "$CCM"` and optionally `~/.cache/ccm`, `~/.local/state/ccm` and `~/.config/ccm`.
