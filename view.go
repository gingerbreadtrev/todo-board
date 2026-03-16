package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// leftInnerW is the content width of the board panel (excluding border).
const leftInnerW = 26

var (
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 3)

	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// dotStyles renders a priority bullet in the appropriate colour.
	dotStyles = map[Priority]string{
		PriorityLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("●"),
		PriorityMedium:   lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("●"),
		PriorityHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("●"),
		PriorityCritical: lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("●"),
	}

	blockedTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	parentTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	doneTextStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	archiveTagStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("130")).Bold(true)
	colHeaderStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true).Underline(true)
	indicatorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusBarStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	warnStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	checkedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("76")).Bold(true)
)

// ── Top-level View ────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	switch m.overlay {
	case overlayInput:
		return m.renderInputOverlay()
	case overlayConfirm:
		return m.renderConfirmOverlay()
	case overlayPriority:
		return m.renderPriorityOverlay()
	case overlayPicker:
		return m.renderPickerOverlay()
	case overlayDescEdit:
		return m.renderDescEditOverlay()
	case overlayHelp:
		return m.renderHelpOverlay()
	case overlayColumnPicker:
		return m.renderColumnPickerOverlay()
	}

	if m.detailView {
		return m.renderDetailView()
	}

	if m.kanbanView {
		return m.renderKanban()
	}

	// innerH = total height minus top/bottom borders (2) and status bar (1).
	innerH := m.height - 3
	if innerH < 1 {
		innerH = 1
	}

	// rightInnerW fills the remainder: total width minus left panel total (leftInnerW+2)
	// and right panel's own border (2).
	rightInnerW := m.width - leftInnerW - 4
	if rightInnerW < 10 {
		rightInnerW = 10
	}

	titleH := innerH - 2
	if titleH < 1 {
		titleH = 1
	}
	leftContent := boldStyle.Render("Boards") + "\n\n" + m.renderBoardList(leftInnerW, titleH)
	rightContent := boldStyle.Render("Cards") + "\n\n" + m.renderCardList(rightInnerW, titleH)

	var leftStyle, rightStyle lipgloss.Style
	if m.focus == focusBoards {
		leftStyle = focusedBorderStyle
		rightStyle = unfocusedBorderStyle
	} else {
		leftStyle = unfocusedBorderStyle
		rightStyle = focusedBorderStyle
	}

	leftPanel := leftStyle.Width(leftInnerW).Height(innerH).Render(leftContent)
	rightPanel := rightStyle.Width(rightInnerW).Height(innerH).Render(rightContent)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	return lipgloss.JoinVertical(lipgloss.Left, panels, m.renderStatusBar())
}

// ── Overlays ──────────────────────────────────────────────────────────────────

func (m Model) renderInputOverlay() string {
	var title string
	switch m.inputAction {
	case inputNewBoard:
		title = "New board"
	case inputNewCard:
		title = "New card"
	case inputRenameBoard:
		title = "Rename board"
	case inputRenameCard:
		title = "Rename card"
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		boldStyle.Render(title),
		"",
		m.inputField.View(),
		"",
		dimStyle.Render("enter to confirm  ·  esc to cancel"),
	)
	popup := popupStyle.Width(52).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

func (m Model) renderConfirmOverlay() string {
	var title, question string
	switch m.confirmAction {
	case confirmDeleteBoard:
		title = "Delete board"
		question = fmt.Sprintf("Delete board %q and all its cards?", truncate(m.confirmTarget, 40))
	default:
		title = "Archive card"
		question = fmt.Sprintf("Archive %q?", truncate(m.confirmTarget, 48))
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		boldStyle.Render(title),
		"",
		question,
		"",
		dimStyle.Render("enter / y to confirm  ·  esc / n to cancel"),
	)
	popup := popupStyle.Width(56).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

func (m Model) renderPriorityOverlay() string {
	labels := []string{"low", "medium", "high", "critical"}
	var rows []string
	rows = append(rows, boldStyle.Render("Set priority"), "")
	for i, p := range priorityOptions {
		cursor := "  "
		if i == m.priorityIdx {
			cursor = cursorStyle.Render("▶ ")
		}
		dot := dotStyles[p]
		label := labels[i]
		if i == m.priorityIdx {
			label = boldStyle.Render(label)
		}
		rows = append(rows, cursor+dot+"  "+label)
	}
	rows = append(rows, "", dimStyle.Render("↑/↓ k/j: move  ·  enter: select  ·  esc: cancel"))
	popup := popupStyle.Width(36).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

func (m Model) renderPickerOverlay() string {
	const innerW = 58
	const maxVisible = 10

	var title, relation string
	switch m.pickerEdgeType {
	case EdgeBlocks:
		title = "Manage blockers"
		relation = "blocks →"
	case EdgeParentOf:
		title = "Manage children"
		relation = "is parent of →"
	}

	subtitle := fmt.Sprintf("%q %s", truncate(m.pickerFocusTitle, 32), relation)

	var rows []string
	rows = append(rows, boldStyle.Render(title))
	rows = append(rows, dimStyle.Render(subtitle))
	rows = append(rows, "")

	total := len(m.pickerItems)
	if total == 0 {
		rows = append(rows, dimStyle.Render("No other cards on this board."))
	} else {
		// Compute scroll window centred on pickerIdx.
		visible := maxVisible
		if visible > total {
			visible = total
		}
		start := m.pickerIdx - visible/2
		if start < 0 {
			start = 0
		}
		end := start + visible
		if end > total {
			end = total
			start = end - visible
			if start < 0 {
				start = 0
			}
		}

		// Title column budget: innerW - cursor(2) - checkbox(3) - sp(1) - dot(1) - sp(1)
		titleW := innerW - 8
		if titleW < 8 {
			titleW = 8
		}

		for i := start; i < end; i++ {
			item := m.pickerItems[i]
			cursor := "  "
			if i == m.pickerIdx {
				cursor = cursorStyle.Render("▶ ")
			}
			check := "[ ]"
			if item.checked {
				check = checkedStyle.Render("[x]")
			}
			dot := dotStyles[item.card.Priority]
			itemTitle := truncate(item.card.Title, titleW)
			rows = append(rows, cursor+check+" "+dot+" "+itemTitle)
		}

		if total > maxVisible {
			rows = append(rows, dimStyle.Render(fmt.Sprintf("  %d / %d", m.pickerIdx+1, total)))
		}
	}

	if m.pickerErr != "" {
		rows = append(rows, "", warnStyle.Render("⚠  "+m.pickerErr))
	}
	rows = append(rows, "", dimStyle.Render("space / enter: toggle  ·  esc: done"))

	popup := popupStyle.Width(innerW).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

// ── Description edit overlay ──────────────────────────────────────────────────

func (m Model) renderDescEditOverlay() string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		boldStyle.Render("Edit description"),
		"",
		m.descInput.View(),
		"",
		dimStyle.Render("ctrl+s: save  ·  esc: cancel"),
	)
	popupW := m.width*3/4
	if popupW < 44 {
		popupW = 44
	}
	popup := popupStyle.Width(popupW).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

// ── Column picker overlay ─────────────────────────────────────────────────────

func (m Model) renderColumnPickerOverlay() string {
	var rows []string
	rows = append(rows, boldStyle.Render("Move to phase"), "")
	for i, col := range m.columns {
		cursor := "  "
		if i == m.colPickerIdx {
			cursor = cursorStyle.Render("▶ ")
		}
		label := col.Name
		if i == m.colPickerIdx {
			label = boldStyle.Render(col.Name)
		}
		if col.ID == m.detailCard.ColumnID {
			label += dimStyle.Render("  (current)")
		}
		rows = append(rows, cursor+label)
	}
	rows = append(rows, "", dimStyle.Render("↑/↓ k/j: move  ·  enter: select  ·  esc: cancel"))
	popup := popupStyle.Width(40).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

// ── Detail view ───────────────────────────────────────────────────────────────

const detailRelAreaH = 9 // lines reserved for the 4 relation tables at the bottom

func (m Model) renderDetailView() string {
	totalW := m.width
	totalH := m.height - 1 // minus status bar
	if totalW < 20 {
		totalW = 20
	}
	if totalH < 8 {
		totalH = 8
	}

	card := m.detailCard
	sep := dimStyle.Render(strings.Repeat("─", totalW))

	// ── Title line ────────────────────────────────────────────────────────
	dot := dotStyles[card.Priority]
	archiveTag := ""
	if card.Archived {
		archiveTag = "  " + archiveTagStyle.Render("[archived]")
	}
	stackHint := ""
	if len(m.detailStack) > 0 {
		stackHint = dimStyle.Render(fmt.Sprintf("  (%d deep)", len(m.detailStack)))
	}
	titleLine := lipgloss.NewStyle().Width(totalW).Align(lipgloss.Center).
		Render(dot + "  " + boldStyle.Render(truncate(card.Title, totalW-12)) + archiveTag + stackHint)

	// ── Fixed overhead: title(1) sep(1) props(4) sep(1) sep(1) rels(detailRelAreaH) ──
	const propsLines = 4
	const fixedOverhead = 1 + 1 + propsLines + 1 + 1 + detailRelAreaH
	descH := totalH - fixedOverhead
	if descH < 2 {
		descH = 2
	}

	var lines []string
	lines = append(lines, titleLine, sep)

	// ── Properties ────────────────────────────────────────────────────────
	lines = append(lines, m.renderDetailProps(totalW, m.detailSection == detailAreaProps)...)
	lines = append(lines, sep)

	// ── Description ───────────────────────────────────────────────────────
	descLines := m.renderDetailDesc(totalW, descH, m.detailSection == detailAreaDesc)
	for len(descLines) < descH {
		descLines = append(descLines, "")
	}
	lines = append(lines, descLines[:descH]...)
	lines = append(lines, sep)

	// ── Relation tables ───────────────────────────────────────────────────
	relLines := m.renderDetailRels(totalW, detailRelAreaH)
	for len(relLines) < detailRelAreaH {
		relLines = append(relLines, "")
	}
	lines = append(lines, relLines[:detailRelAreaH]...)

	return strings.Join(lines, "\n") + "\n" + m.renderStatusBar()
}

// renderDetailProps renders 4 property rows (no trailing blank).
// When focused is true the active row shows a cursor.
func (m Model) renderDetailProps(w int, focused bool) []string {
	card := m.detailCard
	const labelW = 12
	props := []struct {
		label string
		value string
		hint  string
	}{
		{"Title", truncate(card.Title, w-labelW-20), "enter to edit"},
		{"Phase", m.columnName(card.ColumnID), "enter to change"},
		{"Priority", dotStyles[card.Priority] + " " + string(card.Priority), "enter to change"},
		{"Status", string(card.Status), "enter to toggle"},
	}

	var lines []string
	for i, p := range props {
		val := p.value
		if i == 3 && card.Status == StatusDone {
			val = doneTextStyle.Render(val)
		}
		if focused && i == m.detailRowIdx {
			lines = append(lines, cursorStyle.Render("▶ ")+
				boldStyle.Render(fmt.Sprintf("%-*s", labelW, p.label))+
				val+
				"  "+dimStyle.Render(p.hint))
		} else {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("  %-*s", labelW, p.label))+val)
		}
	}
	return lines
}

// renderDetailDesc renders the scrollable description area.
// When focused is true, show a scroll/edit hint.
func (m Model) renderDetailDesc(w, h int, focused bool) []string {
	desc := strings.TrimRight(m.detailCard.Description, "\n")
	var allLines []string
	if desc == "" {
		placeholder := "  (no description)"
		if focused {
			placeholder = "  (no description — press enter to add)"
		}
		allLines = []string{dimStyle.Render(placeholder)}
	} else {
		for _, line := range strings.Split(desc, "\n") {
			allLines = append(allLines, "  "+line)
		}
	}

	maxScroll := len(allLines) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.detailDescScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + h
	if end > len(allLines) {
		end = len(allLines)
	}
	visible := make([]string, end-scroll)
	copy(visible, allLines[scroll:end])

	// Last line: scroll indicator or edit hint.
	if focused {
		var hint string
		if len(allLines) > h {
			pct := 0
			if maxScroll > 0 {
				pct = scroll * 100 / maxScroll
			}
			hint = dimStyle.Render(fmt.Sprintf("  ─── %d%% ───  enter: edit", pct))
		} else {
			hint = dimStyle.Render("  enter: edit")
		}
		if len(visible) > 0 {
			visible[len(visible)-1] = hint
		} else {
			visible = append(visible, hint)
		}
	}

	return visible
}

// renderDetailRels renders 4 relation tables side by side in h lines.
func (m Model) renderDetailRels(w, h int) []string {
	type relCol struct {
		label  string
		cards  []Card
		hasAdd bool
	}
	cols := []relCol{
		{"Parents", m.detailParents, false},
		{"Children", m.detailChildren, true},
		{"Blocking", m.detailBlocking, true},
		{"Blocked by", m.detailBlockers, false},
	}
	n := len(cols)
	// Divide width evenly; last column absorbs remainder.
	baseColW := (w - (n - 1)) / n // -3 for vertical separators between 4 cols
	colWidths := make([]int, n)
	usedW := 0
	for i := 0; i < n-1; i++ {
		colWidths[i] = baseColW
		usedW += baseColW
	}
	colWidths[n-1] = w - usedW - (n - 1)
	if colWidths[n-1] < 4 {
		colWidths[n-1] = 4
	}

	// Render each column into its own line slice.
	rendered := make([][]string, n)
	for i, col := range cols {
		relFocused := m.detailSection == detailAreaRels && m.detailRelCol == i
		rendered[i] = m.renderRelCol(col.label, col.cards, col.hasAdd, relFocused, colWidths[i], h)
	}

	// Zip columns together with │ separators.
	lines := make([]string, h)
	for row := 0; row < h; row++ {
		var parts []string
		for ci := 0; ci < n; ci++ {
			var cell string
			if row < len(rendered[ci]) {
				cell = rendered[ci][row]
			}
			// Pad to exact column width (ANSI-aware).
			cw := lipgloss.Width(cell)
			if cw < colWidths[ci] {
				cell += strings.Repeat(" ", colWidths[ci]-cw)
			}
			parts = append(parts, cell)
		}
		lines[row] = strings.Join(parts, dimStyle.Render("│"))
	}
	return lines
}

// renderRelCol renders one relation table column into exactly h lines.
func (m Model) renderRelCol(label string, cards []Card, hasAdd, focused bool, w, h int) []string {
	var lines []string

	// Header.
	hdrLabel := truncate(label, w-1)
	if focused {
		lines = append(lines, " "+lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true).Underline(true).Render(hdrLabel))
	} else {
		lines = append(lines, " "+dimStyle.Render(hdrLabel))
	}
	lines = append(lines, dimStyle.Render(" "+strings.Repeat("─", w-1)))

	// Card rows area: h minus header(1) sep(1) and optionally add-row(1).
	cardAreaH := h - 2
	if hasAdd {
		cardAreaH--
	}
	if cardAreaH < 0 {
		cardAreaH = 0
	}

	// Scroll to keep focused row visible.
	scroll := 0
	if focused && m.detailRowIdx < len(cards) && len(cards) > cardAreaH {
		scroll = m.detailRowIdx - cardAreaH/2
		if scroll < 0 {
			scroll = 0
		}
		if scroll+cardAreaH > len(cards) {
			scroll = len(cards) - cardAreaH
		}
	}

	// Render card rows.
	displayed := 0
	for i := scroll; i < len(cards) && displayed < cardAreaH; i++ {
		c := cards[i]
		isFocused := focused && i == m.detailRowIdx
		dot := dotStyles[c.Priority]
		// Budget: cursor(2) + dot(1) + space(1) = 4
		title := truncate(c.Title, w-4)
		var row string
		if isFocused {
			row = cursorStyle.Render("▶ ") + dot + " " + title
		} else {
			row = "  " + dot + " " + title
		}
		lines = append(lines, row)
		displayed++
	}
	// Empty state.
	if len(cards) == 0 && !hasAdd {
		lines = append(lines, dimStyle.Render("  (none)"))
		displayed++
	}
	// Pad remaining card slots.
	for displayed < cardAreaH {
		lines = append(lines, "")
		displayed++
	}

	// Add row.
	if hasAdd {
		addIdx := len(cards)
		if focused && m.detailRowIdx == addIdx {
			lines = append(lines, cursorStyle.Render("▶ ")+dimStyle.Render("Add…"))
		} else {
			lines = append(lines, "  "+dimStyle.Render("+ Add…"))
		}
	}

	return lines
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func (m Model) renderHelpOverlay() string {
	var lines []string

	add := func(key, desc string) {
		lines = append(lines, fmt.Sprintf("  %-22s %s", key, desc))
	}

	if m.detailView {
		lines = append(lines, boldStyle.Render("Card detail"))
		lines = append(lines, "")
		add("tab / shift+tab", "switch area (props/desc/rels)")
		add("↑/↓  k/j", "navigate rows / scroll desc")
		add("←/→  h/l", "switch relation table (in rels)")
		add("enter", "edit field / open card / add")
		add("x", "remove focused relationship")
		add("d", "archive card")
		add("esc", "back (or pop drill-in stack)")
	} else if m.kanbanView {
		lines = append(lines, boldStyle.Render("Kanban view"))
		lines = append(lines, "")
		add("←/→  h/l", "switch column")
		add("↑/↓  k/j", "navigate cards")
		add("enter", "open card detail")
		add("shift+←/→", "move card to adjacent column")
		add("b", "manage blockers")
		add("s", "manage children")
		add("esc / shift+tab", "return to list view")
	} else if m.archiveView {
		lines = append(lines, boldStyle.Render("Archive view"))
		lines = append(lines, "")
		add("↑/↓  k/j", "navigate")
		add("r", "restore card")
		add("d  d", "delete permanently")
		add("D", "exit archive view")
		add("tab", "switch panels")
	} else {
		lines = append(lines, boldStyle.Render("Board panel"))
		lines = append(lines, "")
		add("↑/↓  k/j", "navigate boards")
		add("enter", "select board")
		add("n", "new board")
		add("r", "rename board")
		add("d", "delete board")
		lines = append(lines, "")
		lines = append(lines, boldStyle.Render("Card panel"))
		lines = append(lines, "")
		add("↑/↓  k/j", "navigate cards")
		add("enter", "open card detail")
		add("n", "new card")
		add("r", "rename card")
		add("e", "edit description")
		add("c", "toggle done")
		add("p", "set priority")
		add("←/→  h/l", "move to prev/next column")
		add("d", "archive card")
		add("D", "toggle archive view")
		add("b", "manage blockers")
		add("s", "manage children")
		add("/", "search")
		add("V / shift+tab", "toggle kanban view")
		add("tab", "switch panels")
	}

	lines = append(lines, "")
	add("q / ctrl+c", "quit")
	add("?", "close help")

	popup := popupStyle.Width(52).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

// ── Kanban view ───────────────────────────────────────────────────────────────

func (m Model) renderKanban() string {
	if len(m.columns) == 0 {
		msg := dimStyle.Render("No columns on this board.")
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, msg),
			m.renderStatusBar(),
		)
	}

	n := len(m.columns)
	// Each column box occupies the full height minus the status bar.
	boxH := m.height - 1 // outer height including border
	innerH := boxH - 2   // content area height

	// Distribute width: base width for all columns, last column takes remainder.
	baseW := m.width / n

	colPanels := make([]string, n)
	for ci, col := range m.columns {
		colW := baseW
		if ci == n-1 {
			colW = m.width - baseW*(n-1)
		}
		innerW := colW - 2
		if innerW < 4 {
			innerW = 4
		}

		content := m.renderKanbanColumn(col, ci, innerW, innerH)

		var style lipgloss.Style
		if ci == m.kanbanColIdx {
			style = focusedBorderStyle
		} else {
			style = unfocusedBorderStyle
		}
		colPanels[ci] = style.Width(innerW).Height(innerH).Render(content)
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, colPanels...)
	return lipgloss.JoinVertical(lipgloss.Left, board, m.renderStatusBar())
}

func (m Model) renderKanbanColumn(col Column, colIdx, innerW, innerH int) string {
	header := boldStyle.Render(truncate(col.Name, innerW))
	separator := strings.Repeat("─", innerW)

	colCards := m.cardsForColumn(col.ID)

	// Lines available for cards after the header (1) and separator (1).
	cardsH := innerH - 2
	if cardsH < 0 {
		cardsH = 0
	}

	var cardLines []string
	selectedLine := 0
	for i, cr := range colCards {
		selected := colIdx == m.kanbanColIdx && i == m.kanbanCardIdx
		if selected {
			selectedLine = len(cardLines)
		}
		cardLines = append(cardLines, renderCard(cr, selected, innerW))
	}

	// Scroll to keep the selected card visible.
	if len(cardLines) > cardsH {
		start := selectedLine - cardsH/2
		if start < 0 {
			start = 0
		}
		end := start + cardsH
		if end > len(cardLines) {
			end = len(cardLines)
			start = end - cardsH
			if start < 0 {
				start = 0
			}
		}
		cardLines = cardLines[start:end]
	}

	lines := []string{header, separator}
	if len(colCards) == 0 {
		lines = append(lines, dimStyle.Render("  (empty)"))
	} else {
		lines = append(lines, cardLines...)
	}
	return strings.Join(lines, "\n")
}

// ── Board panel ───────────────────────────────────────────────────────────────

func (m Model) renderBoardList(w, h int) string {
	if len(m.boards) == 0 {
		return dimStyle.Render("No boards yet.\nPress n to create.")
	}

	var lines []string
	for i, b := range m.boards {
		cursor := "  "
		name := truncate(b.Name, w-2)
		if i == m.boardIdx {
			cursor = cursorStyle.Render("▶ ")
			name = boldStyle.Render(name)
		}
		lines = append(lines, cursor+name)
	}

	return strings.Join(lines, "\n")
}

// ── Card panel ────────────────────────────────────────────────────────────────

// displayItem is a row in the rendered card list — a column header, a blank
// spacer, or a reference to a card by its index in m.cards.
type displayItem struct {
	isHeader bool
	isSpacer bool
	header   string
	cardIdx  int
}

func (m Model) buildDisplayItems(cards []CardRow) []displayItem {
	var items []displayItem
	currentColID := ""

	colName := func(id string) string {
		for _, col := range m.columns {
			if col.ID == id {
				return col.Name
			}
		}
		return id
	}

	for i, cr := range cards {
		if cr.ColumnID != currentColID {
			// Suppress column headers while a search filter is active — the
			// matching cards may span columns and headers add noise.
			if m.searchQuery == "" {
				if currentColID != "" {
					// Blank line between sections (not before the very first header).
					items = append(items, displayItem{isSpacer: true})
				}
				items = append(items, displayItem{isHeader: true, header: colName(cr.ColumnID)})
			}
			currentColID = cr.ColumnID
		}
		items = append(items, displayItem{cardIdx: i})
	}
	return items
}

func (m Model) renderCardList(w, h int) string {
	if len(m.boards) == 0 {
		return dimStyle.Render("Select a board.")
	}

	// Reserve lines for fixed UI elements at the top/bottom of the panel.
	bannerLine := ""
	if m.archiveView {
		bannerLine = archiveTagStyle.Render("  ⌂ archive") +
			dimStyle.Render("  r: restore  dd: delete permanently  D: back")
		h--
	}

	searchLine := ""
	if m.searchMode {
		query := m.searchQuery + "▌"
		searchLine = dimStyle.Render("  / ") + query
		h--
	}

	visible := m.visibleCards()

	if len(visible) == 0 {
		var msg string
		switch {
		case m.searchQuery != "":
			msg = dimStyle.Render(fmt.Sprintf("  No cards match %q.", m.searchQuery))
		case m.archiveView:
			msg = dimStyle.Render("No archived cards.")
		default:
			msg = dimStyle.Render("No cards on this board.")
		}
		return joinNonEmpty("\n", bannerLine, searchLine, msg)
	}

	items := m.buildDisplayItems(visible)

	// Build all rendered lines, tracking which display line the selected card is on.
	lines := make([]string, 0, len(items))
	selectedLine := 0
	for _, item := range items {
		if item.isSpacer {
			lines = append(lines, "")
		} else if item.isHeader {
			lines = append(lines, "  "+colHeaderStyle.Render(item.header))
		} else {
			cr := visible[item.cardIdx]
			selected := item.cardIdx == m.cardIdx
			if selected {
				selectedLine = len(lines)
			}
			lines = append(lines, renderCard(cr, selected, w))
		}
	}

	// Scroll so the selected line stays visible.
	if len(lines) > h {
		start := selectedLine - h/2
		if start < 0 {
			start = 0
		}
		end := start + h
		if end > len(lines) {
			end = len(lines)
			start = end - h
			if start < 0 {
				start = 0
			}
		}
		lines = lines[start:end]
	}

	return joinNonEmpty("\n", bannerLine, strings.Join(lines, "\n"), searchLine)
}

func renderCard(cr CardRow, selected bool, w int) string {
	dot := dotStyles[cr.Priority]

	// Build indicator suffix.
	var inds []string
	if cr.HasParent {
		inds = append(inds, "↑")
	}
	if cr.ChildrenCount > 0 {
		inds = append(inds, fmt.Sprintf("↓%d", cr.ChildrenCount))
	}
	if cr.IsBlocked {
		inds = append(inds, "⊘ blocked")
	}
	if cr.BlockingCount > 0 {
		inds = append(inds, fmt.Sprintf("⊘%d", cr.BlockingCount))
	}

	indicatorStr := ""
	if len(inds) > 0 {
		indicatorStr = indicatorStyle.Render(" " + strings.Join(inds, " "))
	}

	// cursor=2  dot=1  space=1  title  indicators
	indW := lipgloss.Width(indicatorStr)
	titleMaxW := w - 2 - 1 - 1 - indW
	if titleMaxW < 4 {
		titleMaxW = 4
	}
	title := truncate(cr.Title, titleMaxW)

	var titleStyled string
	switch {
	case cr.IsBlocked:
		titleStyled = blockedTextStyle.Render(title)
	case cr.ChildrenCount > 0:
		titleStyled = parentTextStyle.Render(title)
	case cr.Status == StatusDone:
		titleStyled = doneTextStyle.Render(title)
	default:
		titleStyled = title
	}

	cursor := "  "
	if selected {
		cursor = cursorStyle.Render("▶ ")
	}

	return cursor + dot + " " + titleStyled + indicatorStr
}

// ── Status bar ────────────────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	var hint string
	switch {
	case m.detailView:
		hint = dimStyle.Render(" [detail]  tab: area  k/j: rows  h/l: rel table  enter: edit/open  x: remove  d: archive  esc: back  ?: help  q: quit")
	case m.kanbanView:
		hint = dimStyle.Render(" [kanban]  ←/→ h/l: column  ↑/↓ k/j: navigate  enter: detail  shift+←/→: move  b: blockers  s: children  esc: back  ?: help  q: quit")
	case m.pendingD:
		hint = warnStyle.Render(" d again to delete permanently") +
			dimStyle.Render("  ·  any other key to cancel")
	case m.archiveView:
		hint = archiveTagStyle.Render(" [archive]") +
			dimStyle.Render("  r: restore  dd: delete  D: exit archive  tab: switch  q: quit")
	case m.searchMode:
		hint = dimStyle.Render(" [search]  type to filter  ↑/↓: navigate  esc: clear")
	case m.focus == focusBoards:
		hint = dimStyle.Render(" [boards]  n: new  r: rename  d: delete  ↑/↓ k/j: navigate  enter: select  tab: switch  q: quit")
	default:
		hint = dimStyle.Render(" [cards]  n: new  r: rename  e: edit  c: done  p: priority  ←/→ h/l: move col  d: archive  b: blockers  s: children  /: search  V: kanban  ?: help  q: quit")
	}
	return statusBarStyle.Width(m.width).Render(hint)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// joinNonEmpty joins non-empty strings with sep, skipping empty ones.
func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// truncate clips s to at most maxW runes, appending "…" if it was longer.
func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	return string(runes[:maxW-1]) + "…"
}
