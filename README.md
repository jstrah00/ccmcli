<p align="center"><img src="docs/logo.svg" alt="CCM CLI" width="312"></p>

Popup de tmux que lista todas tus sesiones de Claude Code abiertas, con su estado, y te lleva a cualquiera en dos teclas.

## Para qué

Con varias sesiones de Claude Code en paralelo, repartidas en sesiones y windows de tmux, `ccm` responde de un vistazo cuál te espera, cuál terminó y cuál sigue trabajando. `C-g` + número y ya estás ahí.

```
                ▄▀▀ ▄▀▀ █▄ ▄█  ▄▀▀ █   █
                ▀▄▄ ▀▄▄ █ ▀ █  ▀▄▄ █▄▄ █
               6 sesiones · 1 ●  1 ✓  1 ⠼

 homelab
›  1 ✓  :1                Cambios pendientes por pushear
 stow_dotfiles
   2 ●  :1                Sesión de test
 personal
   3 ⠼  :1 ccmcli         Rediseñar el header
 ────────────────────────────────────────────
 arreglá el login · terminó · hace 12m
 j/k mover · ⏎ ir · / filtrar · r renombrar · dd ocultar · p pegar · U restaurar · q salir
```

Agrupa por sesión de tmux. Cada fila tiene número, estado, window (`:2`, más la carpeta si difiere de la sesión, o el nombre que le pongas con `r`) y título. Abajo va el último prompt de la sesión seleccionada.

## Estados

| | Estado |
|---|---|
| `●` rojo | espera tu input (permiso o pregunta) |
| `⠋` amarillo | trabajando |
| `✓` verde | terminó y no lo viste |
| `○` gris | idle / ya lo viste |
| `?` | sin datos todavía (sesión abierta antes de instalar los hooks) |

## Teclas

| Tecla | Acción |
|---|---|
| `j` / `k`, `gg` / `G` | mover |
| `Enter` | ir a esa sesión, window y pane |
| `1`–`9` (y `10`+) | ir directo a esa entrada |
| `/` | filtrar (fuzzy); `Enter` lo fija, `Esc` lo limpia |
| `r` | renombrar la entrada (reemplaza `:2 …`); vacío vuelve al original |
| `dd` | ocultar la entrada (queda en el registro) |
| `p` / `P` | pegarla debajo / arriba: así reordenás |
| `U` | volver a mostrar todas las ocultas |
| `q` / `Esc` | cerrar |

El orden, las ocultas y los nombres se guardan entre aperturas.

## Cómo funciona

- **Estado:** los hooks de Claude Code llaman a `ccm hook <evento>`, que escribe un JSON por pane en `~/.cache/ccm/panes/`. Tarda unos 6 ms, nunca escribe a stdout y siempre sale con 0.
- **Visto:** hooks de tmux llaman a `ccm seen` al cambiar de window o sesión, y lo que estás mirando pasa de `✓` a `○`.
- **Lista:** `tmux list-panes` filtrado por los panes que corren `claude`, cruzado con esos JSON.
- **Títulos:** del transcript de cada sesión, con lectura incremental.
- **Orden y nombres:** `~/.local/state/ccm/order.json`. Respeta `XDG_CACHE_HOME` y `XDG_STATE_HOME`.

## Instalación

Requisitos: tmux, Go ≥ 1.24, Claude Code. Funciona en Linux y macOS.

**1. Binario**

```sh
go install ./cmd/ccm      # desde este repo → ~/go/bin/ccm
```

**2. Hooks de Claude Code:** agregar a `"hooks"` en `~/.claude/settings.json` (se suman a los que ya tengas):

```json
"SessionStart":     [{"hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook start"}]}],
"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook prompt"}]}],
"PreToolUse":       [{"matcher": "AskUserQuestion", "hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook ask"}]}],
"PostToolUse":      [{"hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook tool-done"}]}],
"Notification": [
  {"matcher": "permission_prompt", "hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook permission"}]},
  {"matcher": "idle_prompt",       "hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook idle"}]}
],
"Stop":             [{"hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook stop"}]}],
"SessionEnd":       [{"hooks": [{"type": "command", "command": "$HOME/go/bin/ccm hook end"}]}]
```

**3. tmux:** agregar a `tmux.conf` y recargar (`tmux source-file ~/.config/tmux/tmux.conf`):

```tmux
bind-key -n C-g display-popup -E -h "60%" -w "80%" "$HOME/go/bin/ccm"
set-hook -g session-window-changed 'run-shell -b "$HOME/go/bin/ccm seen"'
set-hook -g client-session-changed 'run-shell -b "$HOME/go/bin/ccm seen"'
```

**4.** Reiniciar las sesiones de Claude Code abiertas para que tomen los hooks.

## Problemas

- **No aparece ninguna sesión:** `tmux list-panes -a -F '#{pane_current_command}'` tiene que mostrar `claude`. Si muestra `node` u otro, `ccm` no las detecta.
- **Una sesión queda en `?`:** todavía no disparó ningún hook. Mandale un prompt.
- **Log de errores de los hooks:** `~/.cache/ccm/log`.

## Desarrollo

```sh
go test ./...
```

Diseño: [`docs/superpowers/specs/2026-09-23-ccm-design.md`](docs/superpowers/specs/2026-09-23-ccm-design.md).
