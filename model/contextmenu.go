package model

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/style"
)

const (
	contextMenuFrameOverhead = 2 // rounded border only (no padding inside frame)
)

// ContextMenuItem describes a single entry in a context menu.
type ContextMenuItem struct {
	ID        string
	Label     string
	Disabled  bool
	Separator bool // when true, renders as a separator line; other fields ignored
}

// ContextMenu is a vertical list modal anchored at mouse coordinates.
// It appears on right-click and executes Menu.ContextMenuAction when an item is selected.
type ContextMenu struct {
	menu      Menu
	itemIndex int // the menu list item that was right-clicked
	items     []ContextMenuItem
	mouseX    int
	mouseY    int

	focused   int // keyboard-focused item index (-1 = none)
	hovered   int // mouse-hovered item index (-1 = none)
	isDismissed bool
	isCanceled  bool
	selected  *ContextMenuItem

	bounds      popupRect
	boundsSet   bool
	itemBounds  []popupRect // absolute screen coordinates for each selectable item
}

// NewContextMenu constructs a context menu anchored at (mouseX, mouseY).
func NewContextMenu(menu Menu, itemIndex int, items []ContextMenuItem, mouseX, mouseY int) *ContextMenu {
	cm := &ContextMenu{
		menu:      menu,
		itemIndex: itemIndex,
		items:     items,
		mouseX:    mouseX,
		mouseY:    mouseY,
		focused:   -1,
		hovered:   -1,
	}
	// Initialize focused to first selectable item
	cm.focused = cm.firstSelectableFrom(0, 1)
	return cm
}

func (cm *ContextMenu) isSelectable(index int) bool {
	if index < 0 || index >= len(cm.items) {
		return false
	}
	item := cm.items[index]
	return !item.Disabled && !item.Separator
}

// firstSelectableFrom finds the first selectable item starting at `from`, advancing by `delta`.
// Returns -1 if none found.
func (cm *ContextMenu) firstSelectableFrom(from, delta int) int {
	if delta == 0 {
		return -1
	}
	for i := from; i >= 0 && i < len(cm.items); i += delta {
		if cm.isSelectable(i) {
			return i
		}
	}
	return -1
}

// nextSelectable wraps around to find the next selectable item.
func (cm *ContextMenu) nextSelectable(from int) int {
	if len(cm.items) == 0 {
		return -1
	}
	// Try forward from current+1
	if next := cm.firstSelectableFrom(from+1, 1); next != -1 {
		return next
	}
	// Wrap to start
	return cm.firstSelectableFrom(0, 1)
}

// prevSelectable wraps around to find the previous selectable item.
func (cm *ContextMenu) prevSelectable(from int) int {
	if len(cm.items) == 0 {
		return -1
	}
	// Try backward from current-1
	if prev := cm.firstSelectableFrom(from-1, -1); prev != -1 {
		return prev
	}
	// Wrap to end
	return cm.firstSelectableFrom(len(cm.items)-1, -1)
}

func (cm *ContextMenu) dismissed() bool {
	return cm.isDismissed
}

func (cm *ContextMenu) dismissOutside() {
	cm.isDismissed = true
	cm.isCanceled = true
}

func (cm *ContextMenu) dismissEscape() {
	cm.isDismissed = true
	cm.isCanceled = true
}

// complete is called after dismissal to execute the selected action.
func (cm *ContextMenu) complete(app *App) (Page, tea.Cmd) {
	if cm.isCanceled || cm.selected == nil {
		return nil, nil
	}
	return cm.menu.ContextMenuAction(app, cm.itemIndex, *cm.selected)
}

func (cm *ContextMenu) update(msg tea.Msg) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return
	}

	switch keyMsg.String() {
	case "esc":
		cm.dismissEscape()
	case "enter":
		if cm.focused >= 0 && cm.focused < len(cm.items) && cm.isSelectable(cm.focused) {
			cm.selected = &cm.items[cm.focused]
			cm.isDismissed = true
		}
	case "up", "k":
		if cm.focused == -1 {
			cm.focused = cm.firstSelectableFrom(len(cm.items)-1, -1)
		} else {
			cm.focused = cm.prevSelectable(cm.focused)
		}
	case "down", "j":
		if cm.focused == -1 {
			cm.focused = cm.firstSelectableFrom(0, 1)
		} else {
			cm.focused = cm.nextSelectable(cm.focused)
		}
	}
}

func (cm *ContextMenu) handleMouse(msg tea.MouseMsg) (bool, tea.Cmd) {
	mouse := msg.Mouse()
	oldHovered := cm.hovered
	cm.hovered = cm.itemAt(mouse.X, mouse.Y)
	hoverChanged := oldHovered != cm.hovered

	var hoverCmd tea.Cmd
	if hoverChanged {
		if cm.hovered >= 0 {
			hoverCmd = setMousePointer("pointer")
		} else {
			hoverCmd = setMousePointer("default")
		}
	}

	if !cm.boundsSet {
		return true, hoverCmd
	}

	// Check if mouse is inside bounds
	if !cm.bounds.contains(mouse.X, mouse.Y) {
		if cm.hovered != -1 {
			cm.hovered = -1
			hoverCmd = setMousePointer("default")
		}
		return false, hoverCmd
	}

	// Handle click inside menu
	if _, isClick := msg.(tea.MouseClickMsg); isClick && mouse.Button == tea.MouseLeft {
		if cm.hovered >= 0 && cm.hovered < len(cm.items) && cm.isSelectable(cm.hovered) {
			cm.selected = &cm.items[cm.hovered]
			cm.isDismissed = true
			return true, hoverCmd
		}
	}

	return true, hoverCmd
}

// itemAt returns the index of the item at screen coordinates (x, y), or -1 if none.
func (cm *ContextMenu) itemAt(x, y int) int {
	for i, bound := range cm.itemBounds {
		if bound.contains(x, y) {
			return i
		}
	}
	return -1
}

// renderModal renders the context menu as a vertical list with rounded border.
func (cm *ContextMenu) renderModal(styles style.StyleSet) modalRender {
	if len(cm.items) == 0 {
		return modalRender{content: "", itemBounds: nil}
	}

	ss := styles
	surface := styles.Popup.Surface
	// Use frame foreground as separator color
	borderFg := styles.Popup.Frame.GetForeground()

	// Build item rows
	var rows []string
	maxLabelWidth := 0
	for _, item := range cm.items {
		if !item.Separator {
			w := lipgloss.Width(item.Label)
			if w > maxLabelWidth {
				maxLabelWidth = w
			}
		}
	}

	// Inner width: max label + horizontal padding (1 cell each side)
	innerWidth := maxLabelWidth + 2
	if innerWidth < 8 {
		innerWidth = 8 // minimum width for usability
	}

	// Render each item
	for i, item := range cm.items {
		var row string
		if item.Separator {
			// Render separator as horizontal line
			sep := strings.Repeat("─", innerWidth)
			row = lipgloss.NewStyle().
				Foreground(borderFg).
				Background(surface).
				Width(innerWidth).
				Render(sep)
		} else {
			// Choose style based on state
			itemStyle := lipgloss.NewStyle().
				Foreground(ss.MenuItem.GetForeground()).
				Background(surface).
				Width(innerWidth).
				Padding(0, 1)

			if item.Disabled {
				itemStyle = itemStyle.Foreground(ss.Muted.GetForeground())
			} else if i == cm.focused && i == cm.hovered {
				itemStyle = itemStyle.
					Foreground(ss.SelectedItem.GetForeground()).
					Background(ss.SelectedItem.GetBackground()).
					Underline(true)
			} else if i == cm.focused {
				itemStyle = itemStyle.
					Foreground(ss.SelectedItem.GetForeground()).
					Background(ss.SelectedItem.GetBackground())
			} else if i == cm.hovered {
				itemStyle = itemStyle.
					Foreground(ss.MenuItemHover.GetForeground()).
					Underline(true)
			}

			row = itemStyle.Render(item.Label)
		}
		rows = append(rows, row)
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, rows...)
	framed := styles.Popup.Frame.
		Padding(0). // No internal padding, border only
		Render(inner)

	// Compute item bounds (relative to menu top-left)
	itemBounds := make([]popupRect, len(cm.items))
	for i := range cm.items {
		if cm.isSelectable(i) {
			// Item is at row (1 + i) due to top border, spans innerWidth
			itemBounds[i] = popupRect{
				x: 1,              // left border offset
				y: 1 + i,          // top border + row index
				w: innerWidth,
				h: 1,
			}
		} else {
			// Non-selectable items get zero bounds
			itemBounds[i] = popupRect{x: 0, y: 0, w: 0, h: 0}
		}
	}

	return modalRender{
		content:    framed,
		itemBounds: itemBounds,
	}
}

// computePosition calculates the top-left (x, y) for the context menu.
// Applies flip+clamp to keep the menu fully visible.
func (cm *ContextMenu) computePosition(termW, termH, menuW, menuH int) (int, int) {
	// Default: top-left corner at mouse position
	x := cm.mouseX
	y := cm.mouseY

	// Flip horizontally if it would overflow right edge
	if x+menuW > termW {
		x = cm.mouseX - menuW
		if x < 0 {
			x = 0
		}
	}

	// Flip vertically if it would overflow bottom edge
	if y+menuH > termH {
		y = cm.mouseY - menuH
		if y < 0 {
			y = 0
		}
	}

	// Final clamp to screen bounds
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+menuW > termW {
		x = termW - menuW
	}
	if y+menuH > termH {
		y = termH - menuH
	}

	return x, y
}

// setModalBounds stores the absolute screen position and computes absolute item bounds.
func (cm *ContextMenu) setModalBounds(x, y, w, h int, itemBounds []popupRect) {
	cm.bounds = popupRect{x: x, y: y, w: w, h: h}
	cm.boundsSet = true
	cm.itemBounds = make([]popupRect, len(itemBounds))
	for i, bound := range itemBounds {
		cm.itemBounds[i] = popupRect{
			x: x + bound.x,
			y: y + bound.y,
			w: bound.w,
			h: bound.h,
		}
	}
}

// allowsRightClickPassthrough returns true for ContextMenu, allowing right-click outside to reopen.
func (cm *ContextMenu) allowsRightClickPassthrough() bool {
	return true
}

type modalRender struct {
	content    string
	itemBounds []popupRect // relative to modal's top-left corner
}
