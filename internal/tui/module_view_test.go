package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/orlandobianco/SecTUI/internal/core"
)

func testFindings() []core.Finding {
	return []core.Finding{
		{
			ID:       "ssh-001",
			Module:   "ssh",
			Severity: core.SeverityCritical,
			TitleKey: "finding.ssh_root_login.title",
			FixID:    "fix-ssh-001",
		},
		{
			ID:       "ssh-002",
			Module:   "ssh",
			Severity: core.SeverityHigh,
			TitleKey: "finding.ssh_password_auth.title",
			FixID:    "fix-ssh-002",
		},
		{
			ID:       "net-001",
			Module:   "network",
			Severity: core.SeverityMedium,
			TitleKey: "finding.net_exposed_port.title",
			FixID:    "", // no auto-fix
		},
	}
}

func TestNewModuleView(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())
	if mv.moduleID != "ssh" {
		t.Errorf("moduleID = %q, want ssh", mv.moduleID)
	}
	if len(mv.findings) != 3 {
		t.Errorf("findings len = %d, want 3", len(mv.findings))
	}
	if mv.cursor != 0 {
		t.Errorf("cursor = %d, want 0", mv.cursor)
	}
}

func TestModuleView_Navigation(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())

	// Move down
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if mv.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", mv.cursor)
	}

	// Move down again
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if mv.cursor != 2 {
		t.Errorf("after 2x j: cursor = %d, want 2", mv.cursor)
	}

	// Move down at bottom (should stay)
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if mv.cursor != 2 {
		t.Errorf("at bottom: cursor = %d, want 2", mv.cursor)
	}

	// Move up
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if mv.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", mv.cursor)
	}
}

func TestModuleView_ToggleSelection(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())

	// Toggle first finding (has FixID)
	mv, _ = mv.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if !mv.selected[0] {
		t.Error("first finding should be selected after space")
	}

	// Toggle again to deselect
	mv, _ = mv.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if mv.selected[0] {
		t.Error("first finding should be deselected after second space")
	}
}

func TestModuleView_ToggleNoFix(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())
	// Move to finding without FixID (index 2)
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	// Try to toggle — should not select (no auto-fix)
	mv, _ = mv.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if mv.selected[2] {
		t.Error("finding without FixID should not be selectable")
	}
}

func TestModuleView_ToggleAll(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())

	// Toggle all
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	// Only fixable findings should be selected (0, 1 have FixID; 2 does not)
	if !mv.selected[0] {
		t.Error("finding 0 should be selected")
	}
	if !mv.selected[1] {
		t.Error("finding 1 should be selected")
	}
	if mv.selected[2] {
		t.Error("finding 2 (no fix) should not be selected")
	}

	// Toggle all again to deselect
	mv, _ = mv.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if mv.selected[0] || mv.selected[1] {
		t.Error("all should be deselected after second toggle-all")
	}
}

func TestModuleView_SelectedFindings(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())
	mv.selected[0] = true
	mv.selected[1] = true

	selected := mv.SelectedFindings()
	if len(selected) != 2 {
		t.Errorf("SelectedFindings() len = %d, want 2", len(selected))
	}
}

func TestModuleView_EnterWithSelection(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())
	mv.selected[0] = true

	_, cmd := mv.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter with selection should return a command")
	}

	// Execute the command to get the message.
	msg := cmd()
	fixMsg, ok := msg.(ApplyFixRequestMsg)
	if !ok {
		t.Fatalf("expected ApplyFixRequestMsg, got %T", msg)
	}
	if fixMsg.ModuleID != "ssh" {
		t.Errorf("ModuleID = %q, want ssh", fixMsg.ModuleID)
	}
	if len(fixMsg.Findings) != 1 {
		t.Errorf("Findings len = %d, want 1", len(fixMsg.Findings))
	}
}

func TestModuleView_EnterWithoutSelection(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())

	_, cmd := mv.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter without selection should not return a command")
	}
}

func TestModuleView_SetSize(t *testing.T) {
	mv := NewModuleView("ssh", testFindings())
	mv = mv.SetSize(80, 40)
	if mv.width != 80 || mv.height != 40 {
		t.Errorf("SetSize: width=%d height=%d, want 80, 40", mv.width, mv.height)
	}
}

func TestModuleView_EmptyFindings(t *testing.T) {
	mv := NewModuleView("ssh", nil)
	mv = mv.SetSize(80, 40)
	view := mv.View()
	if view == "" {
		t.Error("View() with empty findings should not be empty")
	}
}
