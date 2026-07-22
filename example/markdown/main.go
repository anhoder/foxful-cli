package main

import (
	"fmt"

	"charm.land/glamour/v2"
	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
)

var (
	mainMenu = NewMainMenu()
	subMenu  = NewSubMenu()
)

type MainMenu struct {
	model.DefaultMenu
	menus []model.MenuItem
}

func NewMainMenu() *MainMenu {
	m := &MainMenu{}
	m.menus = []model.MenuItem{
		{Title: "Main Menu 1", Subtitle: "Sub Title"},
		{Title: "Main Menu 2", Subtitle: "very long long long long long long long long long long long long subtitle"},
		{Title: "Main Menu 3"},
		{Title: "Main Menu 4"},
		{Title: "Main Menu 5"},
	}
	return m
}

func (m *MainMenu) IsSearchable() bool          { return true }
func (m *MainMenu) GetMenuKey() string           { return "main_menu" }
func (m *MainMenu) MenuViews() []model.MenuItem  { return m.menus }

func (m *MainMenu) SubMenu(_ *model.App, index int) model.Menu {
	if index >= len(m.menus) {
		return nil
	}
	return subMenu
}

type SubMenu struct {
	model.DefaultMenu
	menus []model.MenuItem
}

func NewSubMenu() *SubMenu {
	return &SubMenu{
		menus: []model.MenuItem{
			{Title: "Sub Menu 1"},
			{Title: "Sub Menu 2"},
			{Title: "Sub Menu 3"},
		},
	}
}

func (m *SubMenu) GetMenuKey() string               { return "sub_menu" }
func (m *SubMenu) MenuViews() []model.MenuItem      { return m.menus }
func (m *SubMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }

const markdownContent = `
# Markdown Component Demo

This is a **markdown** rendering demo powered by [Glamour](https://github.com/charmbracelet/glamour).

---

## Text Formatting

You can use **bold**, *italic*, ~~strikethrough~~, and ` + "`" + `inline code` + "`" + `.

---

## Code Block

` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, Foxful!")
}
` + "```" + `

---

## Lists

### Unordered

- Item one
- Item two
  - Nested item
  - Another nested item
- Item three

### Ordered

1. First item
2. Second item
3. Third item

---

## Task List

- [x] Implement MarkdownComponent
- [x] Add Glamour dependency
- [ ] Publish to go-musicfox

---

## Blockquote

> Foxful CLI is a modular TUI component library for building interactive terminal applications.

---

## Table

| Feature  | Status | Priority |
|----------|--------|----------|
| Heading  | done   | high     |
| Code     | done   | high     |
| Table    | done   | medium   |
| Emoji    | todo   | low      |

---

> Tip: Press Esc or click outside the popup to dismiss.
`

// MarkdownController opens a markdown preview popup when the user presses 'm'.
type MarkdownController struct {
	renderer  *glamour.TermRenderer
	lastWidth int
}

func NewMarkdownController() *MarkdownController {
	return &MarkdownController{}
}

func (c *MarkdownController) KeyMsgHandle(msg tea.KeyMsg, a *model.App) (bool, model.Page, tea.Cmd) {
	if msg.String() != "m" {
		return false, nil, nil
	}

	w := a.WindowWidth()
	h := a.WindowHeight()
	popupWidth := w * 70 / 100
	if popupWidth < 40 {
		popupWidth = 40
	}
	if popupWidth > 120 {
		popupWidth = 120
	}

	// Rebuild renderer on width change or first use
	if c.renderer == nil || w != c.lastWidth {
		r, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(popupWidth),
			glamour.WithEmoji(),
			glamour.WithPreservedNewLines(),
		)
		if err != nil {
			return false, nil, nil
		}
		c.renderer = r
		c.lastWidth = w
	}

	rendered, err := c.renderer.Render(markdownContent)
	if err != nil {
		rendered = markdownContent
	}

	maxH := h * 80 / 100
	if maxH < 10 {
		maxH = 10
	}

	popup := model.NewCustomPopup("Markdown Preview", rendered, []model.PopupButton{
		{Text: "OK"},
	}, nil)

	popup.Anchor = model.AnchorCenter
	popup.MaxHeight = maxH
	popup.MaxWidth = popupWidth
	a.ShowPopup(popup)
	return true, nil, a.RerenderCmd(true)
}

func main() {
	ops := model.DefaultOptions()
	ops.MainMenu = mainMenu
	ops.AppName = "Markdown Demo"
	ops.KBControllers = []model.KeyboardController{NewMarkdownController()}

	app := model.NewApp(ops)
	fmt.Println(app.Run())
}
