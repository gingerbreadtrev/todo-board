package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ── Types ──────────────────────────────────────────────────────────────────────

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDone       Status = "done"
)

type EdgeType string

const (
	EdgeBlocks   EdgeType = "blocks"
	EdgeParentOf EdgeType = "parent_of"
)

type Board struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Column struct {
	ID        string
	BoardID   string
	Name      string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Card struct {
	ID          string
	BoardID     string
	ColumnID    string
	Title       string
	Description string
	Priority    Priority
	Status      Status
	Position    int
	Archived    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CardRow extends Card with dependency indicators used for display.
type CardRow struct {
	Card
	HasParent     bool // card has a parent_of edge pointing at it
	ChildrenCount int  // number of parent_of edges originating from it
	BlockingCount int  // number of blocks edges originating from it
	IsBlocked     bool // card has a blocks edge pointing at it
}

// ── DB ────────────────────────────────────────────────────────────────────────

// DB wraps the underlying SQLite connection.
type DB struct {
	db *sql.DB
}

// schema is applied once on open. PRAGMAs must precede DDL.
const dbSchema = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS boards (
	id         TEXT    PRIMARY KEY,
	name       TEXT    NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS columns (
	id         TEXT    PRIMARY KEY,
	board_id   TEXT    NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
	name       TEXT    NOT NULL,
	position   INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS cards (
	id          TEXT    PRIMARY KEY,
	board_id    TEXT    NOT NULL REFERENCES boards(id)  ON DELETE CASCADE,
	column_id   TEXT    NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
	title       TEXT    NOT NULL,
	description TEXT    NOT NULL DEFAULT '',
	priority    TEXT    NOT NULL DEFAULT 'low'
	                    CHECK(priority IN ('low','medium','high','critical')),
	status      TEXT    NOT NULL DEFAULT 'todo'
	                    CHECK(status IN ('todo','in_progress','blocked','done')),
	position    INTEGER NOT NULL DEFAULT 0,
	archived    INTEGER NOT NULL DEFAULT 0,
	created_at  DATETIME NOT NULL,
	updated_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS card_edges (
	source_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
	target_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
	edge_type TEXT NOT NULL CHECK(edge_type IN ('blocks','parent_of')),
	archived  INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (source_id, target_id, edge_type)
);

CREATE INDEX IF NOT EXISTS idx_columns_board   ON columns(board_id);
CREATE INDEX IF NOT EXISTS idx_cards_board     ON cards(board_id);
CREATE INDEX IF NOT EXISTS idx_cards_column    ON cards(column_id);
CREATE INDEX IF NOT EXISTS idx_edges_source    ON card_edges(source_id);
CREATE INDEX IF NOT EXISTS idx_edges_target    ON card_edges(target_id);
`

// DBPath returns the canonical database path for the current user.
func DBPath() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "todo-board", "todo-board.db"), nil
}

// OpenDB opens (or creates) the SQLite database at path.
// Pass ":memory:" for an in-memory database (useful in tests).
func OpenDB(path string) (*DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single writer avoids SQLITE_BUSY on concurrent calls within one process.
	sqldb.SetMaxOpenConns(1)

	if _, err := sqldb.Exec(dbSchema); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{db: sqldb}, nil
}

// Close releases the underlying database connection.
func (d *DB) Close() error { return d.db.Close() }

// ── Helpers ───────────────────────────────────────────────────────────────────

// newID generates a random UUID v4 string.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("rand.Read: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func now() time.Time { return time.Now().UTC() }

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// cardCols is the standard column list for SELECT on cards.
const cardCols = `id, board_id, column_id, title, description,
	priority, status, position, archived, created_at, updated_at`

func scanCard(s scanner) (Card, error) {
	var c Card
	var archived int
	var priority, status string
	err := s.Scan(
		&c.ID, &c.BoardID, &c.ColumnID, &c.Title, &c.Description,
		&priority, &status, &c.Position, &archived,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return Card{}, err
	}
	c.Priority = Priority(priority)
	c.Status = Status(status)
	c.Archived = archived != 0
	return c, nil
}

// ── Boards ────────────────────────────────────────────────────────────────────

func (d *DB) CreateBoard(name string) (Board, error) {
	b := Board{ID: newID(), Name: name, CreatedAt: now(), UpdatedAt: now()}
	_, err := d.db.Exec(
		`INSERT INTO boards (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		b.ID, b.Name, b.CreatedAt, b.UpdatedAt,
	)
	return b, err
}

func (d *DB) GetBoards() ([]Board, error) {
	rows, err := d.db.Query(
		`SELECT id, name, created_at, updated_at FROM boards ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (d *DB) GetBoard(id string) (Board, error) {
	var b Board
	err := d.db.QueryRow(
		`SELECT id, name, created_at, updated_at FROM boards WHERE id = ?`, id,
	).Scan(&b.ID, &b.Name, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

func (d *DB) RenameBoard(id, name string) error {
	_, err := d.db.Exec(
		`UPDATE boards SET name = ?, updated_at = ? WHERE id = ?`, name, now(), id,
	)
	return err
}

func (d *DB) DeleteBoard(id string) error {
	_, err := d.db.Exec(`DELETE FROM boards WHERE id = ?`, id)
	return err
}

// ── Columns ───────────────────────────────────────────────────────────────────

func (d *DB) CreateColumn(boardID, name string) (Column, error) {
	var maxPos sql.NullInt64
	_ = d.db.QueryRow(
		`SELECT MAX(position) FROM columns WHERE board_id = ?`, boardID,
	).Scan(&maxPos)
	pos := 0
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}
	c := Column{
		ID: newID(), BoardID: boardID, Name: name, Position: pos,
		CreatedAt: now(), UpdatedAt: now(),
	}
	_, err := d.db.Exec(
		`INSERT INTO columns (id, board_id, name, position, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.BoardID, c.Name, c.Position, c.CreatedAt, c.UpdatedAt,
	)
	return c, err
}

func (d *DB) GetColumns(boardID string) ([]Column, error) {
	rows, err := d.db.Query(
		`SELECT id, board_id, name, position, created_at, updated_at
		 FROM columns WHERE board_id = ? ORDER BY position`, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) GetColumn(id string) (Column, error) {
	var c Column
	err := d.db.QueryRow(
		`SELECT id, board_id, name, position, created_at, updated_at FROM columns WHERE id = ?`, id,
	).Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (d *DB) RenameColumn(id, name string) error {
	_, err := d.db.Exec(
		`UPDATE columns SET name = ?, updated_at = ? WHERE id = ?`, name, now(), id,
	)
	return err
}

// DeleteColumn removes a column and cascades to all cards in it.
func (d *DB) DeleteColumn(id string) error {
	_, err := d.db.Exec(`DELETE FROM columns WHERE id = ?`, id)
	return err
}

// MoveColumn sets an explicit position value on a column.
// Callers are responsible for keeping positions consistent across siblings.
func (d *DB) MoveColumn(id string, newPos int) error {
	_, err := d.db.Exec(
		`UPDATE columns SET position = ?, updated_at = ? WHERE id = ?`, newPos, now(), id,
	)
	return err
}

// ── Cards ─────────────────────────────────────────────────────────────────────

// CreateCard appends a new card at the end of the column.
func (d *DB) CreateCard(boardID, columnID, title string) (Card, error) {
	var maxPos sql.NullInt64
	_ = d.db.QueryRow(
		`SELECT MAX(position) FROM cards WHERE column_id = ? AND archived = 0`, columnID,
	).Scan(&maxPos)
	pos := 0
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}
	c := Card{
		ID: newID(), BoardID: boardID, ColumnID: columnID, Title: title,
		Priority: PriorityLow, Status: StatusTodo, Position: pos,
		CreatedAt: now(), UpdatedAt: now(),
	}
	_, err := d.db.Exec(
		`INSERT INTO cards
		 (id, board_id, column_id, title, description, priority, status, position, archived, created_at, updated_at)
		 VALUES (?, ?, ?, ?, '', ?, ?, ?, 0, ?, ?)`,
		c.ID, c.BoardID, c.ColumnID, c.Title,
		string(c.Priority), string(c.Status), c.Position,
		c.CreatedAt, c.UpdatedAt,
	)
	return c, err
}

func (d *DB) GetCard(id string) (Card, error) {
	return scanCard(d.db.QueryRow(
		`SELECT `+cardCols+` FROM cards WHERE id = ?`, id,
	))
}

// GetCards returns all cards for a board, optionally including archived ones.
func (d *DB) GetCards(boardID string, includeArchived bool) ([]Card, error) {
	q := `SELECT ` + cardCols + ` FROM cards WHERE board_id = ?`
	if !includeArchived {
		q += ` AND archived = 0`
	}
	q += ` ORDER BY position`
	rows, err := d.db.Query(q, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCardsWithMeta returns cards enriched with dependency indicator counts.
// Only non-archived edges to non-archived cards are counted.
func (d *DB) GetCardsWithMeta(boardID string, includeArchived bool) ([]CardRow, error) {
	q := `
	SELECT
		c.id, c.board_id, c.column_id, c.title, c.description,
		c.priority, c.status, c.position, c.archived, c.created_at, c.updated_at,

		-- has_parent: at least one non-archived parent_of edge points to this card
		CASE WHEN EXISTS (
			SELECT 1 FROM card_edges e
			JOIN cards p ON p.id = e.source_id
			WHERE e.target_id = c.id
			  AND e.edge_type = 'parent_of'
			  AND e.archived  = 0
			  AND p.archived  = 0
		) THEN 1 ELSE 0 END AS has_parent,

		-- children_count: non-archived children via parent_of
		COALESCE((
			SELECT COUNT(*) FROM card_edges e
			JOIN cards ch ON ch.id = e.target_id
			WHERE e.source_id = c.id
			  AND e.edge_type = 'parent_of'
			  AND e.archived  = 0
			  AND ch.archived = 0
		), 0) AS children_count,

		-- blocking_count: cards this card is currently blocking
		COALESCE((
			SELECT COUNT(*) FROM card_edges e
			JOIN cards bl ON bl.id = e.target_id
			WHERE e.source_id = c.id
			  AND e.edge_type = 'blocks'
			  AND e.archived  = 0
			  AND bl.archived = 0
		), 0) AS blocking_count,

		-- is_blocked: at least one active blocker edge points to this card
		CASE WHEN EXISTS (
			SELECT 1 FROM card_edges e
			JOIN cards bk ON bk.id = e.source_id
			WHERE e.target_id = c.id
			  AND e.edge_type = 'blocks'
			  AND e.archived  = 0
			  AND bk.archived = 0
		) THEN 1 ELSE 0 END AS is_blocked

	FROM cards c
	WHERE c.board_id = ?`

	if !includeArchived {
		q += ` AND c.archived = 0`
	}
	q += ` ORDER BY c.position`

	rows, err := d.db.Query(q, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CardRow
	for rows.Next() {
		var cr CardRow
		var archived, hasParent, isBlocked int
		var priority, status string
		if err := rows.Scan(
			&cr.ID, &cr.BoardID, &cr.ColumnID, &cr.Title, &cr.Description,
			&priority, &status, &cr.Position, &archived,
			&cr.CreatedAt, &cr.UpdatedAt,
			&hasParent, &cr.ChildrenCount, &cr.BlockingCount, &isBlocked,
		); err != nil {
			return nil, err
		}
		cr.Priority = Priority(priority)
		cr.Status = Status(status)
		cr.Archived = archived != 0
		cr.HasParent = hasParent != 0
		cr.IsBlocked = isBlocked != 0
		out = append(out, cr)
	}
	return out, rows.Err()
}

func (d *DB) RenameCard(id, title string) error {
	_, err := d.db.Exec(
		`UPDATE cards SET title = ?, updated_at = ? WHERE id = ?`, title, now(), id,
	)
	return err
}

func (d *DB) SetCardDescription(id, description string) error {
	_, err := d.db.Exec(
		`UPDATE cards SET description = ?, updated_at = ? WHERE id = ?`, description, now(), id,
	)
	return err
}

func (d *DB) SetCardPriority(id string, priority Priority) error {
	_, err := d.db.Exec(
		`UPDATE cards SET priority = ?, updated_at = ? WHERE id = ?`, string(priority), now(), id,
	)
	return err
}

func (d *DB) SetCardStatus(id string, status Status) error {
	_, err := d.db.Exec(
		`UPDATE cards SET status = ?, updated_at = ? WHERE id = ?`, string(status), now(), id,
	)
	return err
}

// ToggleCardDone flips between done and todo.
func (d *DB) ToggleCardDone(id string) error {
	_, err := d.db.Exec(`
		UPDATE cards
		SET status     = CASE WHEN status = 'done' THEN 'todo' ELSE 'done' END,
		    updated_at = ?
		WHERE id = ?`, now(), id)
	return err
}

// SetCardPosition directly sets the position within a column.
func (d *DB) SetCardPosition(id string, position int) error {
	_, err := d.db.Exec(
		`UPDATE cards SET position = ?, updated_at = ? WHERE id = ?`, position, now(), id,
	)
	return err
}

// ArchiveCard marks a card and all its edges as archived.
func (d *DB) ArchiveCard(id string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`UPDATE cards SET archived = 1, updated_at = ? WHERE id = ?`, now(), id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE card_edges SET archived = 1 WHERE source_id = ? OR target_id = ?`, id, id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// RestoreCard unarchives a card and reactivates edges whose both endpoints are active.
func (d *DB) RestoreCard(id string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`UPDATE cards SET archived = 0, updated_at = ? WHERE id = ?`, now(), id,
	); err != nil {
		return err
	}
	// Only restore edges where neither endpoint is still archived.
	if _, err := tx.Exec(`
		UPDATE card_edges SET archived = 0
		WHERE (source_id = ? OR target_id = ?)
		  AND archived = 1
		  AND source_id IN (SELECT id FROM cards WHERE archived = 0)
		  AND target_id IN (SELECT id FROM cards WHERE archived = 0)`,
		id, id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteCardPermanent removes a card from the database entirely.
func (d *DB) DeleteCardPermanent(id string) error {
	_, err := d.db.Exec(`DELETE FROM cards WHERE id = ?`, id)
	return err
}

// MoveCardToColumn moves a card to a different column, appending it at the end.
func (d *DB) MoveCardToColumn(cardID, columnID string) error {
	var maxPos sql.NullInt64
	_ = d.db.QueryRow(
		`SELECT MAX(position) FROM cards WHERE column_id = ? AND archived = 0`, columnID,
	).Scan(&maxPos)
	pos := 0
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}
	_, err := d.db.Exec(
		`UPDATE cards SET column_id = ?, position = ?, updated_at = ? WHERE id = ?`,
		columnID, pos, now(), cardID,
	)
	return err
}

// ── Edges ─────────────────────────────────────────────────────────────────────

// hasCycle performs a BFS from targetID along edges of edgeType and reports
// whether sourceID is reachable (which would close a cycle).
func (d *DB) hasCycle(tx *sql.Tx, sourceID, targetID string, edgeType EdgeType) (bool, error) {
	visited := make(map[string]bool)
	queue := []string{targetID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == sourceID {
			return true, nil
		}
		if visited[current] {
			continue
		}
		visited[current] = true

		rows, err := tx.Query(
			`SELECT target_id FROM card_edges
			 WHERE source_id = ? AND edge_type = ? AND archived = 0`,
			current, string(edgeType),
		)
		if err != nil {
			return false, err
		}
		for rows.Next() {
			var next string
			if err := rows.Scan(&next); err != nil {
				rows.Close()
				return false, err
			}
			if !visited[next] {
				queue = append(queue, next)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return false, err
		}
	}
	return false, nil
}

// AddEdge inserts a directed edge (sourceID → targetID) of the given type.
// Returns an error if the edge would create a cycle or is self-referential.
// If the edge already exists but was soft-archived, it is reactivated.
func (d *DB) AddEdge(sourceID, targetID string, edgeType EdgeType) error {
	if sourceID == targetID {
		return fmt.Errorf("self-referential edge not allowed")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	cycle, err := d.hasCycle(tx, sourceID, targetID, edgeType)
	if err != nil {
		return fmt.Errorf("cycle check: %w", err)
	}
	if cycle {
		return fmt.Errorf("edge %s→%s (%s) would create a cycle", sourceID, targetID, edgeType)
	}

	_, err = tx.Exec(`
		INSERT INTO card_edges (source_id, target_id, edge_type, archived)
		VALUES (?, ?, ?, 0)
		ON CONFLICT(source_id, target_id, edge_type) DO UPDATE SET archived = 0`,
		sourceID, targetID, string(edgeType),
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteEdge removes a directed edge permanently.
func (d *DB) DeleteEdge(sourceID, targetID string, edgeType EdgeType) error {
	_, err := d.db.Exec(
		`DELETE FROM card_edges WHERE source_id = ? AND target_id = ? AND edge_type = ?`,
		sourceID, targetID, string(edgeType),
	)
	return err
}

// getLinkedCards is a generic helper that executes a query returning cards.
func (d *DB) getLinkedCards(query string, args ...any) ([]Card, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetParents returns all non-archived cards that are parents of cardID.
func (d *DB) GetParents(cardID string) ([]Card, error) {
	return d.getLinkedCards(`
		SELECT `+cardCols+` FROM cards
		WHERE id IN (
			SELECT source_id FROM card_edges
			WHERE target_id = ? AND edge_type = 'parent_of' AND archived = 0
		) AND archived = 0`, cardID)
}

// GetChildren returns all non-archived cards that are children of cardID.
func (d *DB) GetChildren(cardID string) ([]Card, error) {
	return d.getLinkedCards(`
		SELECT `+cardCols+` FROM cards
		WHERE id IN (
			SELECT target_id FROM card_edges
			WHERE source_id = ? AND edge_type = 'parent_of' AND archived = 0
		) AND archived = 0`, cardID)
}

// GetBlockers returns all non-archived cards that are blocking cardID.
func (d *DB) GetBlockers(cardID string) ([]Card, error) {
	return d.getLinkedCards(`
		SELECT `+cardCols+` FROM cards
		WHERE id IN (
			SELECT source_id FROM card_edges
			WHERE target_id = ? AND edge_type = 'blocks' AND archived = 0
		) AND archived = 0`, cardID)
}

// GetBlocking returns all non-archived cards that cardID is currently blocking.
func (d *DB) GetBlocking(cardID string) ([]Card, error) {
	return d.getLinkedCards(`
		SELECT `+cardCols+` FROM cards
		WHERE id IN (
			SELECT target_id FROM card_edges
			WHERE source_id = ? AND edge_type = 'blocks' AND archived = 0
		) AND archived = 0`, cardID)
}
