package model

import (
	"strings"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/style"
)

// PopupType classifies the popup for default button configuration.
type PopupType int

const (
	PopupInfo   PopupType = iota // OK button only
	PopupConfirm                 // Confirm + Cancel
	PopupCustom                  // Application-defined body
)

// PopupButton is a single action button in the popup.
type PopupButton struct {
	Text     string
	IsCancel bool // Esc triggers this button
}

// PopupResult is passed to the OnResult callback when the popup is dismissed.
type PopupResult struct {
	ButtonIndex int
	ButtonText  string
	IsCancel    bool
}

// PopupAnchor controls where on the screen the popup appears.
type PopupAnchor int

const (
	AnchorCenter       PopupAnchor = iota // centered on screen (default)
	AnchorTopLeft                        // top-left corner
	AnchorTopCenter                      // top-center edge
	AnchorTopRight                       // top-right corner
	AnchorBottomLeft                     // bottom-left corner
	AnchorBottomCenter                   // bottom-center edge
	AnchorBottomRight                    // bottom-right corner
	AnchorCustom                         // absolute coordinates via OffsetX / OffsetY
)

// Popup represents a modal dialog that overlays the current page.
type Popup struct {
	Title   string
	Message string // Used for Info/Confirm types
	Body    string // Custom body string (for Custom type; lipgloss-ready)
	Buttons []PopupButton // For Custom type; Info/Confirm auto-generate these
	OnResult func(PopupResult) // Called on dismiss; nil means fire-and-forget

	// MaxHeight limits the total popup height (0 = no limit, default behavior).
	MaxHeight int

	// MaxWidth limits the total popup width (0 = no limit, default behavior).
	MaxWidth int

	// Internal state
	typ           PopupType
	focusedButton int
	hoveredButton int // -1 = none, 0+ = index in Buttons (mouse hover tracking)
	result        *PopupResult // set when dismissed, consumed by App

	// Scroll state (for Custom popups with overflowing body content)
	scrollOffset   int // current scroll position (0 = top)
	totalBodyLines int // cached body line count after last render

	// Positioning
	Anchor  PopupAnchor // where on screen to place the popup (default: AnchorCenter)
	OffsetX int         // horizontal offset from anchor origin (updated by drag)
	OffsetY int         // vertical offset from anchor origin (updated by drag)

	// Drag state
	dragging      bool // true during left-button drag
	dragMouseX    int  // mouse X at drag start
	dragMouseY    int  // mouse Y at drag start
	dragStartOffX int  // OffsetX at drag start
	dragStartOffY int  // OffsetY at drag start

	// Mouse interaction state (set by compositePopup after rendering)
	x      int // horizontal position on screen
	y      int // vertical position on screen
	width  int // visual width
	height int // visual height


}

// NewInfoPopup creates an informational popup with an "OK" button.
func NewInfoPopup(title, message string, onResult func(PopupResult)) *Popup {
	return &Popup{
		Title:        title,
		Message:      message,
		typ:          PopupInfo,
		Buttons:      []PopupButton{{Text: "OK"}},
		OnResult:     onResult,
		hoveredButton: -1,
	}
}

// NewConfirmPopup creates a confirmation popup with "Confirm" and "Cancel" buttons.
// The Cancel button has IsCancel: true, triggered by Esc.
func NewConfirmPopup(title, message string, onResult func(PopupResult)) *Popup {
	return &Popup{
		Title:         title,
		Message:       message,
		typ:           PopupConfirm,
		Buttons: []PopupButton{
			{Text: "Confirm"},
			{Text: "Cancel", IsCancel: true},
		},
		OnResult:      onResult,
		hoveredButton: -1,
	}
}

// NewCustomPopup creates a popup with a custom body string and button set.
// The body should be a lipgloss-styled string.
func NewCustomPopup(title, body string, buttons []PopupButton, onResult func(PopupResult)) *Popup {
	return &Popup{
		Title:         title,
		Body:          body,
		typ:           PopupCustom,
		Buttons:       buttons,
		OnResult:      onResult,
		hoveredButton: -1,
	}
}

// Update handles keyboard input for the popup.
// Returns the popup result if the popup was dismissed, nil otherwise.
// Callers should check Popup.Dismissed() after calling Update.
func (p *Popup) Update(msg tea.Msg) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return
	}

	key := keyMsg.String()

	// Scroll handling for overflowing custom popup bodies
	if p.typ == PopupCustom && p.Body != "" && p.totalBodyLines > 0 {
		visibleBodyLines := p.computeVisibleBodyLines()
		maxScroll := p.totalBodyLines - visibleBodyLines
		if maxScroll > 0 {
			switch key {
			case "up", "k":
				p.scrollOffset--
				if p.scrollOffset < 0 {
					p.scrollOffset = 0
				}
				return // consume key, don't process as button nav
			case "down", "j":
				p.scrollOffset++
				if p.scrollOffset > maxScroll {
					p.scrollOffset = maxScroll
				}
				return
			case "pgup":
				p.scrollOffset -= max(visibleBodyLines/2, 1)
				if p.scrollOffset < 0 {
					p.scrollOffset = 0
				}
				return
			case "pgdn":
				p.scrollOffset += max(visibleBodyLines/2, 1)
				if p.scrollOffset > maxScroll {
					p.scrollOffset = maxScroll
				}
				return
			case "home":
				p.scrollOffset = 0
				return
			case "end":
				p.scrollOffset = maxScroll
				return
			}
		}
	}

	switch key {
	case "esc":
		// Find the cancel button (if any) and trigger it
		for i, btn := range p.Buttons {
			if btn.IsCancel {
				p.result = &PopupResult{ButtonIndex: i, ButtonText: btn.Text, IsCancel: true}
				return
			}
		}
		// No cancel button: treat as dismiss without result
		p.result = &PopupResult{ButtonIndex: -1, IsCancel: true}

	case "enter":
		if len(p.Buttons) > 0 {
			btn := p.Buttons[p.focusedButton]
			p.result = &PopupResult{ButtonIndex: p.focusedButton, ButtonText: btn.Text, IsCancel: btn.IsCancel}
		}

	case "tab":
		if len(p.Buttons) > 1 {
			p.focusedButton = (p.focusedButton + 1) % len(p.Buttons)
		}

	case "left", "h", "H":
		if len(p.Buttons) > 1 {
			p.focusedButton--
			if p.focusedButton < 0 {
				p.focusedButton = len(p.Buttons) - 1
			}
		}

	case "right", "l", "L":
		if len(p.Buttons) > 1 {
			p.focusedButton = (p.focusedButton + 1) % len(p.Buttons)
		}
	}
}

// Dismissed returns true if the popup has been dismissed (via button press or esc).
func (p *Popup) Dismissed() bool {
	return p.result != nil
}

// ConsumeResult returns the result and clears the internal state.
// Returns nil if the popup hasn't been dismissed yet.
func (p *Popup) ConsumeResult() *PopupResult {
	r := p.result
	p.result = nil
	return r
}

// Render returns the popup rendered as a lipgloss-styled box.
// Uses Dialog-style compositing: JoinVertical(Center, ...) + JoinHorizontal(Top, buttons).
func (p *Popup) Render(styles style.StyleSet) string {
	// Render title and buttons first so we can measure their height
	var titleStr, buttonStr string
	var titleLines, buttonLines int

	if p.Title != "" {
		titleStr = styles.PopupTitle.Render(p.Title)
		titleLines = lipgloss.Height(titleStr)
	}

	if len(p.Buttons) > 0 {
		buttonStr = p.renderButtons(styles)
		buttonLines = lipgloss.Height(buttonStr)
	}

	// Calculate available body height
	maxH := p.MaxHeight
	if maxH <= 0 {
		maxH = 9999 // effectively unlimited
	}
	borderLines := 2                           // PopupBase border top+bottom
	chromeLines := titleLines + buttonLines    // title + buttons
	availableBodyH := maxH - borderLines - chromeLines
	if availableBodyH < 1 {
		availableBodyH = 1
	}

	// Render body (possibly scrolled)
	var bodyStr string
	if p.typ == PopupCustom && p.Body != "" {
		bodyText := strings.TrimSpace(p.Body)
		bodyLines := strings.Split(bodyText, "\n")
		p.totalBodyLines = len(bodyLines)

		// If body fits within available height, render as-is and reset scroll
		if len(bodyLines) <= availableBodyH {
			bodyStr = styles.PopupBody.Render(bodyText)
			p.scrollOffset = 0
		} else {
			// Clamp scroll
			maxScroll := len(bodyLines) - availableBodyH
			if p.scrollOffset > maxScroll {
				p.scrollOffset = maxScroll
			}
			if p.scrollOffset < 0 {
				p.scrollOffset = 0
			}

			// Slice the visible portion
			visible := bodyLines[p.scrollOffset : p.scrollOffset+availableBodyH]

			// Style each line and compute max visual width
			styledLines := make([]string, len(visible))
			maxLineW := 0
			for i, line := range visible {
				styled := styles.PopupBody.Render(line)
				styledLines[i] = styled
				if w := lipgloss.Width(styled); w > maxLineW {
					maxLineW = w
				}
			}

			// Scrollbar: track │, thumb █ (thin block)
			trackChar := "│"
			thumbChar := "█"

			// Thumb position within visible window
			thumbLine := 0
			if maxScroll > 0 {
				thumbLine = (p.scrollOffset * (availableBodyH - 1)) / maxScroll
			}

			// Pad lines to uniform width then append scrollbar
			contentW := maxLineW + 2 // +2: spacer + scrollbar char
			for i, styled := range styledLines {
				// Pad to uniform width using lipgloss, preserving left-aligned content
				padded := lipgloss.NewStyle().Width(contentW - 2).Render(styled)
				sb := " " + trackChar
				if i == thumbLine {
					sb = " " + thumbChar
				}
				styledLines[i] = padded + sb
			}

			bodyStr = strings.Join(styledLines, "\n")
		}
	} else if p.Message != "" {
		bodyStr = styles.PopupBody.Render(p.Message)
		p.scrollOffset = 0
	}

	// Compose blocks
	var blocks []string
	if titleStr != "" {
		blocks = append(blocks, titleStr)
	}
	if bodyStr != "" {
		blocks = append(blocks, bodyStr)
	}
	if buttonStr != "" {
		blocks = append(blocks, buttonStr)
	}

	inner := lipgloss.JoinVertical(lipgloss.Center, blocks...)

	// Apply MaxWidth if set
	if p.MaxWidth > 0 {
		inner = lipgloss.NewStyle().MaxWidth(p.MaxWidth).Render(inner)
	}

	return styles.PopupBase.Render(inner)
}

// renderButtons returns a row of Dialog-style buttons (background-colored, no brackets).
// Active button uses reverse background. When both active and hovered, active colors
// are preserved and underline is added. Hover-only uses PopupButtonHover style.
func (p *Popup) renderButtons(styles style.StyleSet) string {
	var parts []string
	for i, btn := range p.Buttons {
		switch {
		case i == p.focusedButton && i == p.hoveredButton:
			parts = append(parts, styles.PopupButtonFocused.Underline(true).Render(btn.Text))
		case i == p.focusedButton:
			parts = append(parts, styles.PopupButtonFocused.Render(btn.Text))
		case i == p.hoveredButton:
			parts = append(parts, styles.PopupButtonHover.Render(btn.Text))
		default:
			parts = append(parts, styles.PopupButton.Render(btn.Text))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// computePosition calculates the screen position for the popup based on its
// anchor, the terminal dimensions, and the popup's own width/height.
func (p *Popup) computePosition(termW, termH, popupW, popupH int) (int, int) {
	ox, oy := p.anchorOrigin(termW, termH, popupW, popupH)
	x := ox + p.OffsetX
	y := oy + p.OffsetY

	// Clamp to screen bounds
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+popupW > termW {
		x = termW - popupW
	}
	if y+popupH > termH {
		y = termH - popupH
	}

	return x, y
}

// anchorOrigin returns the top-left origin point for the given anchor position,
// before OffsetX/OffsetY adjustments.
func (p *Popup) anchorOrigin(termW, termH, popupW, popupH int) (int, int) {
	switch p.Anchor {
	case AnchorTopLeft:
		return 0, 0
	case AnchorTopCenter:
		return (termW - popupW) / 2, 0
	case AnchorTopRight:
		return termW - popupW, 0
	case AnchorBottomLeft:
		return 0, termH - popupH
	case AnchorBottomCenter:
		return (termW - popupW) / 2, termH - popupH
	case AnchorBottomRight:
		return termW - popupW, termH - popupH
	case AnchorCustom:
		// Absolute positioning: OffsetX/OffsetY are the exact coordinates.
		// computePosition clamps to screen bounds automatically.
		return 0, 0
	case AnchorCenter:
		fallthrough
	default:
		return (termW - popupW) / 2, (termH - popupH) / 3
	}
}

// SetBounds stores the popup's screen position for mouse hit-testing.
func (p *Popup) SetBounds(x, y, w, h int) {
	p.x = x
	p.y = y
	p.width = w
	p.height = h
}

// HandleMouse processes a mouse event for the popup.
// Returns true if the event was handled (consumed), along with any
// side-effect commands (e.g., mouse pointer shape changes).
func (p *Popup) HandleMouse(msg tea.MouseMsg) (bool, tea.Cmd) {
	mouse := msg.Mouse()

	// ---- Hover tracking ----
	// Update hovered button state on any mouse event, tracking whether
	// the pointer is over a clickable button for visual feedback + cursor.
	oldHovered := p.hoveredButton
	btnIdx := p.buttonAt(mouse.X, mouse.Y)
	if btnIdx >= 0 && btnIdx < len(p.Buttons) {
		p.hoveredButton = btnIdx
	} else {
		p.hoveredButton = -1
	}
	hoverChanged := oldHovered != p.hoveredButton

	// Build commands for pointer shape change
	var hoverCmd tea.Cmd
	if hoverChanged {
		if p.hoveredButton >= 0 {
			hoverCmd = setMousePointer("pointer")
		} else {
			hoverCmd = setMousePointer("default")
		}
	}

	// ---- Drag movement ----
	if p.dragging {
		if mouse.Button == tea.MouseLeft {
			p.OffsetX = p.dragStartOffX + (mouse.X - p.dragMouseX)
			p.OffsetY = p.dragStartOffY + (mouse.Y - p.dragMouseY)
			return true, hoverCmd
		}
		// Button released → stop dragging
		p.dragging = false
		return true, hoverCmd
	}

	// If bounds haven't been set yet (first render hasn't happened),
	// assume the popup is visible and consume the event to prevent
	// premature dismissal as an "outside click".
	if p.x == 0 && p.y == 0 && p.width == 0 {
		return true, hoverCmd
	}

	// Check if mouse is inside popup bounds
	if p.x >= 0 && p.y >= 0 {
		inside := mouse.X >= p.x && mouse.X < p.x+p.width &&
			mouse.Y >= p.y && mouse.Y < p.y+p.height
		if !inside {
			// Mouse left the popup: reset hover
			if p.hoveredButton != -1 {
				p.hoveredButton = -1
				hoverCmd = setMousePointer("default")
			}
			return false, hoverCmd
		}

		// Left click inside popup — only respond to press events (MouseClickMsg),
		// not release events (MouseReleaseMsg), to avoid double-click on a
		// background menu item automatically clicking a popup button or starting
		// a drag when the popup covers the click position.
		if _, isClick := msg.(tea.MouseClickMsg); isClick && mouse.Button == tea.MouseLeft {
			// Check if clicking the title bar area (for drag)
			if mouse.Y == p.y || mouse.Y == p.y+1 {
				// Start drag: save current state
				p.dragging = true
				p.dragMouseX = mouse.X
				p.dragMouseY = mouse.Y
				p.dragStartOffX = p.OffsetX
				p.dragStartOffY = p.OffsetY
				return true, hoverCmd
			}

			if btnIdx := p.buttonAt(mouse.X, mouse.Y); btnIdx >= 0 {
				p.focusedButton = btnIdx
				btn := p.Buttons[btnIdx]
				p.result = &PopupResult{
					ButtonIndex: btnIdx,
					ButtonText:  btn.Text,
					IsCancel:    btn.IsCancel,
				}
				return true, hoverCmd
			}
		}

		// Wheel scroll for body content (if overflowing), otherwise cycle buttons
		if mouse.Button == tea.MouseWheelDown {
			if p.isBodyScrollable() {
				maxScroll := p.totalBodyLines - p.computeVisibleBodyLines()
				if p.scrollOffset < maxScroll {
					p.scrollOffset++
				}
				return true, hoverCmd
			}
			if len(p.Buttons) > 1 {
				p.focusedButton = (p.focusedButton + 1) % len(p.Buttons)
				return true, hoverCmd
			}
		}
		if mouse.Button == tea.MouseWheelUp {
			if p.isBodyScrollable() {
				if p.scrollOffset > 0 {
					p.scrollOffset--
				}
				return true, hoverCmd
			}
			if len(p.Buttons) > 1 {
				p.focusedButton--
				if p.focusedButton < 0 {
					p.focusedButton = len(p.Buttons) - 1
				}
				return true, hoverCmd
			}
		}

		// Non-clickable motion inside popup: still report handled + hover change
		if hoverChanged {
			return true, hoverCmd
		}
	}

	return false, hoverCmd
}

// setMousePointer returns a tea.Cmd that sends an OSC 22 escape sequence to
// change the terminal mouse pointer shape.
func setMousePointer(shape string) tea.Cmd {
	return func() tea.Msg {
		// Sync write directly to stdout to avoid race conditions with lipgloss output.
		print("\x1b]22;" + shape + "\x1b\\")
		return nil
	}
}

// buttonAt returns the index of the button at the given (mx, my) coordinate,
// or -1 if no button is at that position.
func (p *Popup) buttonAt(mx, my int) int {
	if len(p.Buttons) == 0 {
		return -1
	}
	// Buttons are in the bottom section of the popup: roughly at y = p.y + p.height - 3
	buttonY := p.y + p.height - 3
	if my < buttonY || my > buttonY+1 {
		return -1
	}
	// Distribute evenly across popup width
	relX := mx - p.x - 4 // border + padding on each side
	if relX < 0 {
		relX = 0
	}
	btnWidth := (p.width - 8) / len(p.Buttons)
	if btnWidth <= 0 {
		btnWidth = 1
	}
	btnIdx := relX / btnWidth
	if btnIdx < 0 {
		btnIdx = 0
	}
	if btnIdx >= len(p.Buttons) {
		btnIdx = len(p.Buttons) - 1
	}
	return btnIdx
}

// computeVisibleBodyLines returns the number of body lines that fit within MaxHeight.
func (p *Popup) computeVisibleBodyLines() int {
	if p.MaxHeight <= 0 {
		return p.totalBodyLines // no limit
	}
	borderLines := 2
	chromeLines := 0
	// We don't have accurate title/button line counts here, so estimate conservatively
	// title + buttons take ~4 lines total
	chromeLines = 4
	available := p.MaxHeight - borderLines - chromeLines
	if available < 1 {
		return 1
	}
	return available
}

// isBodyScrollable returns true if the popup body exceeds MaxHeight and needs scrolling.
func (p *Popup) isBodyScrollable() bool {
	if p.typ != PopupCustom || p.Body == "" || p.totalBodyLines == 0 {
		return false
	}
	return p.totalBodyLines > p.computeVisibleBodyLines()
}

