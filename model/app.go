package model

import (
	"os"
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
	if len(a.modalStack) == 0 {
		v.SetContent(baseContent)
		return v
	}

	v.SetContent(a.compositeModals(baseContent))
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
	// Execute the rerender command and send its result as a message.
	// Previously we sent the Cmd function itself (which was silently dropped
	// because a func() tea.Msg is not a recognized message type).
	a.program.Send(a.RerenderCmd(cleanScreen)())
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
