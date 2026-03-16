package main

import (
	"database/sql"
	"os"
	"testing"
)

// testDB is the shared in-memory database for all tests in this file.
var testDB *DB

// TestMain sets up a single in-memory SQLite database, runs all tests, and
// tears down. Each Test* function creates its own boards/columns/cards so
// tests are independent.
func TestMain(m *testing.M) {
	db, err := OpenDB(":memory:")
	if err != nil {
		panic("open test db: " + err.Error())
	}
	testDB = db
	code := m.Run()
	db.Close()
	os.Exit(code)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// mustBoard creates a board or fatally fails the test.
func mustBoard(t *testing.T, name string) Board {
	t.Helper()
	b, err := testDB.CreateBoard(name)
	if err != nil {
		t.Fatalf("CreateBoard(%q): %v", name, err)
	}
	return b
}

// mustColumn creates a column or fatally fails the test.
func mustColumn(t *testing.T, boardID, name string) Column {
	t.Helper()
	c, err := testDB.CreateColumn(boardID, name)
	if err != nil {
		t.Fatalf("CreateColumn(%q): %v", name, err)
	}
	return c
}

// mustCard creates a card or fatally fails the test.
func mustCard(t *testing.T, boardID, columnID, title string) Card {
	t.Helper()
	c, err := testDB.CreateCard(boardID, columnID, title)
	if err != nil {
		t.Fatalf("CreateCard(%q): %v", title, err)
	}
	return c
}

// mustEdge adds an edge or fatally fails the test.
func mustEdge(t *testing.T, src, tgt string, et EdgeType) {
	t.Helper()
	if err := testDB.AddEdge(src, tgt, et); err != nil {
		t.Fatalf("AddEdge(%s→%s, %s): %v", src, tgt, et, err)
	}
}

// ── Boards ────────────────────────────────────────────────────────────────────

func TestBoards_CreateAndGet(t *testing.T) {
	b := mustBoard(t, "Alpha")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	if b.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if b.Name != "Alpha" {
		t.Fatalf("Name = %q, want %q", b.Name, "Alpha")
	}
	if b.CreatedAt.IsZero() || b.UpdatedAt.IsZero() {
		t.Fatal("timestamps not set")
	}

	got, err := testDB.GetBoard(b.ID)
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	if got.Name != "Alpha" {
		t.Fatalf("GetBoard.Name = %q, want %q", got.Name, "Alpha")
	}
}

func TestBoards_GetBoards(t *testing.T) {
	b1 := mustBoard(t, "List-1")
	b2 := mustBoard(t, "List-2")
	defer testDB.DeleteBoard(b1.ID) //nolint:errcheck
	defer testDB.DeleteBoard(b2.ID) //nolint:errcheck

	boards, err := testDB.GetBoards()
	if err != nil {
		t.Fatalf("GetBoards: %v", err)
	}
	ids := map[string]bool{}
	for _, brd := range boards {
		ids[brd.ID] = true
	}
	if !ids[b1.ID] || !ids[b2.ID] {
		t.Fatal("one or both boards missing from GetBoards")
	}
}

func TestBoards_Rename(t *testing.T) {
	b := mustBoard(t, "Original")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	if err := testDB.RenameBoard(b.ID, "Renamed"); err != nil {
		t.Fatalf("RenameBoard: %v", err)
	}
	got, _ := testDB.GetBoard(b.ID)
	if got.Name != "Renamed" {
		t.Fatalf("after rename: Name = %q, want %q", got.Name, "Renamed")
	}
}

func TestBoards_Delete(t *testing.T) {
	b := mustBoard(t, "ToDelete")
	if err := testDB.DeleteBoard(b.ID); err != nil {
		t.Fatalf("DeleteBoard: %v", err)
	}
	_, err := testDB.GetBoard(b.ID)
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows after delete, got %v", err)
	}
}

func TestBoards_DeleteCascadesToColumnsAndCards(t *testing.T) {
	b := mustBoard(t, "Cascade")
	col := mustColumn(t, b.ID, "Todo")
	card := mustCard(t, b.ID, col.ID, "A card")

	if err := testDB.DeleteBoard(b.ID); err != nil {
		t.Fatalf("DeleteBoard: %v", err)
	}
	_, err := testDB.GetCard(card.ID)
	if err != sql.ErrNoRows {
		t.Fatalf("expected card to be cascade-deleted, got %v", err)
	}
}

// ── Columns ───────────────────────────────────────────────────────────────────

func TestColumns_CreatePositioning(t *testing.T) {
	b := mustBoard(t, "ColPos")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	c0 := mustColumn(t, b.ID, "First")
	c1 := mustColumn(t, b.ID, "Second")
	c2 := mustColumn(t, b.ID, "Third")

	if c0.Position != 0 {
		t.Fatalf("c0.Position = %d, want 0", c0.Position)
	}
	if c1.Position != 1 {
		t.Fatalf("c1.Position = %d, want 1", c1.Position)
	}
	if c2.Position != 2 {
		t.Fatalf("c2.Position = %d, want 2", c2.Position)
	}
}

func TestColumns_GetColumns(t *testing.T) {
	b := mustBoard(t, "GetCols")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	mustColumn(t, b.ID, "A")
	mustColumn(t, b.ID, "B")

	cols, err := testDB.GetColumns(b.ID)
	if err != nil {
		t.Fatalf("GetColumns: %v", err)
	}
	if len(cols) != 2 {
		t.Fatalf("len = %d, want 2", len(cols))
	}
	if cols[0].Name != "A" || cols[1].Name != "B" {
		t.Fatalf("unexpected order: %v", cols)
	}
}

func TestColumns_GetColumn(t *testing.T) {
	b := mustBoard(t, "GetCol")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	col := mustColumn(t, b.ID, "Specific")
	got, err := testDB.GetColumn(col.ID)
	if err != nil {
		t.Fatalf("GetColumn: %v", err)
	}
	if got.BoardID != b.ID {
		t.Fatalf("BoardID mismatch")
	}
	if got.Name != "Specific" {
		t.Fatalf("Name = %q, want %q", got.Name, "Specific")
	}
}

func TestColumns_Rename(t *testing.T) {
	b := mustBoard(t, "RenCol")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	col := mustColumn(t, b.ID, "Old")
	if err := testDB.RenameColumn(col.ID, "New"); err != nil {
		t.Fatalf("RenameColumn: %v", err)
	}
	got, _ := testDB.GetColumn(col.ID)
	if got.Name != "New" {
		t.Fatalf("Name = %q after rename, want %q", got.Name, "New")
	}
}

func TestColumns_Move(t *testing.T) {
	b := mustBoard(t, "MoveCol")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	col := mustColumn(t, b.ID, "C")
	if err := testDB.MoveColumn(col.ID, 99); err != nil {
		t.Fatalf("MoveColumn: %v", err)
	}
	got, _ := testDB.GetColumn(col.ID)
	if got.Position != 99 {
		t.Fatalf("Position = %d, want 99", got.Position)
	}
}

func TestColumns_Delete(t *testing.T) {
	b := mustBoard(t, "DelCol")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	col := mustColumn(t, b.ID, "Drop")
	if err := testDB.DeleteColumn(col.ID); err != nil {
		t.Fatalf("DeleteColumn: %v", err)
	}
	_, err := testDB.GetColumn(col.ID)
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestColumns_DeleteCascadesToCards(t *testing.T) {
	b := mustBoard(t, "ColCascade")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck

	col := mustColumn(t, b.ID, "Gone")
	card := mustCard(t, b.ID, col.ID, "Orphan")

	if err := testDB.DeleteColumn(col.ID); err != nil {
		t.Fatalf("DeleteColumn: %v", err)
	}
	_, err := testDB.GetCard(card.ID)
	if err != sql.ErrNoRows {
		t.Fatalf("expected card to be cascade-deleted, got %v", err)
	}
}

// ── Cards ─────────────────────────────────────────────────────────────────────

func TestCards_CreateAndGet(t *testing.T) {
	b := mustBoard(t, "CardCreate")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Todo")

	c := mustCard(t, b.ID, col.ID, "First")

	if c.Priority != PriorityLow {
		t.Fatalf("Priority = %q, want low", c.Priority)
	}
	if c.Status != StatusTodo {
		t.Fatalf("Status = %q, want todo", c.Status)
	}
	if c.Position != 0 {
		t.Fatalf("Position = %d, want 0", c.Position)
	}
	if c.Archived {
		t.Fatal("Archived should be false on create")
	}

	got, err := testDB.GetCard(c.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if got.Title != "First" {
		t.Fatalf("Title = %q, want %q", got.Title, "First")
	}
}

func TestCards_PositionAutoIncrement(t *testing.T) {
	b := mustBoard(t, "Pos")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	c0 := mustCard(t, b.ID, col.ID, "Zero")
	c1 := mustCard(t, b.ID, col.ID, "One")
	c2 := mustCard(t, b.ID, col.ID, "Two")

	if c0.Position != 0 || c1.Position != 1 || c2.Position != 2 {
		t.Fatalf("positions = %d,%d,%d; want 0,1,2", c0.Position, c1.Position, c2.Position)
	}
}

func TestCards_GetCards_ExcludesArchived(t *testing.T) {
	b := mustBoard(t, "GCArchive")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Todo")

	active := mustCard(t, b.ID, col.ID, "Active")
	archived := mustCard(t, b.ID, col.ID, "Archived")

	if err := testDB.ArchiveCard(archived.ID); err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}

	// Non-archived list should not include the archived card.
	cards, err := testDB.GetCards(b.ID, false)
	if err != nil {
		t.Fatalf("GetCards(false): %v", err)
	}
	for _, c := range cards {
		if c.ID == archived.ID {
			t.Fatal("archived card returned in non-archived list")
		}
	}

	// Full list must include both.
	cards, err = testDB.GetCards(b.ID, true)
	if err != nil {
		t.Fatalf("GetCards(true): %v", err)
	}
	ids := map[string]bool{}
	for _, c := range cards {
		ids[c.ID] = true
	}
	if !ids[active.ID] || !ids[archived.ID] {
		t.Fatal("full list missing expected cards")
	}
}

func TestCards_Rename(t *testing.T) {
	b := mustBoard(t, "Ren")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "Original")

	if err := testDB.RenameCard(c.ID, "Renamed"); err != nil {
		t.Fatalf("RenameCard: %v", err)
	}
	got, _ := testDB.GetCard(c.ID)
	if got.Title != "Renamed" {
		t.Fatalf("Title = %q, want %q", got.Title, "Renamed")
	}
}

func TestCards_SetDescription(t *testing.T) {
	b := mustBoard(t, "Desc")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "X")

	if err := testDB.SetCardDescription(c.ID, "Some description"); err != nil {
		t.Fatalf("SetCardDescription: %v", err)
	}
	got, _ := testDB.GetCard(c.ID)
	if got.Description != "Some description" {
		t.Fatalf("Description = %q", got.Description)
	}
}

func TestCards_SetPriority(t *testing.T) {
	b := mustBoard(t, "Prio")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "X")

	for _, p := range []Priority{PriorityMedium, PriorityHigh, PriorityCritical, PriorityLow} {
		if err := testDB.SetCardPriority(c.ID, p); err != nil {
			t.Fatalf("SetCardPriority(%s): %v", p, err)
		}
		got, _ := testDB.GetCard(c.ID)
		if got.Priority != p {
			t.Fatalf("Priority = %q, want %q", got.Priority, p)
		}
	}
}

func TestCards_SetStatus(t *testing.T) {
	b := mustBoard(t, "Stat")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "X")

	for _, s := range []Status{StatusInProgress, StatusBlocked, StatusDone, StatusTodo} {
		if err := testDB.SetCardStatus(c.ID, s); err != nil {
			t.Fatalf("SetCardStatus(%s): %v", s, err)
		}
		got, _ := testDB.GetCard(c.ID)
		if got.Status != s {
			t.Fatalf("Status = %q, want %q", got.Status, s)
		}
	}
}

func TestCards_ToggleDone(t *testing.T) {
	b := mustBoard(t, "Toggle")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "X")

	// todo → done
	if err := testDB.ToggleCardDone(c.ID); err != nil {
		t.Fatalf("ToggleCardDone (todo→done): %v", err)
	}
	got, _ := testDB.GetCard(c.ID)
	if got.Status != StatusDone {
		t.Fatalf("Status = %q after first toggle, want done", got.Status)
	}

	// done → todo
	if err := testDB.ToggleCardDone(c.ID); err != nil {
		t.Fatalf("ToggleCardDone (done→todo): %v", err)
	}
	got, _ = testDB.GetCard(c.ID)
	if got.Status != StatusTodo {
		t.Fatalf("Status = %q after second toggle, want todo", got.Status)
	}
}

func TestCards_SetPosition(t *testing.T) {
	b := mustBoard(t, "SetPos")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "X")

	if err := testDB.SetCardPosition(c.ID, 42); err != nil {
		t.Fatalf("SetCardPosition: %v", err)
	}
	got, _ := testDB.GetCard(c.ID)
	if got.Position != 42 {
		t.Fatalf("Position = %d, want 42", got.Position)
	}
}

func TestCards_MoveToColumn(t *testing.T) {
	b := mustBoard(t, "MoveCard")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	src := mustColumn(t, b.ID, "Source")
	dst := mustColumn(t, b.ID, "Dest")

	// Pre-populate dest so position logic is exercised.
	mustCard(t, b.ID, dst.ID, "Already here")

	c := mustCard(t, b.ID, src.ID, "Mover")
	if err := testDB.MoveCardToColumn(c.ID, dst.ID); err != nil {
		t.Fatalf("MoveCardToColumn: %v", err)
	}
	got, _ := testDB.GetCard(c.ID)
	if got.ColumnID != dst.ID {
		t.Fatalf("ColumnID = %q, want %q", got.ColumnID, dst.ID)
	}
	if got.Position != 1 {
		t.Fatalf("Position = %d, want 1 (appended after existing card)", got.Position)
	}
}

func TestCards_ArchiveAndRestore(t *testing.T) {
	b := mustBoard(t, "ArchRestore")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "X")

	if err := testDB.ArchiveCard(c.ID); err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}
	got, _ := testDB.GetCard(c.ID)
	if !got.Archived {
		t.Fatal("card should be archived")
	}

	if err := testDB.RestoreCard(c.ID); err != nil {
		t.Fatalf("RestoreCard: %v", err)
	}
	got, _ = testDB.GetCard(c.ID)
	if got.Archived {
		t.Fatal("card should not be archived after restore")
	}
}

func TestCards_DeletePermanent(t *testing.T) {
	b := mustBoard(t, "PermDel")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "Gone")

	if err := testDB.DeleteCardPermanent(c.ID); err != nil {
		t.Fatalf("DeleteCardPermanent: %v", err)
	}
	_, err := testDB.GetCard(c.ID)
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows after permanent delete, got %v", err)
	}
}

// ── Edges ─────────────────────────────────────────────────────────────────────

func TestEdges_BlocksBasic(t *testing.T) {
	b := mustBoard(t, "BlkBasic")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)

	// GetBlockers for B → [A]
	blockers, err := testDB.GetBlockers(bCard.ID)
	if err != nil {
		t.Fatalf("GetBlockers: %v", err)
	}
	if len(blockers) != 1 || blockers[0].ID != a.ID {
		t.Fatalf("GetBlockers = %v, want [A]", blockers)
	}

	// GetBlocking for A → [B]
	blocking, err := testDB.GetBlocking(a.ID)
	if err != nil {
		t.Fatalf("GetBlocking: %v", err)
	}
	if len(blocking) != 1 || blocking[0].ID != bCard.ID {
		t.Fatalf("GetBlocking = %v, want [B]", blocking)
	}
}

func TestEdges_ParentOfBasic(t *testing.T) {
	b := mustBoard(t, "ParentBasic")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	parent := mustCard(t, b.ID, col.ID, "Parent")
	child1 := mustCard(t, b.ID, col.ID, "Child1")
	child2 := mustCard(t, b.ID, col.ID, "Child2")

	mustEdge(t, parent.ID, child1.ID, EdgeParentOf)
	mustEdge(t, parent.ID, child2.ID, EdgeParentOf)

	// GetChildren for parent → [child1, child2]
	children, err := testDB.GetChildren(parent.ID)
	if err != nil {
		t.Fatalf("GetChildren: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("GetChildren = %d cards, want 2", len(children))
	}

	// GetParents for child1 → [parent]
	parents, err := testDB.GetParents(child1.ID)
	if err != nil {
		t.Fatalf("GetParents: %v", err)
	}
	if len(parents) != 1 || parents[0].ID != parent.ID {
		t.Fatalf("GetParents = %v, want [parent]", parents)
	}

	// GetParents for parent should be empty.
	pp, _ := testDB.GetParents(parent.ID)
	if len(pp) != 0 {
		t.Fatalf("parent should have no parents, got %v", pp)
	}
}

func TestEdges_SelfReferential(t *testing.T) {
	b := mustBoard(t, "SelfRef")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")
	c := mustCard(t, b.ID, col.ID, "X")

	if err := testDB.AddEdge(c.ID, c.ID, EdgeBlocks); err == nil {
		t.Fatal("expected error for self-referential edge")
	}
	if err := testDB.AddEdge(c.ID, c.ID, EdgeParentOf); err == nil {
		t.Fatal("expected error for self-referential parent edge")
	}
}

func TestEdges_CycleDetection_DirectBlocks(t *testing.T) {
	b := mustBoard(t, "CycleBlk")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	// A blocks B
	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)

	// B blocks A → cycle
	if err := testDB.AddEdge(bCard.ID, a.ID, EdgeBlocks); err == nil {
		t.Fatal("expected cycle error: B→A would close cycle A→B→A")
	}
}

func TestEdges_CycleDetection_DirectParent(t *testing.T) {
	b := mustBoard(t, "CyclePar")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	// A parent_of B
	mustEdge(t, a.ID, bCard.ID, EdgeParentOf)

	// B parent_of A → cycle
	if err := testDB.AddEdge(bCard.ID, a.ID, EdgeParentOf); err == nil {
		t.Fatal("expected cycle error: B→A parent_of would close cycle")
	}
}

func TestEdges_CycleDetection_Transitive(t *testing.T) {
	b := mustBoard(t, "CycleTrans")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")
	cCard := mustCard(t, b.ID, col.ID, "C")

	// A parent_of B, B parent_of C  →  chain A→B→C
	mustEdge(t, a.ID, bCard.ID, EdgeParentOf)
	mustEdge(t, bCard.ID, cCard.ID, EdgeParentOf)

	// C parent_of A → closes the cycle A→B→C→A
	if err := testDB.AddEdge(cCard.ID, a.ID, EdgeParentOf); err == nil {
		t.Fatal("expected cycle error for transitive C→A parent_of")
	}

	// Unrelated edge type should be independent: blocks does not interfere with parent_of DAG.
	// C blocks A is fine (different edge type).
	if err := testDB.AddEdge(cCard.ID, a.ID, EdgeBlocks); err != nil {
		t.Fatalf("cross-type edge C(blocks)A should be allowed: %v", err)
	}
}

func TestEdges_CycleDetection_DoesNotCrossTypes(t *testing.T) {
	b := mustBoard(t, "CrossType")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	// A blocks B; adding B parent_of A should be fine (different type).
	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)
	if err := testDB.AddEdge(bCard.ID, a.ID, EdgeParentOf); err != nil {
		t.Fatalf("B parent_of A should be allowed when A blocks B: %v", err)
	}
}

func TestEdges_DeleteEdge(t *testing.T) {
	b := mustBoard(t, "DelEdge")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)

	if err := testDB.DeleteEdge(a.ID, bCard.ID, EdgeBlocks); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}

	blockers, _ := testDB.GetBlockers(bCard.ID)
	if len(blockers) != 0 {
		t.Fatalf("expected no blockers after DeleteEdge, got %v", blockers)
	}

	// After deletion, adding again should succeed (no phantom cycle).
	if err := testDB.AddEdge(a.ID, bCard.ID, EdgeBlocks); err != nil {
		t.Fatalf("re-adding deleted edge: %v", err)
	}
}

func TestEdges_ReactivateOnConflict(t *testing.T) {
	b := mustBoard(t, "ReactEdge")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	// Add, archive via card archive, then restore.
	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)
	testDB.ArchiveCard(a.ID) //nolint:errcheck

	blockers, _ := testDB.GetBlockers(bCard.ID)
	if len(blockers) != 0 {
		t.Fatal("edge should be soft-removed after archiving source card")
	}

	testDB.RestoreCard(a.ID) //nolint:errcheck
	blockers, _ = testDB.GetBlockers(bCard.ID)
	if len(blockers) != 1 {
		t.Fatalf("edge should be restored; got %d blockers", len(blockers))
	}

	// Adding the same edge again (already active) should not error.
	if err := testDB.AddEdge(a.ID, bCard.ID, EdgeBlocks); err != nil {
		t.Fatalf("adding already-active edge: %v", err)
	}
}

// ── GetCardsWithMeta ──────────────────────────────────────────────────────────

func TestGetCardsWithMeta_Indicators(t *testing.T) {
	b := mustBoard(t, "MetaInd")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	parent := mustCard(t, b.ID, col.ID, "Parent")
	child := mustCard(t, b.ID, col.ID, "Child")
	blocker := mustCard(t, b.ID, col.ID, "Blocker")
	blocked := mustCard(t, b.ID, col.ID, "Blocked")
	plain := mustCard(t, b.ID, col.ID, "Plain")

	mustEdge(t, parent.ID, child.ID, EdgeParentOf)
	mustEdge(t, blocker.ID, blocked.ID, EdgeBlocks)

	rows, err := testDB.GetCardsWithMeta(b.ID, false)
	if err != nil {
		t.Fatalf("GetCardsWithMeta: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("len = %d, want 5", len(rows))
	}

	byID := make(map[string]CardRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	// parent: ChildrenCount=1, HasParent=false, IsBlocked=false, BlockingCount=0
	p := byID[parent.ID]
	if p.ChildrenCount != 1 {
		t.Fatalf("parent.ChildrenCount = %d, want 1", p.ChildrenCount)
	}
	if p.HasParent {
		t.Fatal("parent.HasParent should be false")
	}
	if p.IsBlocked {
		t.Fatal("parent.IsBlocked should be false")
	}
	if p.BlockingCount != 0 {
		t.Fatalf("parent.BlockingCount = %d, want 0", p.BlockingCount)
	}

	// child: HasParent=true, ChildrenCount=0
	ch := byID[child.ID]
	if !ch.HasParent {
		t.Fatal("child.HasParent should be true")
	}
	if ch.ChildrenCount != 0 {
		t.Fatalf("child.ChildrenCount = %d, want 0", ch.ChildrenCount)
	}

	// blocker: BlockingCount=1, IsBlocked=false
	bl := byID[blocker.ID]
	if bl.BlockingCount != 1 {
		t.Fatalf("blocker.BlockingCount = %d, want 1", bl.BlockingCount)
	}
	if bl.IsBlocked {
		t.Fatal("blocker.IsBlocked should be false")
	}

	// blocked: IsBlocked=true, BlockingCount=0
	bk := byID[blocked.ID]
	if !bk.IsBlocked {
		t.Fatal("blocked.IsBlocked should be true")
	}
	if bk.BlockingCount != 0 {
		t.Fatalf("blocked.BlockingCount = %d, want 0", bk.BlockingCount)
	}

	// plain: all false / zero
	pl := byID[plain.ID]
	if pl.HasParent || pl.ChildrenCount != 0 || pl.BlockingCount != 0 || pl.IsBlocked {
		t.Fatalf("plain card has unexpected meta: %+v", pl)
	}
}

func TestGetCardsWithMeta_ExcludesArchivedEdges(t *testing.T) {
	b := mustBoard(t, "MetaArch")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)

	// Archive A → edge soft-removed.
	if err := testDB.ArchiveCard(a.ID); err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}

	rows, err := testDB.GetCardsWithMeta(b.ID, false)
	if err != nil {
		t.Fatalf("GetCardsWithMeta: %v", err)
	}
	// Only B is visible (A is archived).
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1", len(rows))
	}
	if rows[0].IsBlocked {
		t.Fatal("B.IsBlocked should be false after A archived")
	}

	// With includeArchived=true both cards appear, but A's edges are still archived.
	rows, err = testDB.GetCardsWithMeta(b.ID, true)
	if err != nil {
		t.Fatalf("GetCardsWithMeta(all): %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2", len(rows))
	}
	byID := map[string]CardRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if byID[bCard.ID].IsBlocked {
		t.Fatal("B.IsBlocked should be false (edge still archived)")
	}
}

func TestGetCardsWithMeta_RestoredEdgesVisible(t *testing.T) {
	b := mustBoard(t, "MetaRest")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")

	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)
	testDB.ArchiveCard(a.ID)  //nolint:errcheck
	testDB.RestoreCard(a.ID)  //nolint:errcheck

	rows, err := testDB.GetCardsWithMeta(b.ID, false)
	if err != nil {
		t.Fatalf("GetCardsWithMeta: %v", err)
	}
	byID := map[string]CardRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if byID[bCard.ID].IsBlocked != true {
		t.Fatal("B.IsBlocked should be true after edge restored")
	}
	if byID[a.ID].BlockingCount != 1 {
		t.Fatalf("A.BlockingCount = %d, want 1 after restore", byID[a.ID].BlockingCount)
	}
}

// ── Archive / Restore edge semantics ─────────────────────────────────────────

func TestArchive_EdgesSoftRemoved(t *testing.T) {
	b := mustBoard(t, "ArchEdge")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")
	cCard := mustCard(t, b.ID, col.ID, "C")

	// A blocks B; A parent_of C
	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)
	mustEdge(t, a.ID, cCard.ID, EdgeParentOf)

	if err := testDB.ArchiveCard(a.ID); err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}

	// B has no blockers; C has no parents.
	blockers, _ := testDB.GetBlockers(bCard.ID)
	if len(blockers) != 0 {
		t.Fatalf("expected 0 blockers for B, got %d", len(blockers))
	}
	parents, _ := testDB.GetParents(cCard.ID)
	if len(parents) != 0 {
		t.Fatalf("expected 0 parents for C, got %d", len(parents))
	}
}

func TestRestore_SkipsStillArchivedCounterpart(t *testing.T) {
	b := mustBoard(t, "RestSkip")
	defer testDB.DeleteBoard(b.ID) //nolint:errcheck
	col := mustColumn(t, b.ID, "Col")

	a := mustCard(t, b.ID, col.ID, "A")
	bCard := mustCard(t, b.ID, col.ID, "B")
	cCard := mustCard(t, b.ID, col.ID, "C")

	// A blocks B; A blocks C
	mustEdge(t, a.ID, bCard.ID, EdgeBlocks)
	mustEdge(t, a.ID, cCard.ID, EdgeBlocks)

	// Archive both A and B.
	testDB.ArchiveCard(a.ID) //nolint:errcheck
	testDB.ArchiveCard(bCard.ID) //nolint:errcheck

	// Restore A: edge to C should come back, but edge to B must stay archived.
	if err := testDB.RestoreCard(a.ID); err != nil {
		t.Fatalf("RestoreCard: %v", err)
	}

	// A blocks C again.
	blocking, _ := testDB.GetBlocking(a.ID)
	if len(blocking) != 1 || blocking[0].ID != cCard.ID {
		t.Fatalf("GetBlocking(A) = %v, want [C]", blocking)
	}

	// B is still archived, so the edge A→B stays archived.
	blockers, _ := testDB.GetBlockers(bCard.ID)
	if len(blockers) != 0 {
		t.Fatalf("B is archived; expected 0 blockers, got %d", len(blockers))
	}
}

// ── DBPath ────────────────────────────────────────────────────────────────────

func TestDBPath_Default(t *testing.T) {
	// Unset XDG_DATA_HOME so we fall through to ~/.local/share.
	old := os.Getenv("XDG_DATA_HOME")
	os.Unsetenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", old) //nolint:errcheck

	p, err := DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if p == "" {
		t.Fatal("DBPath returned empty string")
	}
}

func TestDBPath_XDGOverride(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdgtest")
	p, err := DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	want := "/tmp/xdgtest/todo-board/todo-board.db"
	if p != want {
		t.Fatalf("DBPath = %q, want %q", p, want)
	}
}
