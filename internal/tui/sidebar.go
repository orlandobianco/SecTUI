package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const sidebarWidth = 22

type SidebarItem struct {
	Label   string
	Section string // "overview", "modules", "tools", "secstore"
	ID      string
	Badge   string // "[ON]", "[OFF]", or ""
	Spinner bool   // true if tool has an active background job
	header  bool   // true for section headers (not selectable)
}

type Sidebar struct {
	items        []SidebarItem
	cursor       int
	focused      bool
	spinnerFrame int
	width        int
	height       int
}

type SidebarSelectionMsg struct {
	Item SidebarItem
}

func NewSidebar() Sidebar {
	items := []SidebarItem{
		{Label: "OVERVIEW", Section: "overview", ID: "overview"},
		{Label: "MODULES", header: true},
		{Label: "SSH", Section: "modules", ID: "ssh"},
		{Label: "Firewall", Section: "modules", ID: "firewall"},
		{Label: "Network", Section: "modules", ID: "network"},
		{Label: "Users & Perms", Section: "modules", ID: "users"},
		{Label: "Updates", Section: "modules", ID: "updates"},
		{Label: "Kernel", Section: "modules", ID: "kernel"},
		{Label: "TOOLS", header: true},
		{Label: "SECSTORE", Section: "secstore", ID: "secstore"},
	}

	s := Sidebar{
		items:   items,
		cursor:  0,
		focused: true,
		width:   sidebarWidth,
	}
	return s
}

func (s Sidebar) Init() tea.Cmd {
	return nil
}

func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	if !s.focused {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			s.moveDown()
		case "k", "up":
			s.moveUp()
		}
	}

	return s, nil
}

func (s Sidebar) View() string {
	var b strings.Builder

	contentHeight := s.height
	if contentHeight <= 0 {
		contentHeight = len(s.items) + 4
	}

	for i, item := range s.items {
		if item.header {
			line := StyleSidebarHeader.Render(item.Label)
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}

		label := item.Label
		if item.Spinner {
			frame := spinnerFrames[s.spinnerFrame%len(spinnerFrames)]
			badge := StyleBadgeSpinner.Render(frame)
			pad := s.width - lipgloss.Width(item.Label) - lipgloss.Width(frame) - 4
			if pad < 1 {
				pad = 1
			}
			label = item.Label + strings.Repeat(" ", pad) + badge
		} else if item.Badge != "" {
			badge := item.Badge
			if item.Badge == "[ON]" {
				badge = StyleBadgeON.Render(badge)
			} else {
				badge = StyleBadgeOFF.Render(badge)
			}
			pad := s.width - lipgloss.Width(item.Label) - lipgloss.Width(item.Badge) - 4
			if pad < 1 {
				pad = 1
			}
			label = item.Label + strings.Repeat(" ", pad) + badge
		}

		if i == s.cursor {
			if s.focused {
				line := StyleSidebarActive.Render("▸ " + label)
				b.WriteString(line)
			} else {
				// Dimmed cursor when sidebar is not focused.
				line := lipgloss.NewStyle().
					Foreground(ColorDimmed).
					PaddingLeft(0).
					Render("> " + label)
				b.WriteString(line)
			}
		} else {
			line := StyleSidebarItem.Render(label)
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Border color changes with focus.
	borderColor := ColorDimmed
	if s.focused {
		borderColor = ColorAccent
	}
	style := lipgloss.NewStyle().
		Width(s.width).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(borderColor).
		Padding(1, 1).
		Height(contentHeight)
	return style.Render(b.String())
}

func (s Sidebar) Selected() SidebarItem {
	if s.cursor >= 0 && s.cursor < len(s.items) {
		return s.items[s.cursor]
	}
	return SidebarItem{}
}

func (s Sidebar) SetTools(tools []SidebarItem) Sidebar {
	toolsIdx := -1
	secstoreIdx := -1

	for i, item := range s.items {
		if item.header && item.Label == "TOOLS" {
			toolsIdx = i
		}
		if item.Section == "secstore" {
			secstoreIdx = i
		}
	}

	if toolsIdx == -1 || secstoreIdx == -1 {
		return s
	}

	newItems := make([]SidebarItem, 0, len(s.items)+len(tools))
	newItems = append(newItems, s.items[:toolsIdx+1]...)
	newItems = append(newItems, tools...)
	newItems = append(newItems, s.items[secstoreIdx:]...)

	s.items = newItems
	return s
}

func (s *Sidebar) moveDown() {
	for next := s.cursor + 1; next < len(s.items); next++ {
		if !s.items[next].header {
			s.cursor = next
			return
		}
	}
}

func (s *Sidebar) moveUp() {
	for prev := s.cursor - 1; prev >= 0; prev-- {
		if !s.items[prev].header {
			s.cursor = prev
			return
		}
	}
}

func (s Sidebar) SetFocused(focused bool) Sidebar {
	s.focused = focused
	return s
}

func (s Sidebar) SetSize(width, height int) Sidebar {
	s.width = width
	s.height = height
	return s
}

func (s Sidebar) Focused() bool {
	return s.focused
}

func (s Sidebar) SetSpinnerFrame(frame int) Sidebar {
	s.spinnerFrame = frame
	return s
}

// String implements fmt.Stringer for debug output.
func (s Sidebar) String() string {
	sel := s.Selected()
	return fmt.Sprintf("Sidebar{cursor=%d, selected=%s, focused=%t}", s.cursor, sel.ID, s.focused)
}
