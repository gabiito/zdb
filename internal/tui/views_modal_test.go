package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// ViewsListModel — mode transitions
// ---------------------------------------------------------------------------

// TestViewsListModeTransitionsC verifies that pressing C in modeViews
// transitions to modePickConn and emits EnterPickConnMsg.
func TestViewsListModeTransitionsC(t *testing.T) {
	m := NewViewsListModel([]ViewItem{{Name: "my view", SQL: "SELECT 1"}}, 80, 24)
	if m.mode != modeViews {
		t.Fatalf("initial mode = %v; want modeViews", m.mode)
	}

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	if m2.mode != modePickConn {
		t.Errorf("mode after C = %v; want modePickConn", m2.mode)
	}
	if cmd == nil {
		t.Fatal("cmd is nil; want EnterPickConnMsg command")
	}
	msg := cmd()
	if _, ok := msg.(EnterPickConnMsg); !ok {
		t.Errorf("cmd() = %T; want EnterPickConnMsg", msg)
	}
}

// TestViewsListModeEscFromPickConn verifies that pressing Esc in modePickConn
// steps back to modeViews and emits BackToViewsMsg so the App reloads the
// current connection's views into the still-open modal.
func TestViewsListModeEscFromPickConn(t *testing.T) {
	m := NewViewsListModel(nil, 80, 24)
	m.mode = modePickConn

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m2.mode != modeViews {
		t.Errorf("mode after Esc from pickConn = %v; want modeViews", m2.mode)
	}
	if cmd == nil {
		t.Fatal("cmd is nil; want BackToViewsMsg command")
	}
	msg := cmd()
	if _, ok := msg.(BackToViewsMsg); !ok {
		t.Errorf("cmd() = %T; want BackToViewsMsg", msg)
	}
}

// TestViewsListModeEscFromPickView verifies that pressing Esc in modePickView
// steps back to modePickConn and emits EnterPickConnMsg so the App reloads
// the other-connections list into the still-open modal.
func TestViewsListModeEscFromPickView(t *testing.T) {
	m := NewViewsListModel(nil, 80, 24)
	m.mode = modePickView
	m.pickedConn = "prod"

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m2.mode != modePickConn {
		t.Errorf("mode after Esc from pickView = %v; want modePickConn", m2.mode)
	}
	if m2.pickedConn != "" {
		t.Errorf("pickedConn after Esc from pickView = %q; want empty", m2.pickedConn)
	}
	if cmd == nil {
		t.Fatal("cmd is nil; want EnterPickConnMsg command")
	}
	msg := cmd()
	if _, ok := msg.(EnterPickConnMsg); !ok {
		t.Errorf("cmd() = %T; want EnterPickConnMsg", msg)
	}
}

// TestViewsListModePickConn verifies that pressing Enter on a ConnItem in
// modePickConn transitions to modePickView and emits PickConnSelectedMsg.
func TestViewsListModePickConn(t *testing.T) {
	m := NewViewsListModel(nil, 80, 24)
	m.mode = modePickConn
	m.SetConnItems([]string{"prod", "staging"})

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m2.mode != modePickView {
		t.Errorf("mode after Enter in pickConn = %v; want modePickView", m2.mode)
	}
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd()
	pcMsg, ok := msg.(PickConnSelectedMsg)
	if !ok {
		t.Fatalf("cmd() = %T; want PickConnSelectedMsg", msg)
	}
	if pcMsg.ConnName != "prod" {
		t.Errorf("ConnName = %q; want %q", pcMsg.ConnName, "prod")
	}
}

// TestViewsListModePickView verifies that pressing Enter on a ViewItem in
// modePickView emits CopyViewSelectedMsg with the correct fields.
func TestViewsListModePickView(t *testing.T) {
	m := NewViewsListModel(nil, 80, 24)
	m.mode = modePickView
	m.SetViewItemsForConn("prod", []ViewItem{
		{Name: "active orders", SQL: "SELECT * FROM orders"},
	})

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m2

	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd()
	cvMsg, ok := msg.(CopyViewSelectedMsg)
	if !ok {
		t.Fatalf("cmd() = %T; want CopyViewSelectedMsg", msg)
	}
	if cvMsg.SourceConn != "prod" {
		t.Errorf("SourceConn = %q; want %q", cvMsg.SourceConn, "prod")
	}
	if cvMsg.ViewName != "active orders" {
		t.Errorf("ViewName = %q; want %q", cvMsg.ViewName, "active orders")
	}
	if cvMsg.SQL != "SELECT * FROM orders" {
		t.Errorf("SQL = %q; want %q", cvMsg.SQL, "SELECT * FROM orders")
	}
}

// ---------------------------------------------------------------------------
// SaveViewModel — SetError, HasError, error clears on input, SetPrefilledName
// ---------------------------------------------------------------------------

// TestSaveViewModelSetError verifies that SetError sets the error and HasError
// returns true, and that the error text appears in View().
func TestSaveViewModelSetError(t *testing.T) {
	m := NewSaveViewModel("SELECT 1", 80, 24)
	if m.HasError() {
		t.Fatal("HasError() = true before SetError; want false")
	}

	m.SetError("view named foo already exists in this connection — choose a different name")

	if !m.HasError() {
		t.Error("HasError() = false after SetError; want true")
	}
	if !strings.Contains(m.View(), "already exists") {
		t.Error("View() does not contain error text after SetError")
	}
}

// TestSaveViewModelErrorClearsOnInput verifies that typing clears the inline
// error so subsequent submits start fresh.
func TestSaveViewModelErrorClearsOnInput(t *testing.T) {
	m := NewSaveViewModel("SELECT 1", 80, 24)
	m.SetError("duplicate name")

	// Simulate a key press (typing 'a').
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if m2.HasError() {
		t.Error("HasError() = true after key input; want false (error should clear)")
	}
}

// TestSaveViewModelPrefillName verifies that SetPrefilledName populates the
// input field so it appears in the submitted SaveViewSubmitMsg.
func TestSaveViewModelPrefillName(t *testing.T) {
	m := NewSaveViewModel("SELECT 1", 80, 24)
	m.SetPrefilledName("active orders")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m2

	if cmd == nil {
		t.Fatal("cmd is nil; expected SaveViewSubmitMsg command")
	}
	msg := cmd()
	svMsg, ok := msg.(SaveViewSubmitMsg)
	if !ok {
		t.Fatalf("cmd() = %T; want SaveViewSubmitMsg", msg)
	}
	if svMsg.Name != "active orders" {
		t.Errorf("Name = %q; want %q", svMsg.Name, "active orders")
	}
}
