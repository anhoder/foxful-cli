package model

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/style"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const (
	popupFrameHorizontalOverhead = 4 // rounded border + 1-cell padding on each side
	popupFrameVerticalOverhead   = 4 // rounded border + 1-cell padding on each side
	popupFrameInset              = 2 // left/top border plus padding
)

// PopupAction is an explicit action rendered in a popup's action area.
type PopupAction struct {
	ID       string
	Label    string
	IsCancel bool
}

// PopupDismissCause describes how a popup was dismissed.
type PopupDismissCause uint8

const (
	PopupDismissAction PopupDismissCause = iota
	PopupDismissEscape
	PopupDismissOutsideClick
)

// PopupResult is passed to PopupSpec.OnResult after the popup is dismissed.
// ActionID is empty when dismissal did not trigger an action.
type PopupResult struct {
	ActionID string
	Cause    PopupDismissCause
}

// PopupAnchor controls where on the screen a popup appears.
type PopupAnchor int

const (
	AnchorCenter PopupAnchor = iota
	AnchorTopLeft
	AnchorTopCenter
	AnchorTopRight
	AnchorBottomLeft
	AnchorBottomCenter
	AnchorBottomRight
	AnchorCustom
)

// PopupSpec defines a popup before it is shown. Title and action labels are
// plain, single-line text. Content may be ANSI-styled; Popup preserves its
// foreground and text attributes while owning every content cell background.
type PopupSpec struct {
	Title     string
	Content   string
	Actions   []PopupAction
	MaxWidth  int // whole popup width, including border and padding; 0 = unlimited
	MaxHeight int // whole popup height, including border and padding; 0 = unlimited
	Anchor    PopupAnchor
	OffsetX   int
	OffsetY   int
	OnResult  func(PopupResult)
}

// Popup is an active modal dialog. Its configuration is immutable after
// construction; rendering and input state are private to the model package.
type Popup struct {
	title     string
	content   string
	actions   []PopupAction
	maxWidth  int
	maxHeight int
	anchor    PopupAnchor
	offsetX   int
	offsetY   int
	onResult  func(PopupResult)

	focusedAction int
	hoveredAction int
	result        *PopupResult

	scrollOffset        int
	totalContentLines   int
	visibleContentLines int

	dragging      bool
	dragMouseX    int
	dragMouseY    int
	dragStartOffX int
	dragStartOffY int

	bounds       popupRect
	boundsSet    bool
	actionBounds []popupRect
}

// NewPopup validates spec and constructs a popup ready for App.ShowPopup.
func NewPopup(spec PopupSpec) (*Popup, error) {
	if !isPopupPlainText(spec.Title) {
		return nil, fmt.Errorf("popup title must be single-line plain text")
	}
	if spec.MaxWidth < 0 || spec.MaxHeight < 0 {
		return nil, fmt.Errorf("popup dimensions must not be negative")
	}
	if spec.MaxWidth > 0 && spec.MaxWidth <= popupFrameHorizontalOverhead {
		return nil, fmt.Errorf("popup max width must exceed frame overhead")
	}
	if spec.MaxHeight > 0 && spec.MaxHeight <= popupFrameVerticalOverhead {
		return nil, fmt.Errorf("popup max height must exceed frame overhead")
	}

	actions := make([]PopupAction, len(spec.Actions))
	copy(actions, spec.Actions)
	ids := make(map[string]struct{}, len(actions))
	cancelCount := 0
	for _, action := range actions {
		if action.ID == "" {
			return nil, fmt.Errorf("popup action id must not be empty")
		}
		if action.Label == "" {
			return nil, fmt.Errorf("popup action %q label must not be empty", action.ID)
		}
		if !isPopupPlainText(action.Label) {
			return nil, fmt.Errorf("popup action %q label must be single-line plain text", action.ID)
		}
		if _, exists := ids[action.ID]; exists {
			return nil, fmt.Errorf("popup action id %q is duplicated", action.ID)
		}
		ids[action.ID] = struct{}{}
		if action.IsCancel {
			cancelCount++
		}
	}
	if cancelCount > 1 {
		return nil, fmt.Errorf("popup may have at most one cancel action")
	}

	return &Popup{
		title:         spec.Title,
		content:       spec.Content,
		actions:       actions,
		maxWidth:      spec.MaxWidth,
		maxHeight:     spec.MaxHeight,
		anchor:        spec.Anchor,
		offsetX:       spec.OffsetX,
		offsetY:       spec.OffsetY,
		onResult:      spec.OnResult,
		hoveredAction: -1,
	}, nil
}

func isPopupPlainText(value string) bool {
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\x1b' || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (p *Popup) update(msg tea.Msg) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return
	}

	if p.isContentScrollable() {
		switch keyMsg.String() {
		case "up", "k":
			p.scrollOffset = max(p.scrollOffset-1, 0)
			return
		case "down", "j":
			p.scrollOffset = min(p.scrollOffset+1, p.maxScrollOffset())
			return
		case "pgup":
			p.scrollOffset = max(p.scrollOffset-max(p.visibleContentLines/2, 1), 0)
			return
		case "pgdn":
			p.scrollOffset = min(p.scrollOffset+max(p.visibleContentLines/2, 1), p.maxScrollOffset())
			return
		case "home":
			p.scrollOffset = 0
			return
		case "end":
			p.scrollOffset = p.maxScrollOffset()
			return
		}
	}

	switch keyMsg.String() {
	case "esc":
		p.dismissCancel(PopupDismissEscape)
	case "enter":
		if len(p.actions) > 0 {
			p.dismissAction(p.focusedAction, PopupDismissAction)
		}
	case "tab", "right", "l", "L":
		if len(p.actions) > 1 {
			p.focusedAction = (p.focusedAction + 1) % len(p.actions)
		}
	case "left", "h", "H":
		if len(p.actions) > 1 {
			p.focusedAction = (p.focusedAction - 1 + len(p.actions)) % len(p.actions)
		}
	}
}

func (p *Popup) dismissed() bool {
	return p.result != nil
}

func (p *Popup) consumeResult() *PopupResult {
	result := p.result
	p.result = nil
	return result
}

func (p *Popup) dismissAction(index int, cause PopupDismissCause) {
	if p.result != nil || index < 0 || index >= len(p.actions) {
		return
	}
	p.result = &PopupResult{ActionID: p.actions[index].ID, Cause: cause}
}

func (p *Popup) dismissCancel(cause PopupDismissCause) {
	for i, action := range p.actions {
		if action.IsCancel {
			p.dismissAction(i, cause)
			return
		}
	}
	if p.result == nil {
		p.result = &PopupResult{Cause: cause}
	}
}

type popupRect struct {
	x int
	y int
	w int
	h int
}

func (r popupRect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type popupRender struct {
	content      string
	actionBounds []popupRect // relative to the popup's top-left corner
}

func (p *Popup) render(styles style.PopupStyleSet) popupRender {
	maxContentWidth := p.maxContentWidth()

	actions := p.renderActions(styles, maxContentWidth)

	content := ""
	contentLines := []string(nil)
	if p.content != "" {
		content = normalizePopupSurface(styles.Content.Render(p.content), styles.Surface)
		contentLines = strings.Split(content, "\n")
	}
	p.totalContentLines = len(contentLines)

	visibleHeight := len(contentLines)
	if len(contentLines) > 0 && p.maxHeight > 0 {
		visibleHeight = max(1, p.maxHeight-popupFrameVerticalOverhead-actions.height)
		visibleHeight = min(visibleHeight, len(contentLines))
	}
	p.visibleContentLines = visibleHeight
	p.scrollOffset = min(p.scrollOffset, p.maxScrollOffset())

	visibleLines := contentLines
	if visibleHeight < len(contentLines) {
		visibleLines = contentLines[p.scrollOffset : p.scrollOffset+visibleHeight]
	}
	body, bodyWidth := renderPopupBody(visibleLines, len(contentLines) > visibleHeight, p.scrollOffset, p.maxScrollOffset(), maxContentWidth, styles)
	bodyHeight := textHeight(body)

	innerWidth := max(bodyWidth, actions.width)
	blocks := make([]string, 0, 2)
	if body != "" {
		blocks = append(blocks, lipgloss.NewStyle().Width(innerWidth).Background(styles.Surface).Render(body))
	}
	if actions.content != "" {
		blocks = append(blocks, lipgloss.NewStyle().Width(innerWidth).Background(styles.Surface).Render(actions.content))
	}
	inner := lipgloss.JoinVertical(lipgloss.Left, blocks...)
	framed := styles.Frame.Render(inner)

	if p.title != "" {
		framed = embedTitleInTopBorder(framed, p.title, styles)
	}

	actionY := popupFrameInset + bodyHeight
	actionX := popupFrameInset + (innerWidth-actions.width)/2
	actionBounds := make([]popupRect, len(actions.bounds))
	for i, bound := range actions.bounds {
		actionBounds[i] = popupRect{
			x: actionX + bound.x,
			y: actionY + bound.y,
			w: bound.w,
			h: bound.h,
		}
	}

	return popupRender{
		content:      framed,
		actionBounds: actionBounds,
	}
}

func embedTitleInTopBorder(framed, title string, styles style.PopupStyleSet) string {
	screen := popupStyledScreen(framed)
	if len(screen.Lines) == 0 {
		return framed
	}

	titleRunes := []rune(title)
	frameWidth := len(screen.Lines[0])
	availableWidth := frameWidth - 4
	if availableWidth < 1 || len(titleRunes) == 0 {
		return framed
	}

	displayTitle := titleRunes
	if len(displayTitle) > availableWidth {
		displayTitle = append(displayTitle[:availableWidth-1], '…')
	}

	titleStart := (frameWidth - len(displayTitle)) / 2
	titleStyle := styles.Title.GetForeground()
	for i, r := range displayTitle {
		x := titleStart + i
		if x >= 1 && x < frameWidth-1 {
			cell := &uv.Cell{
				Content: string(r),
				Width:   1,
				Style: uv.Style{
					Fg: titleStyle,
					Bg: styles.Surface,
				},
			}
			screen.Lines[0].Set(x, cell)
		}
	}

	return screen.Render()
}

func (p *Popup) maxContentWidth() int {
	if p.maxWidth == 0 {
		return 0
	}
	return max(1, p.maxWidth-popupFrameHorizontalOverhead)
}

type popupActionsRender struct {
	content string
	width   int
	height  int
	bounds  []popupRect // relative to the action block
}

func (p *Popup) renderActions(styles style.PopupStyleSet, maxWidth int) popupActionsRender {
	if len(p.actions) == 0 {
		return popupActionsRender{}
	}

	type actionRow struct {
		content strings.Builder
		width   int
		height  int
	}

	rows := []actionRow{{}}
	bounds := make([]popupRect, len(p.actions))
	for i, action := range p.actions {
		buttonStyle := styles.Action
		switch {
		case i == p.focusedAction && i == p.hoveredAction:
			buttonStyle = styles.ActionFocused.Underline(true)
		case i == p.focusedAction:
			buttonStyle = styles.ActionFocused
		case i == p.hoveredAction:
			buttonStyle = styles.ActionHover
		}
		button := buttonStyle.Render(action.Label)
		buttonWidth := lipgloss.Width(button)
		if maxWidth > 0 && buttonWidth > maxWidth {
			button = lipgloss.NewStyle().MaxWidth(maxWidth).Render(button)
			buttonWidth = lipgloss.Width(button)
		}

		row := &rows[len(rows)-1]
		gap := 0
		if row.width > 0 {
			gap = 1
		}
		if maxWidth > 0 && row.width > 0 && row.width+gap+buttonWidth > maxWidth {
			rows = append(rows, actionRow{})
			row = &rows[len(rows)-1]
			gap = 0
		}
		if gap > 0 {
			row.content.WriteString(lipgloss.NewStyle().Background(styles.Surface).Render(" "))
		}
		x := row.width + gap
		row.content.WriteString(button)
		row.width += gap + buttonWidth
		row.height = max(row.height, textHeight(button))
		bounds[i] = popupRect{x: x, y: len(rows) - 1, w: buttonWidth, h: textHeight(button)}
	}

	width := 0
	for _, row := range rows {
		width = max(width, row.width)
	}
	rowTexts := make([]string, len(rows))
	height := 0
	for i, row := range rows {
		rowTexts[i] = lipgloss.NewStyle().Width(width).Background(styles.Surface).Render(row.content.String())
		height += max(row.height, 1)
	}
	return popupActionsRender{
		content: strings.Join(rowTexts, "\n"),
		width:   width,
		height:  height,
		bounds:  bounds,
	}
}

func renderPopupBody(lines []string, scrolling bool, offset, maxOffset, maxWidth int, styles style.PopupStyleSet) (string, int) {
	if len(lines) == 0 {
		return "", 0
	}
	if !scrolling {
		if maxWidth == 0 {
			return strings.Join(lines, "\n"), widestLine(lines)
		}
		output := make([]string, len(lines))
		for i, line := range lines {
			output[i] = lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
		}
		return strings.Join(output, "\n"), widestLine(output)
	}

	bodyWidth := widestLine(lines)
	if maxWidth > 0 {
		bodyWidth = min(bodyWidth, max(maxWidth-2, 1))
	}

	thumbLine := 0
	if maxOffset > 0 {
		thumbLine = offset * (len(lines) - 1) / maxOffset
	}
	output := make([]string, len(lines))
	for i, line := range lines {
		line = lipgloss.NewStyle().MaxWidth(bodyWidth).Render(line)
		line = lipgloss.NewStyle().Width(bodyWidth).Render(line)
		scrollbar := styles.ScrollTrack.Render("│")
		if i == thumbLine {
			scrollbar = styles.ScrollThumb.Render("█")
		}
		output[i] = line + " " + scrollbar
	}
	return strings.Join(output, "\n"), bodyWidth + 2
}

func widestLine(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func textHeight(content string) int {
	if content == "" {
		return 0
	}
	return lipgloss.Height(content)
}

// normalizePopupSurface preserves rendered content's foreground, text
// attributes, hyperlinks, and grapheme widths while replacing every cell
// background. Reverse video is removed because it is an implicit background.
func normalizePopupSurface(content string, surface color.Color) string {
	if content == "" {
		return ""
	}

	screen := popupStyledScreen(content)
	for y := range screen.Lines {
		for x := range screen.Lines[y] {
			cell := screen.CellAt(x, y)
			if cell == nil || cell.IsZero() {
				continue
			}
			cell.Style.Bg = surface
			cell.Style.Attrs &^= uv.AttrReverse
		}
	}
	return screen.Render()
}

func popupStyledScreen(content string) uv.ScreenBuffer {
	width := max(lipgloss.Width(content), 1)
	height := strings.Count(content, "\n") + 1
	screen := uv.NewScreenBuffer(width, height)
	screen.Method = ansi.GraphemeWidth
	uv.NewStyledString(content).Draw(screen, screen.Bounds())
	return screen
}

func (p *Popup) setBounds(x, y, w, h int, actionBounds []popupRect) {
	p.bounds = popupRect{x: x, y: y, w: w, h: h}
	p.boundsSet = true
	p.actionBounds = make([]popupRect, len(actionBounds))
	for i, bound := range actionBounds {
		p.actionBounds[i] = popupRect{
			x: x + bound.x,
			y: y + bound.y,
			w: bound.w,
			h: bound.h,
		}
	}
}

func (p *Popup) handleMouse(msg tea.MouseMsg) (bool, tea.Cmd) {
	mouse := msg.Mouse()
	oldHovered := p.hoveredAction
	p.hoveredAction = p.actionAt(mouse.X, mouse.Y)
	hoverChanged := oldHovered != p.hoveredAction

	var hoverCmd tea.Cmd
	if hoverChanged {
		if p.hoveredAction >= 0 {
			hoverCmd = setMousePointer("pointer")
		} else {
			hoverCmd = setMousePointer("default")
		}
	}

	if p.dragging {
		if mouse.Button == tea.MouseLeft {
			p.offsetX = p.dragStartOffX + mouse.X - p.dragMouseX
			p.offsetY = p.dragStartOffY + mouse.Y - p.dragMouseY
			return true, hoverCmd
		}
		p.dragging = false
		return true, hoverCmd
	}

	if !p.boundsSet {
		return true, hoverCmd
	}
	if !p.bounds.contains(mouse.X, mouse.Y) {
		if p.hoveredAction != -1 {
			p.hoveredAction = -1
			hoverCmd = setMousePointer("default")
		}
		return false, hoverCmd
	}

	if _, isClick := msg.(tea.MouseClickMsg); isClick && mouse.Button == tea.MouseLeft {
		if mouse.Y == p.bounds.y || mouse.Y == p.bounds.y+1 {
			p.dragging = true
			p.dragMouseX = mouse.X
			p.dragMouseY = mouse.Y
			p.dragStartOffX = p.offsetX
			p.dragStartOffY = p.offsetY
			return true, hoverCmd
		}
		if action := p.actionAt(mouse.X, mouse.Y); action >= 0 {
			p.focusedAction = action
			p.dismissAction(action, PopupDismissAction)
			return true, hoverCmd
		}
		return true, hoverCmd
	}

	if mouse.Button == tea.MouseWheelDown {
		if p.isContentScrollable() {
			p.scrollOffset = min(p.scrollOffset+1, p.maxScrollOffset())
			return true, hoverCmd
		}
		if len(p.actions) > 1 {
			p.focusedAction = (p.focusedAction + 1) % len(p.actions)
			return true, hoverCmd
		}
	}
	if mouse.Button == tea.MouseWheelUp {
		if p.isContentScrollable() {
			p.scrollOffset = max(p.scrollOffset-1, 0)
			return true, hoverCmd
		}
		if len(p.actions) > 1 {
			p.focusedAction = (p.focusedAction - 1 + len(p.actions)) % len(p.actions)
			return true, hoverCmd
		}
	}

	return true, hoverCmd
}

func setMousePointer(shape string) tea.Cmd {
	return func() tea.Msg {
		print("\x1b]22;" + shape + "\x1b\\")
		return nil
	}
}

func (p *Popup) actionAt(x, y int) int {
	for i, bound := range p.actionBounds {
		if bound.contains(x, y) {
			return i
		}
	}
	return -1
}

func (p *Popup) isContentScrollable() bool {
	return p.totalContentLines > p.visibleContentLines
}

func (p *Popup) maxScrollOffset() int {
	return max(p.totalContentLines-p.visibleContentLines, 0)
}

func (p *Popup) computePosition(termW, termH, popupW, popupH int) (int, int) {
	ox, oy := p.anchorOrigin(termW, termH, popupW, popupH)
	x := ox + p.offsetX
	y := oy + p.offsetY

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

func (p *Popup) anchorOrigin(termW, termH, popupW, popupH int) (int, int) {
	switch p.anchor {
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
		return 0, 0
	default:
		return (termW - popupW) / 2, (termH - popupH) / 3
	}
}
