# todo-board

A terminal Kanban board. Manage boards, cards, priorities, and dependencies from the command line.

## Install

Requires Go 1.22+.

```sh
go install github.com/gingerbreadtrev/todo-board@latest
```

Then run:

```sh
todo-board
```

Data is stored as a SQLite database at `$XDG_DATA_HOME/todo-board/todo-board.db` (typically `~/.local/share/todo-board/todo-board.db`).

## Views

**Main view** — two-panel layout: board list on the left, card list on the right. Cards are grouped by column (phase) and show inline indicators for priority, parent/child relationships, and blockers.

**Kanban view** — one column per phase, all cards visible simultaneously. Toggle with `V`.

**Card detail view** — full-screen view showing all card fields, a scrollable description, and four relationship tables (Parents, Children, Blocking, Blocked by). Open with `Enter`.

**Archive view** — archived cards for the active board. Restore or permanently delete. Toggle with `D`.

## Keybindings

Keys work without any mode switching (Helix-style — no insert mode).

### Board panel

| Key | Action |
|-----|--------|
| `↑` / `↓` / `k` / `j` | Navigate boards |
| `Enter` | Select board |
| `n` | New board |
| `r` | Rename board |
| `d` | Delete board |

### Card panel

| Key | Action |
|-----|--------|
| `↑` / `↓` / `k` / `j` | Navigate cards |
| `Enter` | Open card detail |
| `n` | New card |
| `r` | Rename card |
| `e` | Edit description |
| `c` | Toggle done |
| `p` | Set priority |
| `←` / `→` / `h` / `l` | Move card to prev/next column |
| `d` | Archive card |
| `D` | Toggle archive view |
| `b` | Manage blockers |
| `s` | Manage children |
| `/` | Search cards |
| `V` | Toggle kanban view |
| `Tab` | Switch panel focus |
| `?` | Help overlay |

### Card detail view

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle between areas (properties / description / relations) |
| `↑` / `↓` / `k` / `j` | Navigate rows or scroll description |
| `←` / `→` / `h` / `l` | Switch relation table (in relations area) |
| `Enter` | Edit field / open related card / add relation |
| `x` | Remove focused relationship |
| `d` | Archive card |
| `Esc` | Back |

### Kanban view

| Key | Action |
|-----|--------|
| `←` / `→` / `h` / `l` | Switch column |
| `↑` / `↓` / `k` / `j` | Navigate cards |
| `Enter` | Open card detail |
| `Shift+←` / `Shift+→` | Move card to adjacent column |
| `Esc` | Return to main view |

## Card priorities

Each card has a coloured priority dot: `●` grey = low, `●` yellow = medium, `●` orange = high, `●` red = critical.

## Dependencies

Cards support two relationship types, both enforced as directed acyclic graphs (no cycles allowed):

- **Parent / child** — hierarchical grouping. Parent cards are shown in green; child count shown as `↓N`.
- **Blocks** — one card blocking another. Blocked cards are shown in red with a `⊘ blocked` indicator; blocking count shown as `⊘N`.
