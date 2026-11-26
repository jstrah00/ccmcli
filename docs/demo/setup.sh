#!/bin/sh
# Builds a fake environment for docs/demo/demo.tape: its own tmux server
# (socket "ccmdemo") whose panes run a process named "claude", plus fake ccm
# state. Nothing touches your real tmux or ~/.cache.
#   docs/demo/setup.sh && vhs docs/demo/demo.tape
set -eu

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
D=${CCM_DEMO_DIR:-${TMPDIR:-/tmp}/ccm-demo}
T="tmux -L ccmdemo -f /dev/null"

$T kill-server 2>/dev/null || true
rm -rf "$D"
mkdir -p "$D/bin" "$D/cache/ccm/panes" "$D/state/ccm" "$D/claude/sessions" "$D/transcripts"
go build -o "$D/bin/ccm" "$ROOT/cmd/ccm"
mkdir -p "$D/fake"
printf 'module fake\n\ngo 1.24\n' > "$D/fake/go.mod"
printf 'package main\n\nimport "time"\n\nfunc main() { time.Sleep(1 << 62) }\n' > "$D/fake/main.go"
(cd "$D/fake" && go build -o "$D/bin/claude" .)

for p in api worker web docs infra; do mkdir -p "$D/work/$p"; done

pane() { # session cwd
	banner="printf '\\n  \\033[38;2;217;119;87m✻\\033[0m claude · $2\\n\\n  > '"
	cmd="sh -c \"$banner; exec $D/bin/claude\""
	if $T has-session -t "$1" 2>/dev/null; then
		$T new-window -d -t "$1" -c "$D/work/$2" "$cmd"
	else
		$T new-session -d -s "$1" -c "$D/work/$2" -x 160 -y 45 "$cmd"
	fi
}
pane backend  api
pane backend  worker
pane frontend web
pane frontend docs
pane ops      infra

$T set -g status-style "bg=#1f1f28,fg=#9a9a9a"
$T set -g mouse off
$T set -g status-right ""
$T bind-key -n C-g display-popup -E -h 70% -w 85% \
	"env XDG_CACHE_HOME=$D/cache XDG_STATE_HOME=$D/state CLAUDE_CONFIG_DIR=$D/claude $D/bin/ccm"

ago() { date -u -r $(($(date +%s) - $1)) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$(($(date +%s) - $1))" +%Y-%m-%dT%H:%M:%SZ; }

i=0
$T list-panes -a -F '#{pane_id} #{session_name} #{b:pane_current_path}' | while read -r id s w; do
	i=$((i + 1))
	sid="0000000$i-demo-4000-8000-00000000000$i"
	case "$s/$w" in
	backend/api)   st=waiting; t=40;   title="Refresh token rotation";   prompt="rotate refresh tokens on every login and add a test for reuse detection" ;;
	backend/worker) st=working; t=95;   title="Retry policy for workers"; prompt="add exponential backoff to the email worker" ;;
	frontend/web)  st=done;    t=720;  title="Dark mode toggle";         prompt="add a dark mode toggle to the settings page" ;;
	frontend/docs) st=idle;    t=3600; title="Getting started guide";    prompt="rewrite the quickstart, keep it under one page" ;;
	ops/infra)     st=working; t=20;   title="Terraform plan for staging"; prompt="plan the staging db upgrade to postgres 17" ;;
	esac
	tr="$D/transcripts/$sid.jsonl"
	printf '{"type":"ai-title","aiTitle":"%s"}\n' "$title" > "$tr"
	printf '{"pane_id":"%s","session_id":"%s","cwd":"%s","transcript_path":"%s","state":"%s","since":"%s","last_prompt":"%s","tmux_session":"%s"}\n' \
		"$id" "$sid" "$D/work/$w" "$tr" "$st" "$(ago $t)" "$prompt" "$s" \
		> "$D/cache/ccm/panes/${id#%}.json"
	if [ "$s/$w" = "ops/infra" ]; then
		printf '{"sessionId":"%s","bridgeSessionId":"demo"}\n' "$sid" > "$D/claude/sessions/1.json"
	fi
done

key() { $T list-panes -t "$1" -F "#{session_name}:#{window_index}.#{pane_index}|#{pane_current_path}"; }
printf '{"order":[],"hidden":[],"statuses":{"%s":"in review"}}\n' "$(key frontend:0)" > "$D/state/ccm/order.json"

closed() { # id cwd tmux_session prompt agoSecs title
	printf '{"type":"ai-title","aiTitle":"%s"}\n' "$6" > "$D/transcripts/$1.jsonl"
	printf '{"session_id":"%s","cwd":"%s","transcript_path":"%s","tmux_session":"%s","last_prompt":"%s","closed_at":"%s"}' \
		"$1" "$D/work/$2" "$D/transcripts/$1.jsonl" "$3" "$4" "$(ago "$5")"
}
{
	printf '['
	closed c1 api backend "why does the login test flake on CI?" 1800 "Flaky login test"; printf ','
	closed c2 web frontend "migrate the forms to react-hook-form" 7200 "Forms migration"; printf ','
	closed c3 infra ops "set up log retention for the staging cluster" 86400 "Log retention"; printf ','
	closed c4 api backend "add pagination to GET /orders" 172800 "Orders pagination"
	printf ']\n'
} > "$D/state/ccm/closed.json"

echo "ready: tmux -L ccmdemo attach"
