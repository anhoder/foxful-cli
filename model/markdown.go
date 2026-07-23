package model

import (
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

// MarkdownOption is a functional option for MarkdownComponent.
type MarkdownOption func(*MarkdownComponent)

// WithMarkdownStyle sets the glamour rendering style (e.g., "dark", "light", "dracula").
func WithMarkdownStyle(style string) MarkdownOption {
	return func(m *MarkdownComponent) { m.style = style }
}

// WithMarkdownEmoji enables emoji rendering.
func WithMarkdownEmoji(enable bool) MarkdownOption {
	return func(m *MarkdownComponent) { m.emoji = enable }
}

// WithMarkdownWordWrap sets a fixed word wrap width. 0 means auto-detect from window width (default).
func WithMarkdownWordWrap(width int) MarkdownOption {
	return func(m *MarkdownComponent) { m.wrapWidth = width }
}

// MarkdownComponent renders markdown content in the terminal using Glamour v2.
type MarkdownComponent struct {
	content   string
	style     string // default "dark"
	emoji     bool
	wrapWidth int  // 0 = auto (use window width), >0 = fixed
	renderer  *glamour.TermRenderer
	lastWidth int // last window width used to build renderer
}

// NewMarkdown creates a new MarkdownComponent with the given content and options.
func NewMarkdown(content string, opts ...MarkdownOption) *MarkdownComponent {
	m := &MarkdownComponent{
		content: content,
		style:   "dark",
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// SetContent updates the markdown content to render.
func (m *MarkdownComponent) SetContent(content string) {
	m.content = content
	m.renderer = nil // reset renderer so it rebuilds on next View()
}

// Content returns the current markdown content.
func (m *MarkdownComponent) Content() string {
	return m.content
}

// RenderToString renders the markdown content to a string with the specified width.
// This method is independent of the App context and can be used for popup content.
// If width is 0, uses a default width of 80 characters.
func (m *MarkdownComponent) RenderToString(width int) (string, error) {
	if m.content == "" {
		return "", nil
	}

	renderWidth := m.wrapWidth
	if renderWidth == 0 {
		renderWidth = width
		if renderWidth <= 0 {
			renderWidth = 80 // default width
		}
	}

	// Build a fresh renderer for the specified width
	var opts []glamour.TermRendererOption
	opts = append(opts, glamour.WithStylePath(m.style))
	opts = append(opts, glamour.WithWordWrap(renderWidth))
	if m.emoji {
		opts = append(opts, glamour.WithEmoji())
	}
	opts = append(opts, glamour.WithPreservedNewLines())

	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return m.content, err
	}

	return r.Render(m.content)
}

// Update handles resize events to rebuild the renderer when width changes.
// Note: The framework does not currently call Component.Update from Main.Update.
// This is here for future use or manual invocation.
func (m *MarkdownComponent) Update(msg tea.Msg, a *App) {
	// no-op for now; width is handled in View()
}

// View renders the markdown content. It lazily creates/rebuilds the glamour
// renderer when the window width changes and no fixed wrap width is set.
func (m *MarkdownComponent) View(a *App, main *Main) (string, int) {
	if m.content == "" {
		return "", 0
	}

	w := a.WindowWidth()
	renderWidth := m.wrapWidth
	if renderWidth == 0 {
		renderWidth = w
		if renderWidth < 20 {
			renderWidth = 20
		}
	}

	// Rebuild renderer when width changes or on first use
	if m.renderer == nil || (m.wrapWidth == 0 && w != m.lastWidth) {
		var opts []glamour.TermRendererOption
		opts = append(opts, glamour.WithStylePath(m.style))
		opts = append(opts, glamour.WithWordWrap(renderWidth))
		if m.emoji {
			opts = append(opts, glamour.WithEmoji())
		}
		// Use preserved newlines for consistent rendering
		opts = append(opts, glamour.WithPreservedNewLines())

		r, err := glamour.NewTermRenderer(opts...)
		if err != nil {
			return m.content, lipgloss.Height(m.content)
		}
		m.renderer = r
		m.lastWidth = w
	}

	out, err := m.renderer.Render(m.content)
	if err != nil {
		return m.content, lipgloss.Height(m.content)
	}

	return out, lipgloss.Height(out)
}

// MarkdownPopupSpec defines a popup that displays Markdown content.
type MarkdownPopupSpec struct {
	Title           string
	MarkdownContent string
	Actions         []PopupAction // nil = default Close button, empty slice = no buttons
	MarkdownStyle   string        // default "dark"
	MarkdownEmoji   bool          // enable emoji rendering
	MaxWidth        int           // whole popup width, including border and padding; 0 = unlimited
	MaxHeight       int           // whole popup height, including border and padding; 0 = unlimited
	Anchor          PopupAnchor
	OffsetX         int
	OffsetY         int
	OnResult        func(PopupResult)
	DisableResize   bool // when true, disable mouse-driven resize for this popup
}

// NewMarkdownPopup creates a popup that displays rendered Markdown content.
// If spec.Actions is nil, a default "Close" button is added.
// If spec.Actions is an empty slice, no buttons are added.
func NewMarkdownPopup(spec MarkdownPopupSpec) (*Popup, error) {
	// Determine render width from MaxWidth
	renderWidth := spec.MaxWidth
	if renderWidth > 0 {
		// Account for popup frame overhead
		renderWidth = renderWidth - popupFrameHorizontalOverhead
		if renderWidth < 20 {
			renderWidth = 20
		}
	} else {
		// Use default width if not specified
		renderWidth = 80
	}

	// Create markdown component with options
	style := spec.MarkdownStyle
	if style == "" {
		style = "dark"
	}

	md := NewMarkdown(
		spec.MarkdownContent,
		WithMarkdownStyle(style),
		WithMarkdownEmoji(spec.MarkdownEmoji),
		WithMarkdownWordWrap(renderWidth),
	)

	// Render markdown to string
	rendered, err := md.RenderToString(renderWidth)
	if err != nil {
		return nil, err
	}

	// Handle Actions: nil = default Close button, empty slice = no buttons
	actions := spec.Actions
	if actions == nil {
		actions = []PopupAction{
			{ID: "close", Label: "Close", IsCancel: true},
		}
	}

	// Create popup with rendered markdown content
	return NewPopup(PopupSpec{
		Title:         spec.Title,
		Content:       rendered,
		Actions:       actions,
		MaxWidth:      spec.MaxWidth,
		MaxHeight:     spec.MaxHeight,
		Anchor:        spec.Anchor,
		OffsetX:       spec.OffsetX,
		OffsetY:       spec.OffsetY,
		OnResult:      spec.OnResult,
		DisableResize: spec.DisableResize,
	})
}
