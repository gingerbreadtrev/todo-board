package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.overlay {
		case overlayInput:
			return m.handleInputKey(msg)
		case overlayConfirm:
			return m.handleConfirmKey(msg)
		case overlayPriority:
			return m.handlePriorityKey(msg)
		case overlayPicker:
			return m.handlePickerKey(msg)
		case overlayDescEdit:
			return m.handleDescEditKey(msg)
		case overlayHelp:
			return m.handleHelpKey(msg)
		case overlayColumnPicker:
			return m.handleColPickerKey(msg)
		}
		if m.detailView {
			return m.handleDetailKey(msg)
		}
		if m.searchMode {
			return m.handleSearchKey(msg)
		}
		if m.kanbanView {
			return m.handleKanbanKey(msg)
		}
		return m.handleNormalKey(msg)
	}

	return m, nil
}

// ── Normal key handling ───────────────────────────────────────────────────────

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Resolve a pending "dd" first.
	if m.pendingD {
		m.pendingD = false
		if key == "d" && m.focus == focusCards && m.archiveView {
			return m.doDeletePermanent()
		}
		// Any other key: pendingD cancelled; fall through to handle the key.
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab":
		if m.focus == focusBoards {
			m.focus = focusCards
		} else {
			m.focus = focusBoards
		}

	case "up", "k":
		m = m.navigateUp()

	case "down", "j":
		m = m.navigateDown()

	case "enter":
		if m.focus == focusBoards && len(m.boards) > 0 {
			m = m.loadBoard(m.boardIdx)
			m.focus = focusCards
		} else if m.focus == focusCards && len(m.visibleCards()) > 0 {
			vc := m.visibleCards()
			if m.cardIdx < len(vc) {
				return m.openDetail(vc[m.cardIdx].ID)
			}
		}

	case "left", "h":
		if m.focus == focusCards {
			m = m.moveCardToAdjacentColumn(-1)
		}

	case "right", "l":
		if m.focus == focusCards {
			m = m.moveCardToAdjacentColumn(1)
		}

	case "n":
		return m.handleNew()

	case "r":
		if m.focus == focusCards && m.archiveView {
			return m.doRestore()
		}
		if m.focus == focusCards && len(m.cards) > 0 {
			m.overlayCardID = m.cards[m.cardIdx].ID
		} else {
			m.overlayCardID = ""
		}
		return m.handleRename()

	case "e":
		if m.focus == focusCards && !m.archiveView && len(m.cards) > 0 {
			return m.openDescEdit()
		}

	case "c":
		if m.focus == focusCards && len(m.cards) > 0 {
			_ = m.db.ToggleCardDone(m.cards[m.cardIdx].ID)
			m = m.loadCards()
		}

	case "d":
		switch m.focus {
		case focusBoards:
			if len(m.boards) > 0 {
				m.confirmAction = confirmDeleteBoard
				m.confirmTarget = m.boards[m.boardIdx].Name
				m.overlay = overlayConfirm
			}
		case focusCards:
			if len(m.cards) > 0 {
				if m.archiveView {
					m.pendingD = true
				} else {
					cr := m.cards[m.cardIdx]
					m.overlayCardID = cr.ID
					m.confirmAction = confirmArchive
					m.confirmTarget = cr.Title
					m.overlay = overlayConfirm
				}
			}
		}

	case "D":
		if m.focus == focusCards {
			m.archiveView = !m.archiveView
			m = m.loadCards()
		}

	case "p":
		if m.focus == focusCards && len(m.cards) > 0 {
			m.overlayCardID = m.cards[m.cardIdx].ID
			current := m.cards[m.cardIdx].Priority
			for i, p := range priorityOptions {
				if p == current {
					m.priorityIdx = i
					break
				}
			}
			m.overlay = overlayPriority
		}

	case "/":
		if m.focus == focusCards {
			m.searchMode = true
			m.searchQuery = ""
			m.cardIdx = 0
		}

	case "shift+tab", "V":
		if len(m.boards) > 0 {
			m = m.enterKanban()
		}

	case "b":
		if m.focus == focusCards && len(m.cards) > 0 {
			return m.openPicker(EdgeBlocks)
		}

	case "s":
		if m.focus == focusCards && len(m.cards) > 0 {
			return m.openPicker(EdgeParentOf)
		}

	case "?":
		m.overlay = overlayHelp
	}

	return m, nil
}

// enterKanban switches to kanban view, positioning the column/card cursor so
// that the same card that is focused in the list view remains focused.
func (m Model) enterKanban() Model {
	m.kanbanView = true
	m.kanbanColIdx = 0
	m.kanbanCardIdx = 0
	if m.focus == focusCards && m.cardIdx < len(m.cards) {
		focused := m.cards[m.cardIdx]
		for ci, col := range m.columns {
			if col.ID == focused.ColumnID {
				m.kanbanColIdx = ci
				for i, c := range m.cardsForColumn(col.ID) {
					if c.ID == focused.ID {
						m.kanbanCardIdx = i
						break
					}
				}
				break
			}
		}
	}
	return m
}

func (m Model) navigateUp() Model {
	switch m.focus {
	case focusBoards:
		if m.boardIdx > 0 {
			m = m.loadBoard(m.boardIdx - 1)
		}
	case focusCards:
		if m.cardIdx > 0 {
			m.cardIdx--
		}
	}
	return m
}

func (m Model) navigateDown() Model {
	switch m.focus {
	case focusBoards:
		if m.boardIdx < len(m.boards)-1 {
			m = m.loadBoard(m.boardIdx + 1)
		}
	case focusCards:
		if m.cardIdx < len(m.cards)-1 {
			m.cardIdx++
		}
	}
	return m
}

// moveCardToAdjacentColumn moves the focused card to the previous (dir=-1) or
// next (dir=+1) column in the board, then reloads and restores the cursor.
func (m Model) moveCardToAdjacentColumn(dir int) Model {
	if m.focus != focusCards || len(m.cards) == 0 || m.cardIdx >= len(m.cards) {
		return m
	}
	cr := m.cards[m.cardIdx]
	currentColIdx := -1
	for i, col := range m.columns {
		if col.ID == cr.ColumnID {
			currentColIdx = i
			break
		}
	}
	if currentColIdx < 0 {
		return m
	}
	targetColIdx := currentColIdx + dir
	if targetColIdx < 0 || targetColIdx >= len(m.columns) {
		return m
	}
	if err := m.db.MoveCardToColumn(cr.ID, m.columns[targetColIdx].ID); err != nil {
		return m
	}
	cardID := cr.ID
	m = m.loadCards()
	for i, c := range m.cards {
		if c.ID == cardID {
			m.cardIdx = i
			break
		}
	}
	return m
}

// ── Detail view ───────────────────────────────────────────────────────────────

// openDetailInternal opens (or re-opens) the detail view for cardID without
// touching the stack. Callers are responsible for managing the stack.
func (m Model) openDetailInternal(cardID string) (Model, tea.Cmd) {
	card, err := m.db.GetCard(cardID)
	if err != nil {
		return m, nil
	}
	m.detailCard = card
	m.detailView = true
	m.detailSection = detailAreaProps
	m.detailRowIdx = 0
	m.detailDescScroll = 0
	m = m.refreshDetail()
	return m, nil
}

// openDetail is the public entry point (called from list/kanban Enter). It clears
// the detail stack so Escape returns to the list view.
func (m Model) openDetail(cardID string) (tea.Model, tea.Cmd) {
	m.detailStack = nil
	return m.openDetailInternal(cardID)
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc":
		if len(m.detailStack) > 0 {
			frame := m.detailStack[len(m.detailStack)-1]
			m.detailStack = m.detailStack[:len(m.detailStack)-1]
			newM, cmd := m.openDetailInternal(frame.card.ID)
			newM.detailStack = m.detailStack
			newM.detailSection = frame.area
			newM.detailRelCol = frame.relCol
			newM.detailDescScroll = frame.descScroll
			newM.detailRowIdx = frame.rowIdx
			newM = newM.clampDetailRow()
			return newM, cmd
		}
		m.detailView = false
		m.detailStack = nil
		m = m.loadCards()
		return m, nil

	case "tab":
		m.detailSection = (m.detailSection + 1) % 3
		m.detailRowIdx = 0
		m.detailDescScroll = 0
		return m, nil

	case "shift+tab":
		m.detailSection = (m.detailSection + 2) % 3
		m.detailRowIdx = 0
		m.detailDescScroll = 0
		return m, nil

	case "left", "h":
		if m.detailSection == detailAreaRels && m.detailRelCol > 0 {
			m.detailRelCol--
			m.detailRowIdx = 0
		}
		return m, nil

	case "right", "l":
		if m.detailSection == detailAreaRels && m.detailRelCol < 3 {
			m.detailRelCol++
			m.detailRowIdx = 0
		}
		return m, nil

	case "up", "k":
		if m.detailSection == detailAreaDesc {
			if m.detailDescScroll > 0 {
				m.detailDescScroll--
			}
		} else {
			if m.detailRowIdx > 0 {
				m.detailRowIdx--
			}
		}
		return m, nil

	case "down", "j":
		if m.detailSection == detailAreaDesc {
			m.detailDescScroll++
		} else {
			n := m.detailSectionRowCount()
			if m.detailRowIdx < n-1 {
				m.detailRowIdx++
			}
		}
		return m, nil

	case "enter":
		return m.activateDetailSection()

	case "x":
		return m.handleDetailRemoveRelationship()

	case "d":
		m.overlayCardID = m.detailCard.ID
		m.confirmAction = confirmArchive
		m.confirmTarget = m.detailCard.Title
		m.overlay = overlayConfirm
		return m, nil

	case "?":
		m.overlay = overlayHelp
		return m, nil
	}
	return m, nil
}

// activateDetailSection handles Enter based on the current area and row.
func (m Model) activateDetailSection() (tea.Model, tea.Cmd) {
	switch m.detailSection {
	case detailAreaProps:
		switch m.detailRowIdx {
		case 0: // Title
			ti := textinput.New()
			ti.SetValue(m.detailCard.Title)
			ti.CharLimit = 120
			cmd := ti.Focus()
			m.inputField = ti
			m.inputAction = inputRenameCard
			m.overlayCardID = m.detailCard.ID
			m.overlay = overlayInput
			return m, cmd
		case 1: // Phase
			m.overlayCardID = m.detailCard.ID
			for i, col := range m.columns {
				if col.ID == m.detailCard.ColumnID {
					m.colPickerIdx = i
					break
				}
			}
			m.overlay = overlayColumnPicker
			return m, nil
		case 2: // Priority
			m.overlayCardID = m.detailCard.ID
			for i, p := range priorityOptions {
				if p == m.detailCard.Priority {
					m.priorityIdx = i
					break
				}
			}
			m.overlay = overlayPriority
			return m, nil
		case 3: // Status
			_ = m.db.ToggleCardDone(m.detailCard.ID)
			m = m.refreshDetail()
			m = m.loadCards()
			return m, nil
		}

	case detailAreaDesc:
		return m.openDescEdit()

	case detailAreaRels:
		switch m.detailRelCol {
		case 0: // parents
			if m.detailRowIdx < len(m.detailParents) {
				return m.drillIntoCard(m.detailParents[m.detailRowIdx])
			}
		case 1: // children
			addRow := len(m.detailChildren)
			if m.detailRowIdx == addRow {
				return m.openPicker(EdgeParentOf)
			}
			if m.detailRowIdx < len(m.detailChildren) {
				return m.drillIntoCard(m.detailChildren[m.detailRowIdx])
			}
		case 2: // blocking
			addRow := len(m.detailBlocking)
			if m.detailRowIdx == addRow {
				return m.openPicker(EdgeBlocks)
			}
			if m.detailRowIdx < len(m.detailBlocking) {
				return m.drillIntoCard(m.detailBlocking[m.detailRowIdx])
			}
		case 3: // blocked by
			if m.detailRowIdx < len(m.detailBlockers) {
				return m.drillIntoCard(m.detailBlockers[m.detailRowIdx])
			}
		}
	}
	return m, nil
}

// drillIntoCard pushes the current context onto the stack and opens the given card.
func (m Model) drillIntoCard(target Card) (tea.Model, tea.Cmd) {
	m.detailStack = append(m.detailStack, detailStackFrame{
		card:       m.detailCard,
		area:       m.detailSection,
		rowIdx:     m.detailRowIdx,
		relCol:     m.detailRelCol,
		descScroll: m.detailDescScroll,
	})
	stack := m.detailStack
	newM, cmd := m.openDetailInternal(target.ID)
	newM.detailStack = stack
	return newM, cmd
}

// handleDetailRemoveRelationship removes the focused relationship in the rels area.
func (m Model) handleDetailRemoveRelationship() (tea.Model, tea.Cmd) {
	if m.detailSection != detailAreaRels {
		return m, nil
	}
	switch m.detailRelCol {
	case 0: // parents
		if m.detailRowIdx < len(m.detailParents) {
			_ = m.db.DeleteEdge(m.detailParents[m.detailRowIdx].ID, m.detailCard.ID, EdgeParentOf)
		}
	case 1: // children
		if m.detailRowIdx < len(m.detailChildren) {
			_ = m.db.DeleteEdge(m.detailCard.ID, m.detailChildren[m.detailRowIdx].ID, EdgeParentOf)
		}
	case 2: // blocking
		if m.detailRowIdx < len(m.detailBlocking) {
			_ = m.db.DeleteEdge(m.detailCard.ID, m.detailBlocking[m.detailRowIdx].ID, EdgeBlocks)
		}
	case 3: // blocked by
		if m.detailRowIdx < len(m.detailBlockers) {
			_ = m.db.DeleteEdge(m.detailBlockers[m.detailRowIdx].ID, m.detailCard.ID, EdgeBlocks)
		}
	default:
		return m, nil
	}
	m = m.refreshDetail()
	m = m.loadCards()
	return m, nil
}

// ── Column picker overlay ─────────────────────────────────────────────────────

func (m Model) handleColPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.colPickerIdx > 0 {
			m.colPickerIdx--
		}
	case "down", "j":
		if m.colPickerIdx < len(m.columns)-1 {
			m.colPickerIdx++
		}
	case "enter":
		if m.colPickerIdx < len(m.columns) && m.detailCard.ID != "" {
			_ = m.db.MoveCardToColumn(m.detailCard.ID, m.columns[m.colPickerIdx].ID)
		}
		m.overlay = overlayNone
		if m.detailView {
			m = m.refreshDetail()
		}
		m = m.loadCards()
	case "esc":
		m.overlay = overlayNone
	}
	return m, nil
}

// ── Description edit overlay ──────────────────────────────────────────────────

func (m Model) openDescEdit() (tea.Model, tea.Cmd) {
	id, _, ok := m.focusedCardID()
	if !ok {
		return m, nil
	}
	card, err := m.db.GetCard(id)
	if err != nil {
		return m, nil
	}

	popupW := m.width * 3 / 4
	if popupW < 40 {
		popupW = 40
	}
	innerW := popupW - 8 // subtract popup padding (3 each side) + borders
	if innerW < 20 {
		innerW = 20
	}

	ta := textarea.New()
	ta.SetValue(card.Description)
	ta.CharLimit = 0
	ta.SetWidth(innerW)
	ta.SetHeight(12)
	cmd := ta.Focus()

	m.descInput = ta
	m.overlayCardID = id
	m.overlay = overlayDescEdit
	return m, cmd
}

func (m Model) handleDescEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		desc := strings.TrimRight(m.descInput.Value(), "\n")
		_ = m.db.SetCardDescription(m.overlayCardID, desc)
		m.overlay = overlayNone
		if m.detailView {
			m = m.refreshDetail()
		}
		m = m.loadCards()
		return m, nil
	case "esc":
		m.overlay = overlayNone
		return m, nil
	}
	var cmd tea.Cmd
	m.descInput, cmd = m.descInput.Update(msg)
	return m, cmd
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "?", "q":
		m.overlay = overlayNone
	}
	return m, nil
}

// ── Overlay key handling ──────────────────────────────────────────────────────

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.inputField.Value())
		if value != "" {
			m = m.commitInput(value)
		}
		m.overlay = overlayNone
		return m, nil
	case "esc":
		m.overlay = overlayNone
		return m, nil
	default:
		var cmd tea.Cmd
		m.inputField, cmd = m.inputField.Update(msg)
		return m, cmd
	}
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "enter", "y":
		m.overlay = overlayNone
		switch m.confirmAction {
		case confirmArchive:
			if m.overlayCardID != "" {
				_ = m.db.ArchiveCard(m.overlayCardID)
			}
			if m.detailView {
				m.detailView = false
			}
			m = m.loadCards()
		case confirmDeleteBoard:
			if len(m.boards) > 0 {
				_ = m.db.DeleteBoard(m.boards[m.boardIdx].ID)
				m.boards, _ = m.db.GetBoards()
				if len(m.boards) == 0 {
					m.cards = nil
					m.columns = nil
					m.boardIdx = 0
				} else {
					if m.boardIdx >= len(m.boards) {
						m.boardIdx = len(m.boards) - 1
					}
					m = m.loadBoard(m.boardIdx)
				}
			}
		}
	case "esc", "n":
		m.overlay = overlayNone
	}
	return m, nil
}

// priorityOptions defines the ordered list of selectable priorities.
var priorityOptions = []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical}

func (m Model) handlePriorityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.priorityIdx > 0 {
			m.priorityIdx--
		}
	case "down", "j":
		if m.priorityIdx < len(priorityOptions)-1 {
			m.priorityIdx++
		}
	case "enter":
		m.overlay = overlayNone
		if m.overlayCardID != "" {
			_ = m.db.SetCardPriority(m.overlayCardID, priorityOptions[m.priorityIdx])
		}
		if m.detailView {
			m = m.refreshDetail()
		}
		m = m.loadCards()
	case "esc":
		m.overlay = overlayNone
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchMode = false
		m.searchQuery = ""
		m.cardIdx = 0
	case "backspace", "ctrl+h":
		if len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
			m.cardIdx = 0
		}
	// In search mode only arrow keys navigate — k/j would be typed into the query.
	case "up":
		if m.cardIdx > 0 {
			m.cardIdx--
		}
	case "down":
		visible := m.visibleCards()
		if m.cardIdx < len(visible)-1 {
			m.cardIdx++
		}
	default:
		if len(msg.Runes) > 0 {
			m.searchQuery += string(msg.Runes)
			m.cardIdx = 0
		}
	}
	return m, nil
}

// ── Action handlers ───────────────────────────────────────────────────────────

func (m Model) handleNew() (tea.Model, tea.Cmd) {
	if m.archiveView {
		return m, nil
	}
	var prompt string
	var action inputAction
	if m.focus == focusBoards {
		action = inputNewBoard
		prompt = "Board name"
	} else {
		if len(m.boards) == 0 {
			return m, nil
		}
		action = inputNewCard
		prompt = "Card title"
	}
	ti := textinput.New()
	ti.Placeholder = prompt
	ti.CharLimit = 120
	cmd := ti.Focus()
	m.inputField = ti
	m.inputAction = action
	m.overlay = overlayInput
	return m, cmd
}

func (m Model) handleRename() (tea.Model, tea.Cmd) {
	var current string
	var action inputAction
	switch m.focus {
	case focusBoards:
		if len(m.boards) == 0 {
			return m, nil
		}
		current = m.boards[m.boardIdx].Name
		action = inputRenameBoard
	case focusCards:
		if len(m.cards) == 0 {
			return m, nil
		}
		current = m.cards[m.cardIdx].Title
		action = inputRenameCard
	}
	ti := textinput.New()
	ti.SetValue(current)
	ti.CharLimit = 120
	cmd := ti.Focus()
	m.inputField = ti
	m.inputAction = action
	m.overlay = overlayInput
	return m, cmd
}

func (m Model) doRestore() (tea.Model, tea.Cmd) {
	if len(m.cards) == 0 || m.cardIdx >= len(m.cards) {
		return m, nil
	}
	_ = m.db.RestoreCard(m.cards[m.cardIdx].ID)
	m = m.loadCards()
	return m, nil
}

func (m Model) doDeletePermanent() (tea.Model, tea.Cmd) {
	if len(m.cards) == 0 || m.cardIdx >= len(m.cards) {
		return m, nil
	}
	_ = m.db.DeleteCardPermanent(m.cards[m.cardIdx].ID)
	m = m.loadCards()
	return m, nil
}

// ── Kanban key handling ───────────────────────────────────────────────────────

func (m Model) handleKanbanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc", "shift+tab":
		// Return to list view, syncing the card cursor.
		m.kanbanView = false
		m.focus = focusCards
		if m.kanbanColIdx < len(m.columns) {
			colCards := m.cardsForColumn(m.columns[m.kanbanColIdx].ID)
			if m.kanbanCardIdx < len(colCards) {
				focusedID := colCards[m.kanbanCardIdx].ID
				for i, c := range m.cards {
					if c.ID == focusedID {
						m.cardIdx = i
						break
					}
				}
			}
		}

	case "left", "h":
		if m.kanbanColIdx > 0 {
			m.kanbanColIdx--
			m.kanbanCardIdx = 0
		}

	case "right", "l":
		if m.kanbanColIdx < len(m.columns)-1 {
			m.kanbanColIdx++
			m.kanbanCardIdx = 0
		}

	case "up", "k":
		if m.kanbanCardIdx > 0 {
			m.kanbanCardIdx--
		}

	case "down", "j":
		if m.kanbanColIdx < len(m.columns) {
			colCards := m.cardsForColumn(m.columns[m.kanbanColIdx].ID)
			if m.kanbanCardIdx < len(colCards)-1 {
				m.kanbanCardIdx++
			}
		}

	case "enter":
		if m.kanbanColIdx < len(m.columns) {
			colCards := m.cardsForColumn(m.columns[m.kanbanColIdx].ID)
			if m.kanbanCardIdx < len(colCards) {
				return m.openDetail(colCards[m.kanbanCardIdx].ID)
			}
		}

	case "shift+left":
		return m.kanbanMoveCard(-1)

	case "shift+right":
		return m.kanbanMoveCard(1)

	case "b":
		if m.kanbanColIdx < len(m.columns) {
			colCards := m.cardsForColumn(m.columns[m.kanbanColIdx].ID)
			if len(colCards) > 0 {
				return m.openPicker(EdgeBlocks)
			}
		}

	case "s":
		if m.kanbanColIdx < len(m.columns) {
			colCards := m.cardsForColumn(m.columns[m.kanbanColIdx].ID)
			if len(colCards) > 0 {
				return m.openPicker(EdgeParentOf)
			}
		}

	case "?":
		m.overlay = overlayHelp
	}

	return m, nil
}

// kanbanMoveCard moves the focused card one column in the given direction
// (dir=-1 for left, dir=+1 for right), updates the DB, reloads, and
// repositions the kanban cursor at the moved card.
func (m Model) kanbanMoveCard(dir int) (tea.Model, tea.Cmd) {
	if len(m.columns) == 0 {
		return m, nil
	}
	targetIdx := m.kanbanColIdx + dir
	if targetIdx < 0 || targetIdx >= len(m.columns) {
		return m, nil
	}
	colCards := m.cardsForColumn(m.columns[m.kanbanColIdx].ID)
	if len(colCards) == 0 || m.kanbanCardIdx >= len(colCards) {
		return m, nil
	}

	moved := colCards[m.kanbanCardIdx]
	targetColID := m.columns[targetIdx].ID

	if err := m.db.MoveCardToColumn(moved.ID, targetColID); err != nil {
		return m, nil
	}

	m.kanbanColIdx = targetIdx
	m = m.loadCards()

	// MoveCardToColumn appends to the end; position cursor there.
	newColCards := m.cardsForColumn(targetColID)
	m.kanbanCardIdx = len(newColCards) - 1
	if m.kanbanCardIdx < 0 {
		m.kanbanCardIdx = 0
	}

	return m, nil
}

// ── Dependency picker ─────────────────────────────────────────────────────────

func (m Model) openPicker(edgeType EdgeType) (tea.Model, tea.Cmd) {
	if len(m.boards) == 0 {
		return m, nil
	}

	focusID, focusTitle, ok := m.focusedCardID()
	if !ok {
		return m, nil
	}

	// All non-archived cards on the board with meta (for display indicators).
	allCards, err := m.db.GetCardsWithMeta(m.boards[m.boardIdx].ID, false)
	if err != nil {
		return m, nil
	}

	// Current directed edges FROM the focused card.
	var existing []Card
	switch edgeType {
	case EdgeBlocks:
		existing, _ = m.db.GetBlocking(focusID)
	case EdgeParentOf:
		existing, _ = m.db.GetChildren(focusID)
	}
	existingSet := make(map[string]bool, len(existing))
	for _, c := range existing {
		existingSet[c.ID] = true
	}

	var items []pickerItem
	for _, cr := range allCards {
		if cr.ID == focusID {
			continue
		}
		items = append(items, pickerItem{card: cr, checked: existingSet[cr.ID]})
	}

	m.pickerEdgeType = edgeType
	m.pickerFocusID = focusID
	m.pickerFocusTitle = focusTitle
	m.pickerItems = items
	m.pickerIdx = 0
	m.pickerErr = ""
	m.overlay = overlayPicker
	return m, nil
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.overlay = overlayNone
		if m.detailView {
			m = m.refreshDetail()
		}
		m = m.loadCards()
		return m, nil

	case "up", "k":
		if m.pickerIdx > 0 {
			m.pickerIdx--
			m.pickerErr = ""
		}

	case "down", "j":
		if m.pickerIdx < len(m.pickerItems)-1 {
			m.pickerIdx++
			m.pickerErr = ""
		}

	case " ", "enter":
		if m.pickerIdx >= len(m.pickerItems) {
			break
		}
		m.pickerErr = ""
		item := m.pickerItems[m.pickerIdx]
		if item.checked {
			_ = m.db.DeleteEdge(m.pickerFocusID, item.card.ID, m.pickerEdgeType)
			m.pickerItems[m.pickerIdx].checked = false
		} else {
			if err := m.db.AddEdge(m.pickerFocusID, item.card.ID, m.pickerEdgeType); err != nil {
				m.pickerErr = "would create a cycle"
			} else {
				m.pickerItems[m.pickerIdx].checked = true
			}
		}
	}
	return m, nil
}

// ── Commit input ──────────────────────────────────────────────────────────────

func (m Model) commitInput(value string) Model {
	switch m.inputAction {
	case inputNewBoard:
		board, err := m.db.CreateBoard(value)
		if err != nil {
			return m
		}
		// Seed the new board with default columns.
		_, _ = m.db.CreateColumn(board.ID, "To Do")
		_, _ = m.db.CreateColumn(board.ID, "In Progress")
		_, _ = m.db.CreateColumn(board.ID, "Done")
		m.boards, _ = m.db.GetBoards()
		for i, b := range m.boards {
			if b.ID == board.ID {
				m = m.loadBoard(i)
				break
			}
		}

	case inputNewCard:
		colID := m.columnForNewCard()
		if colID == "" || len(m.boards) == 0 {
			return m
		}
		card, err := m.db.CreateCard(m.boards[m.boardIdx].ID, colID, value)
		if err != nil {
			return m
		}
		m = m.loadCards()
		for i, c := range m.cards {
			if c.ID == card.ID {
				m.cardIdx = i
				break
			}
		}

	case inputRenameBoard:
		if len(m.boards) == 0 {
			return m
		}
		_ = m.db.RenameBoard(m.boards[m.boardIdx].ID, value)
		m.boards, _ = m.db.GetBoards()

	case inputRenameCard:
		if m.overlayCardID == "" {
			return m
		}
		_ = m.db.RenameCard(m.overlayCardID, value)
		if m.detailView {
			m = m.refreshDetail()
		}
		m = m.loadCards()
	}
	return m
}
