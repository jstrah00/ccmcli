# ccm — selector de sesiones de Claude Code en tmux

Fecha: 2026-09-23

## Objetivo

Popup de tmux (`C-g`) que lista todos los panes donde corre Claude Code (CC),
agrupados por sesión de tmux, con el estado de cada uno. Permite saltar a
cualquiera con `Enter` o con su número, filtrar, y reordenar/ocultar entradas
con teclas estilo vim. Tiene que funcionar igual en Fedora (Hyprland) y en la
Mac.

Éxito: con varias sesiones de CC abiertas, en un vistazo se ve cuál espera input,
cuál terminó y cuál trabaja, y se llega a ella en 2 teclas (`C-g` + número).

## Arquitectura

Tres piezas:

1. **Binario `ccm`** (Go + Bubble Tea, este repo). Subcomandos:
   - `ccm` — la TUI.
   - `ccm hook <evento>` — lo invocan los hooks de CC; actualiza el estado del pane.
   - `ccm seen` — lo invocan hooks de tmux al cambiar de window/sesión.
2. **Hooks de CC** en `~/.claude/settings.json` (dotfiles, ambas branches) que
   llaman a `$HOME/go/bin/ccm hook <evento>`. `notify.sh` sigue igual (sonido,
   banner, `@claude_state` del status bar); `ccm` no depende de él.
3. **tmux.conf** (dotfiles, ambas branches):
   ```
   bind-key -n C-g display-popup -E -h "60%" -w "80%" "$HOME/go/bin/ccm"
   set-hook -g session-window-changed 'run-shell -b "$HOME/go/bin/ccm seen"'
   set-hook -g client-session-changed 'run-shell -b "$HOME/go/bin/ccm seen"'
   ```

## Estado

### Archivo por pane

`$XDG_CACHE_HOME/ccm/panes/<pane_id>.json` (default `~/.cache/ccm/...`; el `%`
del pane id se reemplaza por nada: `%41` → `41.json`):

```json
{
  "pane_id": "%41",
  "session_id": "231f926e-...",
  "cwd": "/home/zepik/personal/ccmcli",
  "transcript_path": "/home/zepik/.claude/projects/.../231f926e-....jsonl",
  "state": "working",
  "since": "2026-09-23T22:48:00Z"
}
```

- `pane_id` sale de `$TMUX_PANE` (heredado por el hook). Sin `$TMUX_PANE` el hook
  no hace nada.
- `session_id`, `cwd`, `transcript_path` salen del JSON que CC manda por stdin.
- `since` solo cambia cuando cambia `state`.
- Escrituras atómicas (tmp + rename).

### Estados e indicadores

| Estado    | Indicador                 | Significado                      |
|-----------|---------------------------|----------------------------------|
| `waiting` | `●` rojo `#f7768e`        | espera input (permiso/pregunta)  |
| `working` | spinner amarillo `#e0af68`| trabajando                       |
| `done`    | `✓` verde `#9ece6a`       | terminó y no lo viste            |
| `idle`    | `○` gris `#565f89`        | terminó y ya lo viste / sin uso  |
| `unknown` | `?` gris `#3b4261`        | pane con CC sin archivo de estado|

### Transiciones

| Evento de CC (hook)                               | `ccm hook` arg | Nuevo estado |
|---------------------------------------------------|----------------|--------------|
| `SessionStart`                                    | `start`        | `idle`       |
| `UserPromptSubmit`                                | `prompt`       | `working`    |
| `PreToolUse` con matcher `AskUserQuestion`        | `ask`          | `waiting`    |
| `Notification` con matcher `permission_prompt`    | `permission`   | `waiting`    |
| `PostToolUse` (cualquier tool)                    | `tool-done`    | `working`    |
| `Notification` con matcher `idle_prompt`          | `idle`         | si estaba `working`/`waiting`: igual que `stop` (destraba interrupciones con Esc, que no disparan `Stop`) |
| `Stop`                                            | `stop`         | `done`, o `idle` si la window del pane está activa en una sesión attached |
| `SessionEnd`                                      | `end`          | borra el archivo |

`ccm seen`: para cada window activa de una sesión attached, pasa sus panes de
`done` a `idle`.

`UserPromptSubmit` inyectado por el sistema (prompt que empieza con
`<task-notification>` o `<system-reminder>`) no cambia el estado (misma regla que
`notify.sh`).

### Reglas de `ccm hook`

- Nunca escribe a stdout (CC lo interpreta como control). Errores a
  `$XDG_CACHE_HOME/ccm/log`.
- Siempre exit 0.
- Objetivo < 50 ms: no lee transcripts ni llama a tmux salvo en `stop` (una
  llamada a `display-message` para saber si la window está activa).

## Descubrimiento de entradas

1. `tmux list-panes -a -F` con `pane_id`, `session_name`, `window_index`,
   `pane_index`, `pane_current_command`, `pane_current_path`.
2. Entrada = pane con `pane_current_command == "claude"`.
3. Se une con su archivo de estado por `pane_id`. Sin archivo → `unknown`, cwd =
   `pane_current_path`.
4. Al abrir la TUI se borran archivos de estado de panes que ya no existen.

Binario tmux: `exec.LookPath("tmux")`, fallback `/usr/bin/tmux`,
`/opt/homebrew/bin/tmux`, `/usr/local/bin/tmux`.

## Títulos

Del transcript (`transcript_path`), entradas JSONL:

- `{"type":"agent-name","agentName":...}` — nombre de `/rename`.
- `{"type":"ai-title","aiTitle":...}` — título autogenerado; CC lo reescribe
  durante la sesión.

Reglas:

- **Título en la fila**: último `agent-name` si existe; si no, **primer**
  `ai-title`; si no, vacío.
- **Título en la zona de detalle**: **último** `ai-title`.
- Se relee un transcript solo si cambió su tamaño o mtime.
- El primer `ai-title` se cachea por `session_id` en
  `$XDG_CACHE_HOME/ccm/titles/<session_id>` para no reescanear el archivo entero.
- Transcript faltante o ilegible → título vacío, sin error visible.

## Orden y ocultas

Persistido en `$XDG_STATE_HOME/ccm/order.json` (default `~/.local/state/ccm/`):

```json
{
  "order":  ["elpia-api:1.1|/home/zepik/personal/elpia/elpia-api", "..."],
  "hidden": ["temp:2.1|/home/zepik/Downloads"]
}
```

- **Clave de entrada**: `<sesión>:<window>.<pane>|<cwd>`. Sobrevive reinicios
  de tmux (resurrect) y `/clear` de CC. Si la sesión se renombra o las windows
  se renumeran, la entrada pierde su lugar y va al final (aceptado).
- Entradas vivas sin clave en `order` → al final, en el orden de
  `list-panes -a`.
- Claves en `order` sin entrada viva se conservan (vuelven a su lugar si la
  entrada reaparece).
- `dd`: saca la clave de `order`, la agrega a `hidden` y la pone en el registro (un solo slot: un segundo `dd` reemplaza el registro; la anterior queda oculta).
- `p` / `P`: pega la clave del registro debajo / arriba de la entrada del cursor
  y la saca de `hidden`. Registro vacío → no hace nada.
- Cerrar sin pegar → la entrada queda en `hidden`.
- `U`: todas las `hidden` pasan al final de `order`.
- Escritura atómica tras cada cambio.

## TUI

### Layout

```
 ccm                                        / filtro
 elpia-api
  1 ●  elpia-api      fix-auth-refresh
  2 ⠋  elpia-api      add-rate-limit
 stow_dotfiles
  3 ✓  stow_dotfiles  port-mac-changes-to-fedora
 personal
  4 ○  ccmcli         ccm-design
 ──────────────────────────────────────────────
 Opción para mostrar ocultas · waiting hace 3m
 j/k mover · ⏎ ir · / filtrar · dd ocultar · p pegar · U restaurar · q salir
```

- Fila: `número · indicador · ubicación · título`. Ubicación = `:<window>`
  + nombre de la window si no es el automático (el comando) + carpeta si
  difiere del nombre de la sesión. El indicador es siempre visible.
  (2026-09-24: antes era la carpeta, que casi siempre repetía el encabezado.)
- **Numeración** global 1..N según `order` (sin contar ocultas).
- **Encabezados** de sesión tmux, no seleccionables. Se recorre la lista
  ordenada y se emite un encabezado cada vez que cambia la sesión respecto de
  la fila anterior (una sesión puede aparecer más de una vez si sus entradas no
  son contiguas en el orden).
- **Zona de detalle** (entrada bajo el cursor): último prompt del usuario
  (lo guarda `ccm hook prompt` en `last_prompt`, en una línea, máx. 300
  runas; sin él, último `ai-title`) · estado ·
  tiempo desde `since` (`hace 12s`, `hace 3m`, `hace 2h`). `unknown` no muestra
  tiempo.
- Barra de ayuda fija abajo.
- Sin entradas: "sin sesiones de Claude Code" + `q` para salir.

### Teclas — modo normal

| Tecla      | Acción |
|------------|--------|
| `j` / `k` (y flechas) | bajar / subir; saltean encabezados |
| `gg` / `G` | primera / última entrada |
| `Enter`    | ir a la entrada y cerrar |
| `1`–`9`    | ir a la entrada N. Si hay ≥10 entradas y el dígito puede ser prefijo de otra, espera 300 ms por un segundo dígito |
| `dd`       | ocultar entrada (al registro) |
| `p` / `P`  | pegar del registro debajo / arriba |
| `U`        | restaurar todas las ocultas |
| `/`        | entrar a modo filtro |
| `q` / `Esc`| cerrar (en normal con filtro activo, `Esc` limpia el filtro) |

Secuencias de 2 teclas (`dd`, `gg`) se cancelan con cualquier otra tecla o a
los 1 s.

### Teclas — modo filtro

- Texto → filtro fuzzy (`sahilm/fuzzy`) sobre `sesión carpeta título`.
  Encabezados solo aparecen si tienen alguna entrada que matchea.
- `Enter` → vuelve a normal manteniendo el filtro.
- `Esc` → limpia filtro y vuelve a normal.
- `Ctrl-n` / `Ctrl-p` → mover cursor sin salir del filtro.
- Numeración y saltos por número usan siempre el número global, no la posición
  filtrada.

### Ir a una entrada

`tmux switch-client -t <sesión>` + `select-window -t <sesión>:<window>` +
`select-pane -t <pane_id>`, después sale. Esto dispara `ccm seen` vía hooks de
tmux.

### Refresco

Cada 1 s: re-lista panes, relee archivos de estado y títulos cambiados. El
cursor se mantiene sobre la misma clave; si desaparece, pasa a la siguiente
(o a la anterior si era la última). El spinner anima a ~10 fps.

## Errores

- `ccm` fuera de tmux (`$TMUX` vacío) → mensaje a stderr, exit 1.
- tmux no encontrado → mensaje, exit 1.
- `order.json` corrupto → se ignora (orden por defecto) y se loguea; se
  reescribe al siguiente cambio.
- Archivo de estado corrupto → se trata como `unknown`.

## Mac

- Mismo código; rutas vía `XDG_*` con defaults `~/.cache` y `~/.local/state`.
- Instalación con `go install` en cada máquina → `~/go/bin/ccm`.
- Los cambios de `settings.json` y `tmux.conf` se portan a mano a la branch
  `master/mac` de dotfiles.

## Estructura del código (orientativa)

```
cmd/ccm/main.go        dispatch de subcomandos
internal/state/        archivo por pane, transiciones, seen
internal/hook/         parseo de stdin de CC, evento → transición
internal/tmux/         interfaz Tmux + implementación exec + fake
internal/transcript/   títulos desde JSONL
internal/order/        order.json, dd/p/P/U, merge con entradas vivas
internal/tui/          modelo Bubble Tea, vista, teclas
```

## Tests

TDD. Unit:

- Transiciones de estado por evento, incluidos `stop` en window activa y
  prompts inyectados.
- `seen` sobre windows activas.
- Parser de títulos: `agent-name` > primer `ai-title`; último `ai-title` para
  detalle; líneas inválidas ignoradas.
- `order`: entradas nuevas al final, `dd` + `p`/`P`, cerrar sin pegar deja
  oculta, `U`, claves ausentes conservadas.
- Agrupación con encabezados (sesión repetida si no contigua).
- Modelo TUI con mensajes de tecla simulados: `j`/`k` saltean encabezados,
  `dd`, números de 1 y 2 dígitos, filtro.
- tmux detrás de interfaz, con fake.

Manual (ambas máquinas): checklist de abrir popup, estados reales en 2-3
sesiones de CC, saltos, reordenar, ocultar/restaurar, reinicio de tmux.

## Fuera de alcance

- Preview del contenido del pane.
- Crear/matar sesiones de CC desde `ccm`.
- Notificaciones (ya las hace `notify.sh`).
