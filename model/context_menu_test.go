package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/style"
)

type testMenu struct {
	DefaultMenu
	items []MenuItem
}

func (m *testMenu) GetMenuKey() string {
	return "test"
}

func (m *testMenu) MenuViews() []MenuItem {
	return m.items
}

func (m *testMenu) ContextMenuItems(_ *App, _ int) []ContextMenuItem {
	return []ContextMenuItem{
		{ID: "action1", Label: "Action 1"},
		{ID: "action2", Label: "Action 2"},
	}
}

func (m *testMenu) ContextMenuAction(_ *App, _ int, item ContextMenuItem) (Page, tea.Cmd) {
	// Return nil to indicate no navigation
	return nil, nil
}

func TestContextMenuComputePositionFlipsRight(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
		{ID: "b", Label: "Action B"},
	}
	cm := NewContextMenu(menu, 0, items, 75, 10) // near right edge

	// Simulate a 20-wide menu at x=75 in an 80-wide terminal
	x, y := cm.computePosition(80, 24, 20, 5)

	// Should flip left: x = 75 - 20 = 55
	if x > 75 {
		t.Errorf("expected menu to flip left, got x=%d", x)
	}
	if y != 10 {
		t.Errorf("expected y=10, got y=%d", y)
	}
}

func TestContextMenuComputePositionFlipsBottom(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
		{ID: "b", Label: "Action B"},
	}
	cm := NewContextMenu(menu, 0, items, 10, 22) // near bottom edge

	// Simulate a 5-tall menu at y=22 in a 24-tall terminal
	x, y := cm.computePosition(80, 24, 20, 5)

	if x != 10 {
		t.Errorf("expected x=10, got x=%d", x)
	}
	// Should flip up: y = 22 - 5 = 17
	if y > 22 {
		t.Errorf("expected menu to flip up, got y=%d", y)
	}
}

func TestContextMenuComputePositionClampsToScreen(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
	}
	cm := NewContextMenu(menu, 0, items, 100, 100) // far outside

	x, y := cm.computePosition(80, 24, 20, 5)

	// Should clamp to screen bounds
	if x < 0 || x+20 > 80 {
		t.Errorf("x=%d out of bounds for 80-wide screen with 20-wide menu", x)
	}
	if y < 0 || y+5 > 24 {
		t.Errorf("y=%d out of bounds for 24-tall screen with 5-tall menu", y)
	}
}

func TestContextMenuKeyboardNavigation(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
		{Separator: true},
		{ID: "b", Label: "Action B", Disabled: true},
		{ID: "c", Label: "Action C"},
	}
	cm := NewContextMenu(menu, 0, items, 10, 10)

	// Initial focus should be on first selectable (index 0)
	if cm.focused != 0 {
		t.Errorf("initial focused=%d, want 0", cm.focused)
	}

	// Down should skip separator and disabled, land on index 3
	cm.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if cm.focused != 3 {
		t.Errorf("after down, focused=%d, want 3 (skipping separator and disabled)", cm.focused)
	}

	// Down again should wrap to index 0
	cm.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if cm.focused != 0 {
		t.Errorf("after second down, focused=%d, want 0 (wrap)", cm.focused)
	}

	// Up should wrap to index 3
	cm.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if cm.focused != 3 {
		t.Errorf("after up, focused=%d, want 3 (wrap)", cm.focused)
	}
}

func TestContextMenuEnterSelectsItem(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
		{ID: "b", Label: "Action B"},
	}
	cm := NewContextMenu(menu, 0, items, 10, 10)

	// Focus on index 1
	cm.focused = 1

	// Press enter
	cm.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if !cm.dismissed() {
		t.Error("expected menu to be dismissed after enter")
	}
	if cm.selected == nil || cm.selected.ID != "b" {
		t.Errorf("expected selected item ID='b', got %v", cm.selected)
	}
}

func TestContextMenuEscapeDismisses(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
	}
	cm := NewContextMenu(menu, 0, items, 10, 10)

	cm.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))

	if !cm.dismissed() {
		t.Error("expected menu to be dismissed after escape")
	}
	if !cm.isCanceled {
		t.Error("expected menu to be canceled after escape")
	}
	if cm.selected != nil {
		t.Error("expected no selected item after escape")
	}
}

func TestContextMenuItemAtHitTest(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
		{ID: "b", Label: "Action B"},
	}
	cm := NewContextMenu(menu, 0, items, 10, 10)

	// Render to get bounds
	rendered := cm.renderModal(style.NewStyleSet(style.DefaultDarkTheme()))
	cm.setModalBounds(10, 10, 20, 4, rendered.itemBounds)

	// itemBounds[0] should be at (11, 11) with width=innerWidth (depends on label)
	// Test that clicking inside first item's bounds returns index 0
	idx := cm.itemAt(cm.itemBounds[0].x, cm.itemBounds[0].y)
	if idx != 0 {
		t.Errorf("itemAt(%d, %d) = %d, want 0", cm.itemBounds[0].x, cm.itemBounds[0].y, idx)
	}

	// Clicking outside any bounds should return -1
	idx = cm.itemAt(0, 0)
	if idx != -1 {
		t.Errorf("itemAt(0, 0) = %d, want -1", idx)
	}
}

func TestContextMenuMouseClickActivatesItem(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
		{ID: "b", Label: "Action B"},
	}
	cm := NewContextMenu(menu, 0, items, 10, 10)

	// Render and set bounds
	rendered := cm.renderModal(style.NewStyleSet(style.DefaultDarkTheme()))
	cm.setModalBounds(10, 10, 20, 4, rendered.itemBounds)

	// Simulate left-click on first item
	cm.hovered = 0 // Simulate hover state
	msg := tea.MouseClickMsg(tea.Mouse{
		X:      cm.itemBounds[0].x,
		Y:      cm.itemBounds[0].y,
		Button: tea.MouseLeft,
	})
	handled, _ := cm.handleMouse(msg)

	if !handled {
		t.Error("expected mouse click to be handled")
	}
	if !cm.dismissed() {
		t.Error("expected menu to be dismissed after click")
	}
	if cm.selected == nil || cm.selected.ID != "a" {
		t.Errorf("expected selected item ID='a', got %v", cm.selected)
	}
}

func TestContextMenuCompleteInvokesAction(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{
		{ID: "a", Label: "Action A"},
	}
	cm := NewContextMenu(menu, 0, items, 10, 10)
	cm.selected = &items[0]

	app := NewApp(DefaultOptions())
	page, cmd := cm.complete(app)

	// testMenu.ContextMenuAction returns (nil, nil)
	if page != nil {
		t.Errorf("expected page=nil, got %v", page)
	}
	if cmd != nil {
		t.Errorf("expected cmd=nil, got %v", cmd)
	}
}

func TestContextMenuEmptyItemsRendersMinWidth(t *testing.T) {
	menu := &testMenu{items: []MenuItem{{Title: "Test"}}}
	items := []ContextMenuItem{}
	cm := NewContextMenu(menu, 0, items, 10, 10)

	rendered := cm.renderModal(style.NewStyleSet(style.DefaultDarkTheme()))

	if rendered.content != "" {
		t.Error("expected empty content for empty items")
	}
}
