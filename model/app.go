package model

import (
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/layout"
	"github.com/anhoder/foxful-cli/style"
	"github.com/anhoder/foxful-cli/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

type App struct {
	windowWidth  int
	windowHeight int
	options      *Options
	quiting      bool

	program *tea.Program

	startup *StartupPage
	main    *Main

	page       Page    // current page
	modalStack []Modal // stack of active modals (popups, context menus); topmost is last

	notifications      []*Notification // active notifications (newest at end)
	nextNotificationID NotificationID

	listeningKBEventL    sync.Mutex
	listeningMouseEventL sync.Mutex
}

// NewApp create application
func NewApp(options *Options) (a *App) {
	a = &App{
		options: options,
		page:    options.InitPage,
	}

	runewidth.DefaultCondition.EastAsianWidth = false

	return
}

func (a *App) With(w ...WithOption) *App {
	for _, item := range w {
		if item != nil {
			item(a.options)
		}
	}
	return a
}

func (a *App) Init() tea.Cmd {
	if a.options.InitHook != nil {
		a.options.InitHook(a)
	}
	if a.options.Ticker != nil {
		go func() {
			for range a.options.Ticker.Ticker() {
				a.Rerender(false)
			}
		}()
		if err := a.options.Ticker.Start(); err != nil {
			panic("Fail to start ticker: " + err.Error())
		}
	}

	var cmds []tea.Cmd
	// Request initial terminal background color to seed light/dark detection.
	// Handled in Update via BackgroundColorMsg.
	cmds = append(cmds, func() tea.Msg {
		return tea.RequestBackgroundColor()
	})
	// Enable DEC 2031 mode so the terminal proactively reports system
	// light/dark scheme changes. When enabled, the terminal sends a
	// ModeReportMsg with Mode=DECMode(2031) on every color scheme switch.
	// Supported by kitty, WezTerm, Ghostty, iTerm2, and others.
	cmds = append(cmds, tea.Raw(ansi.SetModeLightDark))

	if initPage, ok := a.page.(InitPage); ok {
		cmds = append(cmds, initPage.Init(a))
	}
	return tea.Batch(cmds...)
}

func (a *App) Close() {
	// Reset terminal mouse pointer to default on exit
	resetMousePointer()

	if a.options.CloseHook != nil {
		a.options.CloseHook(a)
	}
	if closer, ok := a.page.(Closer); ok {
		_ = closer.Close()
	}
	if a.options.Ticker != nil {
		_ = a.options.Ticker.Close()
	}
}

// SetMousePointer returns a tea.Cmd that sends an OSC 22 escape sequence to
// change the terminal mouse pointer shape when hovering over the terminal
// window. Supported by Kitty, WezTerm, Ghostty, iTerm2, Terminal.app, and
// others. Unsupported terminals silently ignore the sequence.
//
// Shape names follow CSS cursor conventions. Common values:
//   - "default"  — normal arrow pointer
//   - "pointer"  — pointing hand (for clickable elements)
//   - "text"     — I-beam (for editable text)
//   - "wait"     — wait spinner (for busy state)
//   - "crosshair" — crosshair (for grid selection)
func (a *App) SetMousePointer(shape string) tea.Cmd {
	return func() tea.Msg {
		os.Stdout.WriteString("\x1b]22;" + shape + "\x1b\\")
		return nil
	}
}

// resetMousePointer writes the OSC 22 reset sequence directly to stdout.
// Used synchronously during shutdown where tea.Cmd cannot be returned.
func resetMousePointer() {
	os.Stdout.WriteString("\x1b]22;\x1b\\")
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		if !a.listeningKBEventL.TryLock() {
			return a, nil
		}
		defer a.listeningKBEventL.Unlock()
	} else if _, ok := msg.(tea.MouseMsg); ok {
		if !a.listeningMouseEventL.TryLock() {
			return a, nil
		}
		defer a.listeningMouseEventL.Unlock()
	}

	// Make sure these keys always quit
	switch msgWithType := msg.(type) {
	case tea.KeyPressMsg:
		k := msgWithType.String()
		if k != "q" && k != "Q" && k != "ctrl+c" {
			break
		}
		if a.page != nil && a.page.IgnoreQuitKeyMsg(msgWithType) {
			break
		}
		a.Close()
		a.quiting = true
		return a, tea.Quit
	case tea.WindowSizeMsg:
		a.windowHeight = msgWithType.Height
		a.windowWidth = msgWithType.Width
	case uv.LightColorSchemeEvent:
		a.onBackgroundChanged(false)
		return a, a.RerenderCmd(true)
	case uv.DarkColorSchemeEvent:
		a.onBackgroundChanged(true)
		return a, a.RerenderCmd(true)
	case tea.BackgroundColorMsg:
		a.onBackgroundChanged(msgWithType.IsDark())
		return a, a.RerenderCmd(true)
	case tea.ModeReportMsg:
		// DEC 2031 mode report: terminal proactively reports system color
		// scheme changes when DEC 2031 is enabled (see Init).
		if msgWithType.Mode.Mode() == int(ansi.ModeLightDark) {
			// ModeSet (value 1) = dark, ModeReset (value 2) = light
			a.onBackgroundChanged(msgWithType.Value.IsSet())
			return a, a.RerenderCmd(true)
		}
	}

	// Notification messages are handled before modal interception so they work
	// regardless of any open modal. These are never Key/Mouse messages, so the
	// locks at the top of Update do not apply.
	switch msgWithType := msg.(type) {
	case ShowNotificationMsg:
		return a, a.handleShowNotification(msgWithType.Spec)
	case notificationExpireMsg:
		a.handleExpire(msgWithType.id)
		return a, a.RerenderCmd(true)
	case updateNotificationMsg:
		cmd := a.updateNotificationContent(msgWithType.id, msgWithType.spec)
		if cmd != nil {
			return a, tea.Batch(a.RerenderCmd(true), cmd)
		}
		return a, a.RerenderCmd(true)
	case dismissNotificationMsg:
		a.removeNotification(msgWithType.id)
		return a, a.RerenderCmd(true)
	case clearAllNotificationsMsg:
		a.notifications = nil
		return a, a.RerenderCmd(true)
	}

	// Notification mouse handling — checked before modal/page routing.
	// Notifications intercept mouse events on their bounds: clicks on title
	// dismiss, clicks on content handle text selection, motion/release extend
	// and finalize the selection.
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if notif := a.notificationAt(mouseMsg.Mouse()); notif != nil {
			consumed, dismiss, notifCmd := notif.handleMouse(mouseMsg)
			if consumed {
				var cmds []tea.Cmd
				if dismiss {
					a.removeNotification(notif.id)
					cmds = append(cmds, a.RerenderCmd(true))
				}
				if notifCmd != nil {
					cmds = append(cmds, notifCmd)
				}
				if len(cmds) > 0 {
					return a, tea.Batch(cmds...)
				}
				return a, nil
			}
		}
	}

	// Modal input interception — only the topmost modal receives input.
	if len(a.modalStack) > 0 {
		top := a.modalStack[len(a.modalStack)-1]
		switch msg := msg.(type) {
		case tea.KeyMsg:
			top.update(msg)
			if top.dismissed() {
				page, cmd := a.completeTopModal()
				if page != nil {
					a.page = page
				}
				cmds := []tea.Cmd{a.RerenderCmd(true)}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return a, tea.Batch(cmds...)
			}
			return a, a.RerenderCmd(true)
		case tea.MouseMsg:
			handled, mouseCmd := top.handleMouse(msg)
			if handled {
				if top.dismissed() {
					page, actionCmd := a.completeTopModal()
					if page != nil {
						a.page = page
					}
					cmds := []tea.Cmd{a.RerenderCmd(true)}
					if mouseCmd != nil {
						cmds = append(cmds, mouseCmd)
					}
					if actionCmd != nil {
						cmds = append(cmds, actionCmd)
					}
					return a, tea.Batch(cmds...)
				}
				cmds := []tea.Cmd{a.RerenderCmd(true)}
				if mouseCmd != nil {
					cmds = append(cmds, mouseCmd)
				}
				return a, tea.Batch(cmds...)
			}
			// Click outside the topmost modal: dismiss behavior depends on modal type.
			if _, isClick := msg.(tea.MouseClickMsg); !isClick {
				// Non-click (e.g. motion) outside the modal is not consumed, but a
				// returned pointer-reset command must still be honored.
				if mouseCmd != nil {
					return a, mouseCmd
				}
				return a, nil
			}
			mouse := msg.Mouse()
			if mouse.Button == tea.MouseLeft {
				// Left-click outside dismisses and consumes the click
				top.dismissOutside()
				page, cmd := a.completeTopModal()
				if page != nil {
					a.page = page
				}
				cmds := []tea.Cmd{a.RerenderCmd(true)}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return a, tea.Batch(cmds...)
			} else if mouse.Button == tea.MouseRight {
				// Right-click outside: dismiss if modal allows passthrough, then forward to page
				if top.allowsRightClickPassthrough() {
					top.dismissOutside()
					page, modalCmd := a.completeTopModal()
					if page != nil {
						a.page = page
					}
					// Forward the right-click to the page
					newPage, pageCmd := a.page.Update(msg, a)
					if newPage != nil {
						a.page = newPage
					}
					cmds := []tea.Cmd{a.RerenderCmd(true)}
					if modalCmd != nil {
						cmds = append(cmds, modalCmd)
					}
					if pageCmd != nil {
						cmds = append(cmds, pageCmd)
					}
					return a, tea.Batch(cmds...)
				}
			}
			return a, nil
		}
		// Forward non-input messages (ticks, etc.) to the page so it continues
		// updating while a modal is open.
	}

	page, cmd := a.page.Update(msg, a)
	if page != nil {
		a.page = page
	}
	return a, cmd
}

func (a *App) View() tea.View {
	var v tea.View
	v.AltScreen = a.options.AltScreen
	v.MouseMode = a.options.MouseMode

	if a.quiting || a.WindowHeight() <= 0 || a.WindowWidth() <= 0 || a.page == nil {
		return v
	}

	baseContent := a.page.View(a)

	// Composite modals on top of the page content (if any).
	if len(a.modalStack) > 0 {
		baseContent = a.compositeModals(baseContent)
	}

	// Composite notifications on top of everything (page + modals).
	if len(a.notifications) > 0 {
		baseContent = a.compositeNotifications(baseContent)
	}

	v.SetContent(baseContent)
	return v
}

// resolveTheme selects the appropriate theme based on the configured options
// and current terminal background detection.
//
// Priority:
//  1. Both DarkTheme and LightTheme configured → auto-select based on detectedBg.
//  2. Neither configured → use DefaultTheme() (auto-adaptive).
func (a *App) resolveTheme() style.Theme {
	if a.options.DarkTheme.Primary != nil && a.options.LightTheme.Primary != nil {
		if style.HasDarkBackground() {
			return a.options.DarkTheme
		}
		return a.options.LightTheme
	}
	return style.DefaultTheme()
}

// onBackgroundChanged handles a detected change in terminal background
// color (light/dark). Updates the cached detection and rebuilds the
// StyleSet so all rendered UI elements switch theme immediately.
func (a *App) onBackgroundChanged(isDark bool) {
	style.SetDarkBackground(isDark)
	style.SetStyleSet(style.NewStyleSet(a.resolveTheme()))
}

func (a *App) Run() error {
	util.PrimaryColor = a.options.PrimaryColor

	// Detect terminal background color synchronously at startup.
	// This seeds the cached value used by DefaultTheme() before the
	// first render, avoiding a flash. Runtime updates arrive via
	// BackgroundColorMsg in Update().
	style.SetDarkBackground(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))

	// Initialize the global StyleSet from the configured theme.
	style.SetStyleSet(style.NewStyleSet(a.resolveTheme()))

	if a.page == nil {
		a.main = NewMain(a, a.options)
		a.startup = NewStartup(&a.options.StartupOptions, a.main)
		if a.options.InitPage == nil {
			a.options.InitPage = a.main
			if a.options.EnableStartup {
				a.options.InitPage = a.startup
			}
		}
		a.page = a.options.InitPage
	}

	if len(a.options.GlobalKeyHandlers) > 0 {
		ListenGlobalKeys(a, a.options.GlobalKeyHandlers)
	}

	a.options.TeaOptions = append(a.options.TeaOptions, tea.WithHardTabs(false), tea.WithFoxfulRenderer())
	a.program = tea.NewProgram(a, a.options.TeaOptions...)
	_, err := a.program.Run()
	return err
}

func (a *App) Rerender(cleanScreen bool) {
	if a.program == nil {
		return
	}
	// Send in a goroutine to avoid blocking on the unbuffered msgs channel.
	// This is called from goroutines (e.g., ticker) and must not deadlock
	// when the event loop is busy or hasn't started yet.
	go func() {
		if cleanScreen {
			a.program.Send(tea.ClearScreen())
		}
		if a.page != nil {
			a.program.Send(a.page.Msg())
		}
	}()
}

func (a *App) RerenderCmd(cleanScreen bool) tea.Cmd {
	return func() tea.Msg {
		if cleanScreen {
			a.program.Send(tea.ClearScreen())
		}
		if a.page == nil {
			return nil
		}
		return a.page.Msg()
	}
}

func (a *App) Tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		if a.page == nil {
			return nil
		}
		return a.page.Msg()
	})
}

func (a *App) WindowWidth() int {
	return a.windowWidth
}

func (a *App) WindowHeight() int {
	return a.windowHeight
}

func (a *App) CurPage() Page {
	return a.page
}

func (a *App) Startup() *StartupPage {
	return a.startup
}

func (a *App) Main() *Main {
	return a.main
}

func (a *App) MustMain() *Main {
	if a.main != nil {
		return a.main
	}
	panic("main page is empty")
}

func (a *App) MustStartup() *StartupPage {
	if a.startup != nil {
		return a.startup
	}
	panic("startup page is empty")
}

func (a *App) Options() *Options {
	return a.options
}

// Quit gracefully shuts down the application. It calls Close() to run any
// registered close hooks, then sends the quit signal to the bubbletea program.
func (a *App) Quit() {
	a.Close()
	a.quiting = true
	if a.program != nil {
		a.program.Send(tea.Quit())
	}
}

func (a *App) pushModal(m Modal) {
	if m == nil {
		panic("cannot push a nil modal")
	}
	a.modalStack = append(a.modalStack, m)
}

// ShowPopup pushes a validated popup onto the modal stack.
// The topmost modal receives input first.
func (a *App) ShowPopup(p *Popup) {
	if p == nil {
		panic("cannot show a nil popup")
	}
	a.pushModal(p)
}

// DismissPopup dismisses the topmost modal on the stack.
// Does nothing if the stack is empty.
func (a *App) DismissPopup() {
	if len(a.modalStack) > 0 {
		a.modalStack = a.modalStack[:len(a.modalStack)-1]
	}
}

// completeTopModal pops the topmost modal, calls its complete() method,
// and returns the resulting (Page, tea.Cmd).
func (a *App) completeTopModal() (Page, tea.Cmd) {
	if len(a.modalStack) == 0 {
		return nil, nil
	}
	topIndex := len(a.modalStack) - 1
	top := a.modalStack[topIndex]
	a.modalStack = a.modalStack[:topIndex]
	return top.complete(a)
}

// HasPopup returns whether a modal (popup or context menu) is currently active.
func (a *App) HasPopup() bool {
	return len(a.modalStack) > 0
}

// compositeModals renders the base page content with all modals in the stack
// overlaid using lipgloss Compositor layers. Modals are rendered in stack
// order (bottom of stack = back layer, top of stack = front layer).
func (a *App) compositeModals(baseContent string) string {
	w, h := a.WindowWidth(), a.WindowHeight()
	ss := style.CurrentStyleSet()

	layers := []*layout.Layer{layout.NewLayer(baseContent)}
	for _, modal := range a.modalStack {
		// Type-switch to render Popup vs ContextMenu
		switch m := modal.(type) {
		case *Popup:
			rendered := m.render(ss.Popup)
			popupH := lipgloss.Height(rendered.content)
			popupW := layout.Width(rendered.content)
			x, y := m.computePosition(w, h, popupW, popupH)
			m.setBounds(x, y, popupW, popupH, rendered.actionBounds)
			m.SetTermSize(w, h)
			layers = append(layers, layout.NewLayer(rendered.content).X(x).Y(y))
		case *ContextMenu:
			rendered := m.renderModal(ss)
			menuH := lipgloss.Height(rendered.content)
			menuW := layout.Width(rendered.content)
			x, y := m.computePosition(w, h, menuW, menuH)
			m.setModalBounds(x, y, menuW, menuH, rendered.itemBounds)
			layers = append(layers, layout.NewLayer(rendered.content).X(x).Y(y))
		}
	}
	return layout.NewCompositor(layers...).Render()
}

// ---- Notification API ----

// Notify displays a notification and returns its ID. For Info/Success levels,
// the notification auto-dismisses after the configured timeout unless spec.Timeout
// is explicitly set. For Warning/Error levels, the notification persists until
// dismissed manually or replaced by newer notifications beyond the screen limit.
//
// Safe to call from goroutines, including during Init(); internally sends a
// message to the Update loop via a non-blocking goroutine to avoid deadlocks
// when called before the event loop starts.
func (a *App) Notify(spec NotificationSpec) NotificationID {
	if a.program == nil {
		return 0
	}
	// Assign ID optimistically for return (actual assignment happens in Update).
	// This is a heuristic; for guaranteed ID tracking, use the returned ID.
	nextID := a.nextNotificationID + 1
	go a.program.Send(ShowNotificationMsg{Spec: spec})
	return nextID
}

// UpdateNotification updates the content of an existing notification.
// Does nothing if the ID does not exist.
//
// Safe to call from goroutines, including during Init().
func (a *App) UpdateNotification(id NotificationID, spec NotificationSpec) {
	if a.program == nil {
		return
	}
	go a.program.Send(updateNotificationMsg{id: id, spec: spec})
}

// DismissNotification dismisses a specific notification by ID.
// Does nothing if the ID does not exist.
//
// Safe to call from goroutines, including during Init().
func (a *App) DismissNotification(id NotificationID) {
	if a.program == nil {
		return
	}
	go a.program.Send(dismissNotificationMsg{id: id})
}

// ClearAllNotifications dismisses all visible notifications immediately.
//
// Safe to call from goroutines, including during Init().
func (a *App) ClearAllNotifications() {
	if a.program == nil {
		return
	}
	go a.program.Send(clearAllNotificationsMsg{})
}

// handleShowNotification creates a notification and returns a timeout Cmd if needed.
func (a *App) handleShowNotification(spec NotificationSpec) tea.Cmd {
	a.nextNotificationID++
	id := a.nextNotificationID

	notif := &Notification{
		id:        id,
		spec:      spec,
		createdAt: time.Now(),
	}
	a.notifications = append(a.notifications, notif)

	cmds := []tea.Cmd{a.RerenderCmd(true)}

	// Determine timeout: explicit spec.Timeout takes precedence; otherwise
	// Info/Success default to configured timeout, Warning/Error persist.
	timeout := spec.Timeout
	if timeout == 0 {
		if spec.Level == NotificationInfo || spec.Level == NotificationSuccess {
			timeout = a.options.NotificationOptions.DefaultTimeout
			if timeout == 0 {
				timeout = 4 * time.Second
			}
		}
	}

	if timeout > 0 {
		notif.expireAt = time.Now().Add(timeout)
		cmds = append(cmds, tea.Tick(timeout, func(time.Time) tea.Msg {
			return notificationExpireMsg{id: id}
		}))
	}

	return tea.Batch(cmds...)
}

// removeNotification removes a notification by ID.
func (a *App) removeNotification(id NotificationID) {
	for i, n := range a.notifications {
		if n.id == id {
			a.notifications = append(a.notifications[:i], a.notifications[i+1:]...)
			return
		}
	}
}

// handleExpire checks expireAt before removing. When UpdateNotification clears
// or extends the timeout, stale ticks are ignored because expireAt was updated.
func (a *App) handleExpire(id NotificationID) {
	for _, n := range a.notifications {
		if n.id == id {
			if !n.expireAt.IsZero() && time.Now().After(n.expireAt) {
				a.removeNotification(id)
			}
			return
		}
	}
}

// updateNotificationContent updates the spec of an existing notification and
// manages the expiration timeout. For updates, spec.Timeout == 0 means "no
// auto-expire" (unlike initial creation which falls back to DefaultTimeout for
// Info/Success). Returns a tea.Tick if a new timeout should be set.
func (a *App) updateNotificationContent(id NotificationID, spec NotificationSpec) tea.Cmd {
	for _, n := range a.notifications {
		if n.id == id {
			n.spec = spec
			// Update expiration: 0 means no timeout for updates.
			if spec.Timeout > 0 {
				n.expireAt = time.Now().Add(spec.Timeout)
				return tea.Tick(spec.Timeout, func(time.Time) tea.Msg {
					return notificationExpireMsg{id: id}
				})
			}
			n.expireAt = time.Time{} // clear expiration
			return nil
		}
	}
	return nil
}

// notificationAt returns the notification at the given mouse position,
// or nil if no notification is under the mouse.
func (a *App) notificationAt(mouse tea.Mouse) *Notification {
	for i := len(a.notifications) - 1; i >= 0; i-- {
		n := a.notifications[i]
		if n.boundsSet && n.bounds.contains(mouse.X, mouse.Y) {
			return n
		}
	}
	return nil
}

// compositeNotifications overlays all active notifications on top of the base content.
func (a *App) compositeNotifications(baseContent string) string {
	w, h := a.WindowWidth(), a.WindowHeight()
	ss := style.CurrentStyleSet()
	opts := a.options.NotificationOptions

	// Calculate effective max width (0 = min(termWidth/3, 60)).
	maxWidth := opts.MaxWidth
	if maxWidth == 0 {
		maxWidth = min(w/3, 60)
	}
	maxWidth = max(maxWidth, 20) // minimum sanity bound

	// Dynamic height limit: notifications occupy at most half the screen.
	maxTotalHeight := h / 2
	gap := opts.Gap

	layers := []*layout.Layer{layout.NewLayer(baseContent)}
	currentHeight := 0

	// Render from newest (end of slice) to oldest, accumulating height.
	// Stop when we exceed the screen height limit.
	for i := len(a.notifications) - 1; i >= 0; i-- {
		n := a.notifications[i]
		rendered := a.renderNotification(n, ss.Notification, maxWidth, opts.MaxLines)
		notifH := lipgloss.Height(rendered)
		notifW := layout.Width(rendered)

		if currentHeight+notifH > maxTotalHeight {
			break // Oldest notifications are pushed out of view.
		}

		x, y := a.computeNotificationPosition(opts.Anchor, w, h, notifW, notifH, currentHeight, gap)
		n.setBounds(x, y, notifW, notifH)

		layers = append(layers, layout.NewLayer(rendered).X(x).Y(y))
		currentHeight += notifH
	}

	return layout.NewCompositor(layers...).Render()
}

// renderNotification renders a single notification with the given constraints.
func (a *App) renderNotification(n *Notification, styles style.NotificationStyleSet, maxWidth, maxLines int) string {
	spec := n.spec

	// Select frame style and icon based on level.
	var frameStyle lipgloss.Style
	var icon string
	switch spec.Level {
	case NotificationInfo:
		frameStyle = styles.InfoFrame
		icon = styles.InfoIcon
	case NotificationSuccess:
		frameStyle = styles.SuccessFrame
		icon = styles.SuccessIcon
	case NotificationWarning:
		frameStyle = styles.WarningFrame
		icon = styles.WarningIcon
	case NotificationError:
		frameStyle = styles.ErrorFrame
		icon = styles.ErrorIcon
	}

	// Content width = maxWidth - frame overhead (2 border + 2 padding).
	contentWidth := maxWidth - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	var blocks []string

	titleHeight := 0
	titleText := ""

	// Build unified content lines: title (if present) + body lines.
	var allLines []string
	if spec.Title != "" {
		titleHeight = 1
		titleText = icon + spec.Title
		allLines = append(allLines, titleText)
	}
	if spec.Message != "" {
		wrapped := lipgloss.NewStyle().MaxWidth(contentWidth).Render(spec.Message)
		bodyLines := strings.Split(wrapped, "\n")
		// Truncate to maxLines if needed, adding ellipsis on the last visible line.
		if len(bodyLines) > maxLines {
			bodyLines = bodyLines[:maxLines]
			lastLine := bodyLines[maxLines-1]
			lastLine = ansi.Truncate(lastLine, contentWidth-1, "…")
			bodyLines[maxLines-1] = lastLine
		}
		allLines = append(allLines, bodyLines...)
	}

	// Cache geometry for hit-testing.
	n.setContentGeometry(allLines, contentWidth, titleHeight, titleText)

	// Apply selection highlight across all content lines.
	displayLines := allLines
	if n.hasSelection && len(allLines) > 0 {
		displayLines = n.applySelectionHighlight(allLines, contentWidth)
	}

	// Render first line as title (if present), remaining as body.
	remainingLines := displayLines
	if titleHeight > 0 && len(displayLines) > 0 {
		titleLine := displayLines[0]
		// Truncate title text to fit contentWidth-2 (reserve space for close btn)
		titleRunes := []rune(titleLine)
		effectiveWidth := contentWidth - 2
		if effectiveWidth < 1 {
			effectiveWidth = 1
		}
		if len(titleRunes) > effectiveWidth {
			titleRunes = append(titleRunes[:effectiveWidth-1], '…')
		}
		truncatedTitle := string(titleRunes)
		// Render title text left-aligned in contentWidth-2, padded to fill
		titleRendered := styles.Title.Width(contentWidth - 2).Render(truncatedTitle)
		// Close button: muted foreground, same background as notification
		closeBtn := style.CurrentStyleSet().Muted.Copy().Background(styles.Title.GetBackground()).Render(" ✕")
		blocks = append(blocks, titleRendered+closeBtn)
		remainingLines = displayLines[1:]
	}
	if len(remainingLines) > 0 {
		msg := styles.Message.Width(contentWidth).Render(strings.Join(remainingLines, "\n"))
		blocks = append(blocks, msg)
	}

	// Join title and message vertically.
	inner := lipgloss.JoinVertical(lipgloss.Left, blocks...)

	// Apply the frame.
	framed := frameStyle.Render(inner)
	return framed
}

// computeNotificationPosition calculates the (x, y) position for a notification
// based on anchor, terminal dimensions, notification dimensions, and stack offset.
func (a *App) computeNotificationPosition(anchor PopupAnchor, termW, termH, notifW, notifH, stackOffset, gap int) (int, int) {
	const margin = 1

	var x, y int

	// Horizontal positioning.
	switch anchor {
	case AnchorTopLeft, AnchorBottomLeft:
		x = margin
	case AnchorTopRight, AnchorBottomRight:
		x = termW - notifW - margin
	case AnchorTopCenter, AnchorBottomCenter, AnchorCenter:
		x = (termW - notifW) / 2
	default:
		x = (termW - notifW) / 2
	}

	// Vertical positioning with stack offset.
	switch anchor {
	case AnchorTopLeft, AnchorTopCenter, AnchorTopRight:
		// Stack downward from the top.
		y = margin + stackOffset
	case AnchorBottomLeft, AnchorBottomCenter, AnchorBottomRight:
		// Stack upward from the bottom. y is the top-left of the notification.
		y = termH - margin - stackOffset - notifH
	default: // AnchorCenter or custom
		y = (termH / 3) + stackOffset
	}

	// Clamp to screen bounds.
	x = max(0, min(x, termW-notifW))
	y = max(0, min(y, termH-notifH))

	return x, y
}
