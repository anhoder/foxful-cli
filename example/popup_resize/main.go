package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/style"
)

// ResizeDemoMenu demonstrates the popup resize feature
type ResizeDemoMenu struct {
	model.DefaultMenu
}

func NewResizeDemoMenu() *ResizeDemoMenu {
	return &ResizeDemoMenu{}
}

func (m *ResizeDemoMenu) MenuViews() []model.MenuItem {
	return []model.MenuItem{
		{Title: "Popup Resize Demo (MVP)"},
		{Title: ""},
		{Title: "1 — Simple resizable popup"},
		{Title: "2 — Markdown resizable popup"},
		{Title: "3 — Long content with scrollbar"},
		{Title: "4 — Instructions"},
		{Title: ""},
		{Title: "q — Quit"},
	}
}

func (m *ResizeDemoMenu) MenuKeyMsgHandle(key string, a *model.App) (bool, model.Page, tea.Cmd) {
	switch key {
	case "1":
		showSimpleResizablePopup(a)
		return true, nil, nil
	case "2":
		showMarkdownResizablePopup(a)
		return true, nil, nil
	case "3":
		showScrollableResizablePopup(a)
		return true, nil, nil
	case "4":
		showInstructions(a)
		return true, nil, nil
	case "q", "Q":
		return true, nil, tea.Quit
	}
	return false, nil, nil
}

func (m *ResizeDemoMenu) Action(a *model.App, index int) (model.Page, tea.Cmd) {
	return nil, nil
}

func showSimpleResizablePopup(a *model.App) {
	popup, err := model.NewPopup(model.PopupSpec{
		Title: "Resizable Popup",
		Content: `This is a simple resizable popup.

Look at the bottom-right corner (◢) - you can drag it to resize!

Try it now:
• Move mouse to the bottom-right corner
• Click and drag to resize
• The popup will grow or shrink`,
		Actions: []model.PopupAction{
			{ID: "ok", Label: "OK"},
			{ID: "cancel", Label: "Cancel", IsCancel: true},
		},
		MaxWidth:  60,
		MaxHeight: 16,
		OnResult: func(result model.PopupResult) {
			fmt.Printf("Popup dismissed: %+v\n", result)
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

func showMarkdownResizablePopup(a *model.App) {
	popup, err := model.NewMarkdownPopup(model.MarkdownPopupSpec{
		Title: "Markdown Resize Demo",
		MarkdownContent: `# Resize Me! 

This is a **Markdown** popup with resize support.

## Features
- Drag the **◢** in the bottom-right corner
- Resize to see more or less content
- Works with all Markdown formatting

## Code Example
` + "```go" + `
popup, _ := model.NewPopup(model.PopupSpec{
    Title: "Resizable",
    Content: "Drag the corner!",
})
` + "```" + `

*Try resizing now!*`,
		Actions: []model.PopupAction{
			{ID: "close", Label: "Close", IsCancel: true},
		},
		MaxWidth:  70,
		MaxHeight: 20,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

func showScrollableResizablePopup(a *model.App) {
	longContent := `Long Content Example

This popup contains a lot of text to demonstrate
scrolling combined with resizing.

` + func() string {
		result := ""
		for i := 1; i <= 30; i++ {
			result += fmt.Sprintf("Line %d: This is a line of scrollable content.\n", i)
		}
		return result
	}() + `

You can:
1. Scroll using mouse wheel
2. Drag the scrollbar
3. Resize by dragging the ◢ corner
4. All features work together!`

	popup, err := model.NewPopup(model.PopupSpec{
		Title:     "Scrollable & Resizable",
		Content:   longContent,
		MaxWidth:  60,
		MaxHeight: 20,
		Actions: []model.PopupAction{
			{ID: "ok", Label: "OK"},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

func showInstructions(a *model.App) {
	popup, err := model.NewMarkdownPopup(model.MarkdownPopupSpec{
		Title: "How to Resize",
		MarkdownContent: `# Popup Resize Instructions

## MVP Features (Phase 1)

✅ **Bottom-Right Corner Resize**
- Look for the **◢** indicator in the bottom-right corner
- Your mouse cursor will change to ↘ (nwse-resize) when hovering
- Click and drag to resize the popup

## Mouse Cursor Indicators

| Position | Cursor | Action |
|----------|--------|--------|
| Bottom-right corner (◢) | ↘ | Resize |
| Scrollbar | 👆 Pointer | Scroll |
| Action buttons | 👆 Pointer | Click |
| Content area | 🖰 Text | Select |

## Minimum Size

Popups have a minimum size to ensure:
- Buttons remain visible
- At least one line of content shows
- Title is readable

## Terminal Compatibility

✅ **Full Support**: iTerm2, Ghostty, Alacritty, WezTerm, Kitty
⚠️ **Partial**: Windows Terminal, GNOME Terminal
❌ **Limited**: macOS Terminal, xterm (no cursor indicator, but resize works)

## Future Enhancements (Phase 2+)

🔲 All four corners (◢ ◣ ◤ ◥)
🔲 Edge resizing (sides)
🔲 Keyboard shortcuts (Alt + arrows)
🔲 Throttle optimization for smooth dragging

---

*This is Phase 1 MVP - try it now!*`,
		Actions: []model.PopupAction{
			{ID: "close", Label: "Got it!"},
		},
		MaxWidth:  80,
		MaxHeight: 30,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	a.ShowPopup(popup)
}

func main() {
	theme := style.DefaultDarkTheme()
	styleSet := style.NewStyleSet(theme)
	style.SetCurrentStyleSet(styleSet)

	app := model.NewApp(
		model.WithInitialPage(NewResizeDemoMenu()),
		model.WithEnableMouseAllMotion(true),
	)

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
