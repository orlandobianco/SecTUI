package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewSidebar_DefaultSelection(t *testing.T) {
	s := NewSidebar()
	sel := s.Selected()
	if sel.ID != "overview" {
		t.Errorf("default selected = %q, want overview", sel.ID)
	}
	if sel.Section != "overview" {
		t.Errorf("default section = %q, want overview", sel.Section)
	}
}

func TestSidebar_MoveDown(t *testing.T) {
	s := NewSidebar()
	// Move down: overview → ssh (skips "MODULES" header)
	s, _ = s.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	sel := s.Selected()
	if sel.ID != "ssh" {
		t.Errorf("after j: selected = %q, want ssh", sel.ID)
	}
}

func TestSidebar_MoveUp_AtTop(t *testing.T) {
	s := NewSidebar()
	// Already at top, should not move.
	s, _ = s.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	sel := s.Selected()
	if sel.ID != "overview" {
		t.Errorf("after k at top: selected = %q, want overview", sel.ID)
	}
}

func TestSidebar_SkipsHeaders(t *testing.T) {
	s := NewSidebar()
	// Navigate through all items to verify headers are skipped.
	selectableIDs := []string{"overview", "ssh", "firewall", "network", "users", "updates", "kernel", "secstore"}

	for i, wantID := range selectableIDs {
		sel := s.Selected()
		if sel.ID != wantID {
			t.Errorf("step %d: selected = %q, want %q", i, sel.ID, wantID)
		}
		s, _ = s.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
}

func TestSidebar_SetFocused(t *testing.T) {
	s := NewSidebar()
	if !s.Focused() {
		t.Error("sidebar should start focused")
	}

	s = s.SetFocused(false)
	if s.Focused() {
		t.Error("sidebar should be unfocused")
	}

	// Unfocused sidebar should not respond to keys.
	s, _ = s.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if s.Selected().ID != "overview" {
		t.Error("unfocused sidebar should not respond to keys")
	}
}

func TestSidebar_SetSize(t *testing.T) {
	s := NewSidebar()
	s = s.SetSize(30, 40)
	if s.width != 30 || s.height != 40 {
		t.Errorf("SetSize: width=%d height=%d, want 30, 40", s.width, s.height)
	}
}

func TestSidebar_View_NotEmpty(t *testing.T) {
	s := NewSidebar()
	s = s.SetSize(22, 20)
	view := s.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}
