# ccm: sesiones cerradas recientemente

## Para qué

Volver a una sesión de Claude Code que ya se cerró (a propósito, por error o con
`/clear`) sin salir del popup ni buscar el session id a mano.

## Objetivos

- Una tecla en el popup muestra las últimas sesiones cerradas.
- Encontrar una sesión escribiendo, estilo fzf.
- `Enter` la retoma en tmux y te lleva ahí.

No objetivos: borrar/editar entradas de la lista, retomar fuera de tmux, listar
sesiones que ccm nunca vio (anteriores al feature).

Anti-objetivo: que `ccm hook` deje de ser rápido, escriba a stdout o salga ≠ 0.

## Enfoques

1. **Registro propio al cerrar (elegido).** ccm ya recibe `hook end` y ya detecta
   panes muertos en `Purge`; ahí tiene session id, cwd y ubicación en tmux.
2. Leer `~/.claude/projects` sin registro. Descartado: no sabe la sesión de tmux,
   mezcla subagentes, y no distingue cerradas de abiertas.
3. Retomar en el pane actual. Descartado: puede estar ocupado por otro `claude`.

## Diseño

**Registro.** `~/.local/state/ccm/closed.json` (respeta `XDG_STATE_HOME`), lista
con la más reciente primero. Por entrada: `session_id`, `cwd`, `transcript_path`,
`tmux_session`, `alias`, `status`, `last_prompt`, `closed_at`.

- Se agrega desde `hook end` (antes de borrar el JSON del pane) y desde `Purge`
  (pane muerto sin `end`).
- No se guardan sesiones sin `session_id` o sin ningún prompt.
- Sin duplicados por `session_id` (la nueva reemplaza). Tope: 50.
- `hook start` con un `session_id` que está en la lista lo saca (se retomó, desde
  ccm o a mano).
- `/clear` dispara `end`: la conversación limpiada queda en la lista.
- La sesión de tmux se guarda en el estado del pane en `hook start` (una llamada
  a tmux por sesión), para tenerla también cuando el pane ya murió. Alias y
  status se copian de `order.json` al cerrar, si se puede armar la clave.
- Escritura atómica (`fsutil.WriteFileAtomic`). Archivo corrupto = lista vacía.

**Vista.** `c` en modo normal abre "Cerradas". Arranca con el buscador activo:

```
 Cerradas · 12
 > head▏
   hace 1h   personal        ccmcli         Rediseñar el header
   hace 2d   stow_dotfiles   stow_dotfiles  Statusline: header de uso
 ────────────────────────────────────────────
 arreglá el header · cerrada hace 1h
  escribí para buscar  ↑/↓ mover  ⏎ retomar  esc volver
```

- Columnas: hace cuánto, sesión de tmux, alias o carpeta, título. Título resuelto
  en la TUI con el cache de transcripts existente; si el transcript ya no existe,
  se usa el último prompt.
- Buscador fuzzy (`sahilm/fuzzy`, ya en el repo) sobre sesión de tmux + alias +
  status + carpeta + título + último prompt. Sin query: orden por cierre. Con
  query: orden por score, caracteres que matchean resaltados.
- Teclas: texto filtra; `↑/↓`, `C-p/C-n`, `C-k/C-j` mueven; `Enter` retoma;
  `Esc` limpia la query, y con query vacía vuelve a la lista normal; `C-c` cierra.
  Como todo lo tipeado va al buscador, en esta vista no hay `j/k` ni números.
- Lista vacía: "todavía no hay sesiones cerradas".

**Retomar.** `tmux new-window` en `tmux_session` (si ya no existe o está vacío, en
la sesión actual), con `-c cwd`, corriendo `claude --resume <session_id>`; después
`switch-client` + `select-window` y el popup se cierra. Si `cwd` no existe, error
en la línea de estado y no se abre nada. Alias y status guardados se reaplican a
la entrada nueva.

## Queda para quien lo implemente

Nombres de paquete/tipos, cómo compartir el render de filas con la vista actual,
formato exacto de "hace X", tests (el repo testea todo con `tmux.Fake` y stores
en `t.TempDir()`).

README: agregar la tecla `c` y una sección corta.
