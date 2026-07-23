package model

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// NotificationLevel defines the semantic level of a notification.
type NotificationLevel uint8

const (
	NotificationInfo NotificationLevel = iota
	NotificationSuccess
	NotificationWarning
	NotificationError
)

// NotificationID uniquely identifies an active notification.
type NotificationID uint64

// NotificationSpec defines the content and behavior of a notification.
// Message may contain ANSI-styled text. Title is optional plain single-line text.
// Timeout of 0 means the notification must be dismissed manually (default for
// Warning/Error). For Info/Success, a zero Timeout falls back to the app's
// configured default timeout.
type NotificationSpec struct {
	Level   NotificationLevel
	Title   string
	Message string
	Timeout time.Duration
}

// notificationRect is the screen-absolute bounding box of a rendered notification.
type notificationRect struct {
	x, y, w, h int
}

func (r notificationRect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// Notification is an active notification instance managed by the App.
type Notification struct {
	id        NotificationID
	spec      NotificationSpec
	createdAt time.Time
	expireAt  time.Time // zero means no auto-expire

	bounds    notificationRect
	boundsSet bool

	// Selection state
	selecting     bool
	hasSelection  bool
	selAnchorLine int
	selAnchorCol  int
	selCursorLine int
	selCursorCol  int

	// Cached rendering state for hit-testing (populated during render)
	contentLines []string // wrapped content lines
	contentWidth int      // width in columns
	titleHeight int    // 1 if title present, 0 if no title
	titleText   string // raw title text (icon + title) for clipboard copy
}

func (n *Notification) setBounds(x, y, w, h int) {
	n.bounds = notificationRect{x: x, y: y, w: w, h: h}
	n.boundsSet = true
}

func (n *Notification) setContentGeometry(bodyLines []string, cw, th int, titleText string) {
	// Include title as the first content line so selection works across title + body.
	if titleText != "" && th > 0 {
		n.contentLines = append([]string{titleText}, bodyLines...)
	} else {
		n.contentLines = bodyLines
	}
	n.contentWidth = cw
	n.titleHeight = th
	n.titleText = titleText
}

func (n *Notification) clearSelection() {
	n.selecting = false
	n.hasSelection = false
	n.selAnchorLine = 0
	n.selAnchorCol = 0
	n.selCursorLine = 0
	n.selCursorCol = 0
}

func (n *Notification) normalizedSelection() (int, int, int, int) {
	if n.selAnchorLine < n.selCursorLine ||
		(n.selAnchorLine == n.selCursorLine && n.selAnchorCol <= n.selCursorCol) {
		return n.selAnchorLine, n.selAnchorCol, n.selCursorLine, n.selCursorCol
	}
	return n.selCursorLine, n.selCursorCol, n.selAnchorLine, n.selAnchorCol
}

func (n *Notification) selectionRangeForLine(i, width int) (int, int, bool) {
	sL, sC, eL, eC := n.normalizedSelection()
	if i < sL || i > eL {
		return 0, 0, false
	}
	left, right := 0, width
	if i == sL {
		left = sC
	}
	if i == eL {
		right = eC
	}
	left = clampInt(left, 0, width)
	right = clampInt(right, 0, width)
	if right <= left {
		return 0, 0, false
	}
	return left, right, true
}

func (n *Notification) selectionText() string {
	if !n.hasSelection {
		return ""
	}
	sL, _, eL, _ := n.normalizedSelection()
	sL = clampInt(sL, 0, max(len(n.contentLines)-1, 0))
	eL = clampInt(eL, 0, max(len(n.contentLines)-1, 0))
	parts := make([]string, 0, eL-sL+1)
	for i := sL; i <= eL; i++ {
		line := n.contentLines[i]
		width := lipgloss.Width(line)
		left, right, ok := n.selectionRangeForLine(i, width)
		if !ok {
			parts = append(parts, "")
			continue
		}
		segment := ansi.Strip(ansi.Cut(line, left, right))
		if right >= width {
			segment = strings.TrimRight(segment, " ")
		}
		parts = append(parts, segment)
	}
	return strings.Join(parts, "\n")
}

func (n *Notification) finalizeSelection() tea.Cmd {
	text := n.selectionText()
	if strings.TrimSpace(text) == "" {
		n.clearSelection()
		return nil
	}
	return tea.SetClipboard(text)
}

// pointInTitle returns true if (x, y) — screen absolute coords — falls on the title area,
// excluding the close button area (last 2 chars).
func (n *Notification) pointInTitle(x, y int) bool {
	if n.titleHeight == 0 {
		return false
	}
	relX := x - n.bounds.x
	relY := y - n.bounds.y
	// Title text area excludes the last 2 chars (close button)
	return relY == 1 && relX >= 2 && relX < 2+n.contentWidth-2
}

// pointInCloseButton returns true if (x, y) falls on the close button (✕).
func (n *Notification) pointInCloseButton(x, y int) bool {
	if n.titleHeight == 0 {
		return false
	}
	relX := x - n.bounds.x
	relY := y - n.bounds.y
	// Close button is at the rightmost 2 chars of the title line
	return relY == 1 && relX >= 2+n.contentWidth-2 && relX < 2+n.contentWidth
}

// pointInContent returns true if (x, y) — screen absolute coords — falls on the message body area.
func (n *Notification) pointInContent(x, y int) bool {
	if len(n.contentLines) == 0 {
		return false
	}
	relX := x - n.bounds.x
	relY := y - n.bounds.y
	// Content starts at relY=1 (below top border), spans all content lines
	bodyRelY := 1
	maxY := bodyRelY + len(n.contentLines)
	return relY >= bodyRelY && relY < maxY && relX >= 2 && relX < 2+n.contentWidth
}

// contentCoordAt maps screen-absolute (x, y) to content line and display column.
func (n *Notification) contentCoordAt(x, y int) (int, int) {
	relX := x - n.bounds.x
	relY := y - n.bounds.y
	contentRelY := 1 // content (title + body) starts at relY=1
	row := clampInt(relY-contentRelY, 0, max(len(n.contentLines)-1, 0))
	col := clampInt(relX-2, 0, n.contentWidth)
	// If on the title line, clamp column to exclude the close button area
	if n.titleHeight > 0 && row == 0 {
		col = clampInt(col, 0, n.contentWidth-2)
	}
	return row, col
}

// handleMouse processes mouse events for the notification.
// Returns: (consumed, shouldDismiss, cmd).
// consumed=true means the notification handled the event (don't pass to modals/page).
// shouldDismiss=true means the notification should be removed.
func (n *Notification) handleMouse(msg tea.MouseMsg) (bool, bool, tea.Cmd) {
	mouse := msg.Mouse()
	switch msg.(type) {
	case tea.MouseClickMsg:
		// Click on close button → dismiss
		if n.pointInCloseButton(mouse.X, mouse.Y) {
			return true, true, nil
		}
		// Click on content (title or body) → start text selection
		if n.pointInContent(mouse.X, mouse.Y) {
			line, col := n.contentCoordAt(mouse.X, mouse.Y)
			n.selecting = true
			n.hasSelection = true
			n.selAnchorLine = line
			n.selAnchorCol = col
			n.selCursorLine = line
			n.selCursorCol = col
			return true, false, nil
		}
		// Click on frame border (not title, not content) → dismiss
		return true, true, nil

	case tea.MouseMotionMsg:
		// Update selection if actively dragging
		if n.selecting {
			line, col := n.contentCoordAt(mouse.X, mouse.Y)
			n.selCursorLine = line
			n.selCursorCol = col
			return true, false, nil
		}
		// Cursor hint: pointer on close button, I-beam on selectable content
		if n.pointInCloseButton(mouse.X, mouse.Y) {
			return true, false, setMousePointer("pointer")
		}
		if n.pointInContent(mouse.X, mouse.Y) {
			return true, false, setMousePointer("text")
		}
		return true, false, setMousePointer("default")

	case tea.MouseReleaseMsg:
		if n.selecting {
			n.selecting = false
			return true, false, n.finalizeSelection()
		}
	}
	return false, false, nil
}

func (n *Notification) applySelectionHighlight(visibleLines []string, width int) []string {
	out := make([]string, len(visibleLines))
	for i, line := range visibleLines {
		left, right, ok := n.selectionRangeForLine(i, width)
		if !ok {
			out[i] = line
			continue
		}
		out[i] = highlightColumns(line, left, right)
	}
	return out
}

// ---- messages ----

// ShowNotificationMsg triggers displaying a notification. It can be sent from a
// goroutine via program.Send to remain race-free in the Update loop.
type ShowNotificationMsg struct {
	Spec NotificationSpec
}

// notificationExpireMsg signals that a notification's timeout elapsed.
type notificationExpireMsg struct {
	id NotificationID
}

// updateNotificationMsg updates the content of an existing notification.
type updateNotificationMsg struct {
	id   NotificationID
	spec NotificationSpec
}

// dismissNotificationMsg dismisses a specific notification early.
type dismissNotificationMsg struct {
	id NotificationID
}

// clearAllNotificationsMsg dismisses all visible notifications.
type clearAllNotificationsMsg struct{}
