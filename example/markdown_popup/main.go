package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/style"
)

// MainMenu demonstrates different Markdown popup configurations
type MainMenu struct {
	model.DefaultMenu
}

func NewMainMenu() *MainMenu {
	return &MainMenu{}
}

func (m *MainMenu) MenuViews() []model.MenuItem {
	return []model.MenuItem{
		{Title: "Markdown Popup Examples"},
		{Title: ""},
		{Title: "1 — Default (Close button)"},
		{Title: "2 — Custom buttons (Confirm/Cancel)"},
		{Title: "3 — No buttons"},
		{Title: "4 — Rich content with emoji"},
		{Title: "5 — Help document"},
		{Title: "q — Quit"},
	}
}

func (m *MainMenu) MenuKeyMsgHandle(key string, a *model.App) (bool, model.Page, tea.Cmd) {
	switch key {
	case "1":
		showDefaultPopup(a)
		return true, nil, nil
	case "2":
		showCustomButtonsPopup(a)
		return true, nil, nil
	case "3":
		showNoButtonsPopup(a)
		return true, nil, nil
	case "4":
		showEmojiPopup(a)
		return true, nil, nil
	case "5":
		showHelpPopup(a)
		return true, nil, nil
	case "q", "Q":
		return true, nil, tea.Quit
	}
	return false, nil, nil
}

func (m *MainMenu) Action(a *model.App, index int) (model.Page, tea.Cmd) {
	return nil, nil
}

// showDefaultPopup demonstrates the default Close button behavior
func showDefaultPopup(a *model.App) {
	popup, err := model.NewMarkdownPopup(model.MarkdownPopupSpec{
		Title: "Welcome",
		MarkdownContent: `# Welcome to Foxful CLI

This is a **Markdown** popup with the default *Close* button.

- Easy to use
- Renders Markdown beautifully
- Supports ANSI styling`,
		// Actions is nil, so a default "Close" button is added
		MaxWidth:  60,
		MaxHeight: 15,
		OnResult: func(result model.PopupResult) {
			fmt.Printf("Default popup dismissed: %+v\n", result)
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating popup: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

// showCustomButtonsPopup demonstrates custom action buttons
func showCustomButtonsPopup(a *model.App) {
	popup, err := model.NewMarkdownPopup(model.MarkdownPopupSpec{
		Title: "Confirm Action",
		MarkdownContent: `## Delete Files?

Are you sure you want to delete the following files?

- \`config.yaml\`
- \`data.db\`
- \`cache/\`

**This action cannot be undone!**`,
		Actions: []model.PopupAction{
			{ID: "confirm", Label: "Delete"},
			{ID: "cancel", Label: "Cancel", IsCancel: true},
		},
		MaxWidth:  70,
		MaxHeight: 18,
		OnResult: func(result model.PopupResult) {
			if result.ActionID == "confirm" {
				fmt.Println("User confirmed deletion")
			} else {
				fmt.Println("User cancelled")
			}
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating popup: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

// showNoButtonsPopup demonstrates a popup without any buttons
func showNoButtonsPopup(a *model.App) {
	popup, err := model.NewMarkdownPopup(model.MarkdownPopupSpec{
		Title: "Read-Only Information",
		MarkdownContent: `# System Status

**All systems operational**

This is a read-only information popup with no buttons.
Press **Esc** to dismiss.

---

Status: ✓ Online  
Uptime: 99.9%`,
		Actions:   []model.PopupAction{}, // Empty slice = no buttons
		MaxWidth:  60,
		MaxHeight: 16,
		OnResult: func(result model.PopupResult) {
			fmt.Printf("Read-only popup dismissed by: %v\n", result.Cause)
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating popup: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

// showEmojiPopup demonstrates emoji rendering
func showEmojiPopup(a *model.App) {
	popup, err := model.NewMarkdownPopup(model.MarkdownPopupSpec{
		Title: "Emoji Support",
		MarkdownContent: `# Emoji Markdown :rocket:

Markdown popup supports **emoji codes**! :tada:

## Examples:

- :heart: Favorite
- :star: Star this repo
- :fire: Hot topic
- :bug: Report bug
- :sparkles: New feature

---

Press any button to continue :point_down:`,
		MarkdownEmoji: true, // Enable emoji rendering
		Actions: []model.PopupAction{
			{ID: "ok", Label: "Got it!"},
		},
		MaxWidth:  60,
		MaxHeight: 20,
		OnResult: func(result model.PopupResult) {
			fmt.Println("Emoji popup dismissed")
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating popup: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

// showHelpPopup demonstrates a larger help document
func showHelpPopup(a *model.App) {
	helpContent := `# Keyboard Shortcuts

## Navigation
- **↑/↓** or **j/k** — Move up/down
- **Enter** — Select item
- **Esc** — Go back / Close popup

## Popups
- **Mouse drag** — Move popup by title bar
- **Mouse wheel** — Scroll content
- **Click** — Activate buttons

## Text Selection
- **Click + drag** — Select text in popup
- Selected text is copied to clipboard (OSC 52)

## Other
- **q** — Quit application
- **?** — Show help (this popup)

---

*Tip: You can scroll this popup if the content is long!*`

	popup, err := model.NewMarkdownPopup(model.MarkdownPopupSpec{
		Title:           "Help",
		MarkdownContent: helpContent,
		Actions: []model.PopupAction{
			{ID: "close", Label: "Close", IsCancel: true},
		},
		MaxWidth:  70,
		MaxHeight: 25,
		OnResult: func(result model.PopupResult) {
			fmt.Println("Help popup closed")
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating popup: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

func main() {
	theme := style.DefaultDarkTheme()
	styleSet := style.NewStyleSet(theme)
	style.SetCurrentStyleSet(styleSet)

	app := model.NewApp(
		model.WithInitialPage(NewMainMenu()),
		model.WithEnableMouseAllMotion(true),
	)

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
