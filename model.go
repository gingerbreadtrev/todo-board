package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type panelFocus int

const (
	focusBoards panelFocus = iota
	focusCards
)

type overlayMode int

const (
	overlayNone overlayMode = iota
	overlayInput
	overlayConfirm
	overlayPriority
	overlayPicker
	overlayDescEdit
	overlayHelp
	overlayColumnPicker
)

// ── Detail view areas ─────────────────────────────────────────────────────────

const (
	detailAreaProps = 0 // properties rows (title, phase, priority, status)
	detailAreaDesc  = 1 // scrollable description
	detailAreaRels  = 2 // 4 relation tables side by side
)

// detailStackFrame saves context for going back when drilling into a related card.
type detailStackFrame struct {
	card       Card
	area       int
	rowIdx     int
	relCol     int
	descScroll int
}

// pickerItem is one row in the dependency picker popup.
type pickerItem struct {
	card    CardRow
	checked bool
}

type inputAction int

const (
	inputNewBoard inputAction = iota
	inputNewCard
	inputRenameBoard
	inputRenameCard
)

type confirmAction int

const (
	confirmArchive confirmAction = iota
	confirmDeleteBoard
)

// Model is the top-level BubbleTea model.
type Model struct {
	db     *DB
	width  int
	height int

	boards   []Board
	boardIdx int

	columns []Column
	cards   []CardRow
	cardIdx int

	focus       panelFocus
	archiveView bool
	pendingD    bool // first d in archive view; second d deletes permanently

	overlay       overlayMode
	overlayCardID string // card ID used by overlays that operate on a specific card
	inputAction   inputAction
	inputField    textinput.Model
	confirmAction confirmAction
	confirmTarget string // displayed in the confirm dialog

	priorityIdx int // cursor position within the priority picker (0–3)

	searchMode  bool
	searchQuery string

	kanbanView    bool
	kanbanColIdx  int // focused column index (into m.columns)
	kanbanCardIdx int // focused card index within the focused column

	pickerEdgeType   EdgeType
	pickerFocusID    string // ID of the card that opened the picker
	pickerFocusTitle string
	pickerItems      []pickerItem
	pickerIdx        int
	pickerErr        string // non-empty when the last toggle produced a cycle error

	// Detail view
	detailView     bool
	detailCard     Card
	detailParents  []Card
	detailChildren []Card
	detailBlockers []Card
	detailBlocking []Card
	detailSection  int // active area (detailAreaProps / detailAreaDesc / detailAreaRels)
	detailRowIdx   int // cursor row within the active area
	detailRelCol   int // focused relation table: 0=parents 1=children 2=blocking 3=blocked-by
	detailDescScroll int // line scroll offset for the description section
	detailStack    []detailStackFrame

	colPickerIdx int // cursor in overlayColumnPicker

	// Description edit overlay
	descInput textarea.Model
}

func NewModel(db *DB) (Model, error) {
	boards, err := db.GetBoards()
	if err != nil {
		return Model{}, err
	}

	m := Model{
		db:     db,
		boards: boards,
		focus:  focusBoards,
	}

	if len(boards) > 0 {
		m = m.loadBoard(0)
	}

	return m, nil
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) loadBoard(idx int) Model {
	if idx < 0 || idx >= len(m.boards) {
		return m
	}
	m.boardIdx = idx
	board := m.boards[idx]
	m.columns, _ = m.db.GetColumns(board.ID)
	m.cardIdx = 0
	return m.loadCards()
}

// loadCards reloads m.cards for the current board, respecting archiveView.
// In archive view, only archived cards are shown.
func (m Model) loadCards() Model {
	if len(m.boards) == 0 || m.boardIdx >= len(m.boards) {
		return m
	}
	board := m.boards[m.boardIdx]
	all, _ := m.db.GetCardsWithMeta(board.ID, m.archiveView)
	if m.archiveView {
		var archived []CardRow
		for _, c := range all {
			if c.Archived {
				archived = append(archived, c)
			}
		}
		m.cards = archived
	} else {
		m.cards = all
	}
	if m.cardIdx >= len(m.cards) {
		if len(m.cards) > 0 {
			m.cardIdx = len(m.cards) - 1
		} else {
			m.cardIdx = 0
		}
	}
	return m
}

// refreshDetail reloads the card and all relationship slices.
func (m Model) refreshDetail() Model {
	card, err := m.db.GetCard(m.detailCard.ID)
	if err != nil {
		return m
	}
	m.detailCard = card
	m.detailParents, _ = m.db.GetParents(card.ID)
	m.detailChildren, _ = m.db.GetChildren(card.ID)
	m.detailBlockers, _ = m.db.GetBlockers(card.ID)
	m.detailBlocking, _ = m.db.GetBlocking(card.ID)
	// Clamp row cursor to valid range for the active section.
	m = m.clampDetailRow()
	return m
}

// detailSectionRowCount returns the number of navigable rows in the active area.
// The description area uses scroll instead of a row cursor, so it returns 0.
func (m Model) detailSectionRowCount() int {
	switch m.detailSection {
	case detailAreaProps:
		return 4 // title, phase, priority, status
	case detailAreaDesc:
		return 0
	case detailAreaRels:
		switch m.detailRelCol {
		case 0:
			return len(m.detailParents)
		case 1:
			return len(m.detailChildren) + 1 // +1 for "Add child"
		case 2:
			return len(m.detailBlocking) + 1 // +1 for "Add card to block"
		case 3:
			return len(m.detailBlockers)
		}
	}
	return 0
}

// clampDetailRow ensures detailRowIdx is within bounds for the current section.
func (m Model) clampDetailRow() Model {
	n := m.detailSectionRowCount()
	if n == 0 {
		m.detailRowIdx = 0
		return m
	}
	if m.detailRowIdx >= n {
		m.detailRowIdx = n - 1
	}
	if m.detailRowIdx < 0 {
		m.detailRowIdx = 0
	}
	return m
}

// columnName returns the name of the column with the given ID.
func (m Model) columnName(id string) string {
	for _, col := range m.columns {
		if col.ID == id {
			return col.Name
		}
	}
	return id
}

// visibleCards returns the subset of m.cards that match the current search
// query (case-insensitive substring). Returns all cards when no query is set.
func (m Model) visibleCards() []CardRow {
	if m.searchQuery == "" {
		return m.cards
	}
	q := strings.ToLower(m.searchQuery)
	var out []CardRow
	for _, c := range m.cards {
		if strings.Contains(strings.ToLower(c.Title), q) {
			out = append(out, c)
		}
	}
	return out
}

// cardsForColumn returns the cards in m.cards that belong to colID, preserving
// their order (which matches position order from the DB query).
func (m Model) cardsForColumn(colID string) []CardRow {
	var out []CardRow
	for _, c := range m.cards {
		if c.ColumnID == colID {
			out = append(out, c)
		}
	}
	return out
}

// columnForNewCard returns the column ID to use for a new card: the column of
// the currently selected card, or the first column if no cards are present.
func (m Model) columnForNewCard() string {
	if len(m.columns) == 0 {
		return ""
	}
	if len(m.cards) > 0 && m.cardIdx < len(m.cards) {
		return m.cards[m.cardIdx].ColumnID
	}
	return m.columns[0].ID
}

// focusedCardID returns the ID and title of the currently focused card across
// all views: detail, kanban, or the card list panel.
func (m Model) focusedCardID() (id, title string, ok bool) {
	if m.detailView {
		return m.detailCard.ID, m.detailCard.Title, true
	}
	if m.kanbanView {
		if m.kanbanColIdx >= len(m.columns) {
			return
		}
		colCards := m.cardsForColumn(m.columns[m.kanbanColIdx].ID)
		if m.kanbanCardIdx >= len(colCards) {
			return
		}
		c := colCards[m.kanbanCardIdx]
		return c.ID, c.Title, true
	}
	if m.focus != focusCards || len(m.cards) == 0 || m.cardIdx >= len(m.cards) {
		return
	}
	c := m.cards[m.cardIdx]
	return c.ID, c.Title, true
}
