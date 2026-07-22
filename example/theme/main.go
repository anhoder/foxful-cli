package main

import (
	"fmt"
	"image/color"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/style"
)

// ThemeController handles the 't' key by showing a popup with theme info.
// It demonstrates accessing custom styles via style.CurrentStyleSet().Custom.
type ThemeController struct{}

func (c *ThemeController) KeyMsgHandle(msg tea.KeyMsg, a *model.App) (bool, model.Page, tea.Cmd) {
	k := msg.String()
	if k != "t" && k != "T" {
		return false, nil, nil
	}

	styles := style.CurrentStyleSet()

	// Access custom domain styles from the global StyleSet.
	bannerStyle, ok := styles.Custom["bannerColor"]
	if !ok {
		bannerStyle = lipgloss.NewStyle().Foreground(lipgloss.BrightGreen)
	}
	highlightStyle, ok := styles.Custom["highlightColor"]
	if !ok {
		highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.BrightYellow)
	}

	// Use bannerColor to style the popup title.
	popupTitle := bannerStyle.Bold(true).Render("Theme Colors")
	now := time.Now().Format("15:04:05")

	body := lipgloss.JoinVertical(lipgloss.Left,
		popupTitle,
		"",
		"Custom domain colors in action:",
		"",
		bannerStyle.Render("  ▶ Banner color text"),
		highlightStyle.Render("  ▶ Highlight color text"),
		"",
		styles.MenuTitle.Render("  ▶ Primary (menu title)"),
		styles.Subtitle.Render("  ▶ Secondary (subtitle)"),
		"",
		fmt.Sprintf("  Time: %s", now),
	)

	popup := model.NewCustomPopup("", body, []model.PopupButton{
		{Text: "OK"},
	}, nil)
	a.ShowPopup(popup)

	return true, nil, a.RerenderCmd(true)
}

// MainMenu is the main menu with two items.
type MainMenu struct {
	model.DefaultMenu
	items []model.MenuItem
}

func NewMainMenu() *MainMenu {
	return &MainMenu{
		items: []model.MenuItem{
			{Title: "Theme Info", Subtitle: "Press 't' to view theme details"},
			{Title: "Quit", Subtitle: "Press 'q' or Ctrl+C to quit"},
		},
	}
}

func (m *MainMenu) GetMenuKey() string {
	return "main_menu"
}

func (m *MainMenu) MenuViews() []model.MenuItem {
	return m.items
}

func (m *MainMenu) SubMenu(_ *model.App, _ int) model.Menu {
	return nil
}

// Action demonstrates custom actions on menu items without navigating to a submenu.
// When the first item is activated (Enter/double-click), a popup is shown.
// Return (nil, nil) to fall through to normal SubMenu behavior.
func (m *MainMenu) Action(app *model.App, index int) (model.Page, tea.Cmd) {
	if index == 0 {
		// Show a popup as a custom action
		body := style.CurrentStyleSet().MenuTitle.Render("Custom Action Demo")
		body += "\n\nThis popup was triggered by a menu item action\n"
		body += fmt.Sprintf("\nSelected index: %d", index)
		body += "\n\nUse this for popups, logging, API calls, or any\narbitrary logic that shouldn't navigate to a submenu."

		popup := model.NewCustomPopup("Action Fired", body, []model.PopupButton{
			{Text: "Got It"},
		}, nil)
		app.ShowPopup(popup)
		return nil, app.RerenderCmd(true)
	}
	// For other items (index 1 = Quit), fall through to SubMenu
	return nil, nil
}

func main() {
	// ── Step 1: Auto-adaptive default theme ──
	// DefaultTheme() auto-detects terminal background (dark/light).
	// Use DefaultDarkTheme() or DefaultLightTheme() for explicit choice.
	isDark := style.HasDarkBackground()
	mode := "dark"
	if !isDark {
		mode = "light"
	}
	fmt.Printf("Detected terminal background: %s mode\n", mode)

	// Start with the auto-detected default theme, then customize it.
	theme := style.DefaultTheme()

	// ── Step 2: Define custom highlight presets ──
	// User-defined presets override built-in presets with the same name.
	theme.HighlightPresets = map[string]style.Highlight{
		"accent": {Fg: lipgloss.Color("#FF5F87"), Bold: style.BoolPtr(true)},
		"dimmed": {Fg: lipgloss.Color("#6E6E6E")},
	}

	// ── Step 3: Apply presets to Highlight fields ──
	// Use the Preset field to reference a built-in or custom preset.
	// Explicit fields (like Fg here on Subtitle) override preset values.
	theme.StatusBar.Preset = "normal"           // uses built-in "normal" preset
	theme.SelectedItem.Preset = "normal"        // uses built-in "normal" preset
	theme.SelectedItem.Fg = lipgloss.Color("#00D7FF") // override preset with explicit Fg
	theme.BackButton.Preset = "bold"            // uses built-in "bold" preset
	theme.PopupTitle.Preset = "accent"          // uses custom "accent" preset

	// StatusBar gets a dark background via explicit field (preset doesn't set Bg)
	theme.StatusBar.Bg = lipgloss.Color("#1E1E1E")

	// You can still set fields directly without any preset
	theme.MenuTitle.Fg = lipgloss.Color("#FF5F87")
	theme.Subtitle.Fg = lipgloss.Color("#6E6E6E")
	theme.StatusBarTime.Bg = lipgloss.Color("#333333")

	// ── Step 4: Configure hover highlight overrides ──
	// Hover highlights control mouse hover feedback for interactive elements.
	// Each has sensible defaults derived from the element's normal style.
	theme.MenuItemHover.Fg = lipgloss.Color("#FF5F87")       // hovered menu items turn accent
	theme.MenuItemHover.Underline = style.BoolPtr(false)       // disable default underline
	theme.BackButtonHover.Fg = lipgloss.Color("#00D7FF")      // hovered back button color
	theme.SelectedItemHover.Underline = style.BoolPtr(true)    // underline selected item on hover

	// ── Step 5: Add custom domain colors.
	theme.Custom = map[string]color.Color{
		"bannerColor":    lipgloss.Color("#FF5F87"),
		"highlightColor": lipgloss.Color("#00FF00"),
		"appExtra":       lipgloss.Color("#FFA500"),
	}

	// ── Step 6: Build the app with the custom theme ──
	opts := model.DefaultOptions()
	opts.EnableStartup = false
	opts.WhetherDisplayTitle = true
	opts.DualColumn = false
	opts.AppName = "Theme Demo"
	opts.KBControllers = []model.KeyboardController{&ThemeController{}}

	app := model.NewApp(opts)
	// Use the auto-detected theme as both dark and light variants.
	// WithThemePair enables adaptive switching at runtime.
	app.With(
		model.WithThemePair(theme, theme),
		model.WithMainMenu(NewMainMenu(), &model.MenuItem{Title: "Theme Demo"}),
	)

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}