<p align="center"><img src="docs/logo.svg" alt="CCM CLI" width="312"></p>

<p align="center">
  <b>A tmux popup that lists every open Claude Code session with its status, and takes you to any of them in two keystrokes.</b>
</p>

<p align="center">
  <a href="https://github.com/jstrah00/ccmcli/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/jstrah00/ccmcli/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go 1.24+" src="https://img.shields.io/badge/go-1.24%2B-00ADD8?logo=go&logoColor=white">
  <img alt="tmux" src="https://img.shields.io/badge/tmux-popup-1BB91F?logo=tmux&logoColor=white">
  <img alt="Linux and macOS" src="https://img.shields.io/badge/linux%20%7C%20macOS-555">
  <img alt="Claude Code" src="https://img.shields.io/badge/Claude%20Code-hooks-d97757">
  <img alt="MIT license" src="https://img.shields.io/badge/license-MIT-blue">
</p>

## Why

When you run several Claude Code sessions in parallel, spread across tmux sessions and windows, `ccm` tells you at a glance which one is waiting for you, which one finished and which one is still working. Press **`Ctrl+g`** anywhere in tmux, then a number, and you're there. When a session needs you while you're looking elsewhere, a desktop notification tells you, and clicking it takes you to that pane.

<p align="center"><img src="docs/demo/demo.gif" alt="ccm demo" width="900"></p>

- **Live status** for every session: waiting for input, working, finished and unseen, idle.
- **Jump by number** to the exact session, window and pane.
- **Your own order, names and status notes**, kept between openings.
- **Closed sessions** with fuzzy search: press `Enter` to resume one where it was, with its name and status.
- **Notifications** when a session needs input or finishes a long turn; click one to jump to its pane (macOS and Linux).
- **Remote-control** sessions marked with a breathing `◉`.
- **Cheap:** the hook takes 3–7 ms, never writes to stdout and never breaks Claude Code.

Rows are grouped by tmux session. Each row shows a number, the state, the window (`:2`, plus the folder if it differs from the session name, or the name you give it with `r`), a status note (free text set with `s`; the column only shows up if some entry has one) and the conversation title. The bottom line shows the last prompt of the selected session.

## States

| | State |
|---|---|
| red `●` | waiting for your input (permission or question) |
| yellow `⠋` | working |
| green `✔` | finished and you haven't looked at it yet |
| gray `○` | idle / already seen |
| `?` | no data yet (session opened before the hooks were installed) |

Next to the state, a breathing blue `◉` marks sessions with remote-control on (the column only shows up if there is one). It comes from `bridgeSessionId` in `~/.claude/sessions/<pid>.json`, which Claude Code doesn't pass to hooks.

## Keys

| Key | Action |
|---|---|
| `j` / `k`, `gg` / `G` | move |
| `Enter` | go to that session, window and pane |
| `1`–`9` (and `10`+) | jump straight to that entry |
| `/` | filter (fuzzy); `Enter` keeps it, `Esc` clears it |
| `r` | rename the entry (replaces `:2 …`); empty restores the original |
| `dr` | rename starting from an empty input |
| `s` | set/edit the entry's status note; empty removes it |
| `ds` | remove the status note |
| `dd` | hide the entry (it stays in the registry) |
| `p` / `P` | paste it below / above: that's how you reorder |
| `U` | show all hidden entries again |
| `c` | closed sessions; type to search (fuzzy), `Enter` resumes it in a new window, `Esc` goes back |
| `q` / `Esc` | close |

Order, hidden entries and names persist between openings.

## Install

### Requirements

- **tmux 3.2+** (for `display-popup`). ccm only sees Claude Code sessions running inside tmux.
- **Go 1.24+** to build it.
- **Claude Code.**
- **Notifications (optional):** on macOS, [`terminal-notifier`](https://github.com/julienXX/terminal-notifier) (`brew install terminal-notifier`) so clicking a notification jumps to the pane; without it you still get the banner via `osascript`, but the click does nothing. On Linux, `notify-send` (`libnotify-bin` / `libnotify`).

Works on macOS and Linux.

### Quick setup with Claude Code

ccm ships as a Claude Code plugin with a setup skill. In Claude Code:

```
/plugin install ccm --marketplace jstrah00/ccmcli
/ccm:setup
```

(On older Claude Code: `/plugin marketplace add jstrah00/ccmcli`, then `/plugin install ccm@ccmcli`.)

The skill checks the requirements, installs the binary, wires the hooks and the `Ctrl+g` shortcut, sends a test notification and tells you what's left. It asks before installing anything or touching your files.

### Manual setup

```sh
go install github.com/jstrah00/ccmcli/cmd/ccm@latest
~/go/bin/ccm setup --dry-run   # see what will change
~/go/bin/ccm setup
~/go/bin/ccm doctor --notify   # check everything and send a test notification
```

Then restart your open Claude Code sessions (hooks load when a session starts) and press **`Ctrl+g`** inside tmux.

`ccm setup` is safe to run again, e.g. after updating ccm:

- **Claude Code hooks:** adds 8 hooks to `~/.claude/settings.json` (or `$CLAUDE_CONFIG_DIR/settings.json`) pointing at the installed binary. Other hooks and settings are left alone, key order included, and the previous file is saved as `settings.json.bak-ccm`.
- **tmux:** adds a block to `~/.config/tmux/tmux.conf` (or `~/.tmux.conf`) and reloads tmux:

  ```tmux
  # >>> ccm >>>
  bind-key -n C-g display-popup -E -h "60%" -w "80%" "/Users/you/go/bin/ccm"
  set-hook -g session-window-changed[42] 'run-shell -b "/Users/you/go/bin/ccm seen"'
  set-hook -g client-session-changed[42] 'run-shell -b "/Users/you/go/bin/ccm seen"'
  # <<< ccm <<<
  ```

  `Ctrl+g` works without the tmux prefix. To use another key: `ccm setup --key M-g`. Other flags: `--settings PATH`, `--tmux-conf PATH`.

<details>
<summary>What the hooks look like in <code>settings.json</code></summary>

```json
"SessionStart":     [{"hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook start"}]}],
"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook prompt"}]}],
"PreToolUse":       [{"matcher": "AskUserQuestion", "hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook ask"}]}],
"PostToolUse":      [{"hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook tool-done"}]}],
"Notification": [
  {"matcher": "permission_prompt", "hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook permission"}]},
  {"matcher": "idle_prompt",       "hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook idle"}]}
],
"Stop":             [{"hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook stop"}]}],
"SessionEnd":       [{"hooks": [{"type": "command", "command": "/Users/you/go/bin/ccm hook end"}]}]
```
</details>

## Notifications

| When | Notification | Sound (macOS) |
|---|---|---|
| Claude asks for permission or asks you a question | **Needs your input** + the request | Sosumi |
| A turn that took 10 s or more finishes | **Finished** + Claude's last message | Glass |

- **No notification if you're already looking at that pane** (its window is active in an attached session), and none for quick turns.
- **Title:** `session:window/folder`. A new notification from the same session replaces the previous one.
- **Click:** on macOS (with `terminal-notifier`) it brings your terminal to the front and runs `tmux switch-client` to that pane. On Linux, the notification has a *Go to session* action that switches tmux to the pane; raising the terminal window is up to your desktop.
- **Which terminal comes to the front (macOS):** detected from the terminal that started the tmux server (`__CFBundleIdentifier`, or `TERM_PROGRAM` as a fallback), so Ghostty, iTerm2, Terminal, WezTerm, kitty, Alacritty, Warp and VS Code work without config. Set `terminal_bundle` if it picks the wrong one.
- **Fast:** the hook hands the notification to a detached `ccm notify` process and returns right away.

Optional config in `~/.config/ccm/config.json` (or `$XDG_CONFIG_HOME/ccm/config.json`), shown with the defaults:

```json
{
  "notify": {
    "banner": true,
    "sound": true,
    "min_turn_seconds": 10,
    "terminal_bundle": ""
  }
}
```

First notification on macOS: allow it in **System Settings → Notifications → terminal-notifier**. Pick the "Alerts" style if you want it to stay on screen until you click it.

## How it works

- **State:** Claude Code hooks call `ccm hook <event>`, which writes one JSON file per pane in `~/.cache/ccm/panes/`. It never writes to stdout and always exits 0. If the session moved to the Claude Code daemon (it runs without `TMUX_PANE`), the hook finds its pane through the pty host; sessions outside tmux are ignored.
- **Seen:** tmux hooks call `ccm seen` when you switch window or session, and whatever you're looking at goes from `✔` to `○`.
- **List:** `tmux list-panes`, filtered to the panes running Claude Code (`claude`, or a version like `2.1.280` with the native binary), joined with those JSON files.
- **Closed:** `ccm hook end` and the cleanup of dead panes record them in `~/.local/state/ccm/closed.json` (last 50, only those that got at least one prompt). Resuming opens a window in the tmux session it was in (or the current one if that's gone) and types `claude --resume <id>` into a shell, so the shell stays when you quit Claude. It gets back its name and status, and leaves the list once it starts.
- **Titles:** from each session's transcript, read incrementally.
- **Order, names and status notes:** `~/.local/state/ccm/order.json`. `XDG_CACHE_HOME`, `XDG_STATE_HOME` and `XDG_CONFIG_HOME` are respected.
- **Notifications:** `stop`, `permission` and `ask` decide whether to notify (see [Notifications](#notifications)) and start `ccm notify` detached.
- **Remote-control:** `~/.claude/sessions/*.json` (or `$CLAUDE_CONFIG_DIR/sessions`).

## Commands

| | |
|---|---|
| `ccm` | opens the TUI (meant for `display-popup`) |
| `ccm hook <event>` | called by Claude Code hooks: `start`, `prompt`, `ask`, `permission`, `tool-done`, `idle`, `stop`, `end` |
| `ccm seen` | called by tmux hooks when you switch window or session |
| `ccm setup [--dry-run] [--key K] [--settings P] [--tmux-conf P]` | installs the Claude Code hooks and the tmux block |
| `ccm doctor [--notify]` | checks the install; `--notify` sends a test notification |
| `ccm notify [--test]` | sends a notification (run by the hook; `--test` to try it) |

## Troubleshooting

Start with **`ccm doctor --notify`**: it checks tmux, the hooks, the tmux block, the notifier and duplicate notification hooks, and tells you how to fix each one.

- **No sessions show up:** `tmux list-panes -a -F '#{pane_current_command}'` must show `claude` or a version number (`2.1.280`). If it shows `node` or something else, `ccm` won't detect them.
- **`ccm: must run inside tmux`:** the TUI needs `$TMUX`; open it with `Ctrl+g` inside tmux.
- **`Ctrl+g` does nothing:** reload tmux (`tmux source-file ~/.config/tmux/tmux.conf`) and check nothing later in your config rebinds `C-g` (`tmux list-keys -T root C-g`).
- **No notifications:** run `ccm notify --test`. On macOS, allow `terminal-notifier` in System Settings → Notifications. Quick turns (under `min_turn_seconds`) and the pane you're looking at don't notify, on purpose.
- **Double notifications:** another hook (your own script, another plugin) also notifies on `Stop`/`Notification`; `ccm doctor` lists them.
- **A session stays at `?`:** no hook has fired for it yet. Send it a prompt.
- **Hook error log:** `~/.cache/ccm/log`.

## Uninstall

Remove the `# >>> ccm >>>` block from your tmux config and the hooks ending in `ccm hook <event>` from `settings.json`, then delete the binary and, if you want, `~/.cache/ccm`, `~/.local/state/ccm` and `~/.config/ccm`. In Claude Code, `/plugin uninstall ccm`.

## Development

```sh
git clone https://github.com/jstrah00/ccmcli && cd ccmcli
go test ./...
go install ./cmd/ccm
```

The demo GIF is reproducible: `docs/demo/setup.sh && vhs docs/demo/demo.tape` (uses its own tmux server and fake sessions).

## License

[MIT](LICENSE)
