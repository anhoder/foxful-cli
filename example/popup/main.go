// Popup example — demonstrates modal dialog usage in foxful-cli.
//
// Press Enter or double-click a menu item to trigger the corresponding popup.
// ESC dismisses the topmost popup.
//
// Mouse support is enabled. When a popup is active, mouse events are
// intercepted to prevent accidental clicks on the background menu.
package main

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/style"
)

var mainMenu = NewMainMenu()

func showAnchoredPopup(a *model.App, anchor model.PopupAnchor, offsetX, offsetY int) {
	var anchorName string
	switch anchor {
	case model.AnchorTopLeft:
		anchorName = "Top-Left"
	case model.AnchorTopCenter:
		anchorName = "Top-Center"
	case model.AnchorTopRight:
		anchorName = "Top-Right"
	case model.AnchorBottomLeft:
		anchorName = "Bottom-Left"
	case model.AnchorBottomCenter:
		anchorName = "Bottom-Center"
	case model.AnchorBottomRight:
		anchorName = "Bottom-Right"
	case model.AnchorCustom:
		anchorName = "Custom"
	default:
		anchorName = "Center"
	}

	msg := fmt.Sprintf("This popup is anchored at %s.\nOffset: (%d, %d)\n\nPress ESC to dismiss.", anchorName, offsetX, offsetY)
	popup := model.NewConfirmPopup(
		fmt.Sprintf("Anchor: %s", anchorName),
		msg,
		func(r model.PopupResult) {
			_ = style.Dim("dismissed") // noop
		},
	)
	popup.Anchor = anchor
	popup.OffsetX = offsetX
	popup.OffsetY = offsetY
	a.ShowPopup(popup)
}

func stackTwoPopups(a *model.App) {
	// Track whether bottom popup has been dismissed
	// First popup: bottom layer
	bottom := model.NewConfirmPopup(
		"Bottom Popup",
		"Bottom popup — press ESC to dismiss me first",
		func(r model.PopupResult) {
			_ = style.Dim("bottom dismissed") // noop
		},
	)
	bottom.Anchor = model.AnchorBottomCenter
	a.ShowPopup(bottom)

	// Second popup: top layer (shown immediately, blocking the first)
	top := model.NewConfirmPopup(
		"Top Popup",
		"Top popup — I block the one below!\n\nPress ESC to dismiss me first, then dismiss the bottom one.",
		func(r model.PopupResult) {
			_ = style.Dim("top dismissed") // noop
		},
	)
	top.Anchor = model.AnchorCenter
	a.ShowPopup(top)
}

func showConfirmPopup(a *model.App) {
	// Ultra simple popup — just a plain text to test rendering
	body := "Press ENTER to confirm, ESC to cancel.\n\nThis is a simple popup test."
	a.ShowPopup(model.NewCustomPopup(
		"Test Popup",
		body,
		[]model.PopupButton{
			{Text: "OK"},
			{Text: "Cancel", IsCancel: true},
		},
		func(r model.PopupResult) {
			_ = style.Dim("dismissed") // noop
		},
	))
}

type MainMenu struct {
	model.DefaultMenu
	menus []model.MenuItem
}

func NewMainMenu() *MainMenu {
	m := &MainMenu{}
	m.menus = []model.MenuItem{
		{Title: "Toggle centered popup"},
		{Title: "Top-left popup"},
		{Title: "Bottom-center popup"},
		{Title: "Top-right popup"},
		{Title: "Custom position popup"},
		{Title: "Stack two popups"},
		{Title: "Dismiss top popup"},
	}
	return m
}

func (m *MainMenu) IsSearchable() bool                     { return true }
func (m *MainMenu) GetMenuKey() string                     { return "main" }
func (m *MainMenu) MenuViews() []model.MenuItem            { return m.menus }
func (m *MainMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }

// Action triggers the corresponding popup when a menu item is activated
// via Enter or double-click, instead of requiring a keyboard controller.
func (m *MainMenu) Action(app *model.App, index int) (model.Page, tea.Cmd) {
	switch index {
	case 0:
		if app.HasPopup() {
			app.DismissPopup()
		} else {
			showConfirmPopup(app)
		}
	case 1:
		showAnchoredPopup(app, model.AnchorTopLeft, 0, 0)
	case 2:
		showAnchoredPopup(app, model.AnchorBottomCenter, 0, 0)
	case 3:
		showAnchoredPopup(app, model.AnchorTopRight, 0, 0)
	case 4:
		showAnchoredPopup(app, model.AnchorCustom, 15, 8)
	case 5:
		stackTwoPopups(app)
	case 6:
		app.DismissPopup()
	}
	return nil, app.RerenderCmd(true)
}

func main() {
	ops := model.DefaultOptions()
	ops.AppName = "Popup Test"
	ops.MainMenu = mainMenu
	ops.Ticker = model.DefaultTicker(300 * time.Millisecond)

	app := model.NewApp(ops)

	// Show popup automatically after 1 second.
	// Rerender(true) is called right after ShowPopup to force an immediate
	// view cycle so popup bounds are set for mouse hit-testing.
	// go func() {
	// 	time.Sleep(1 * time.Second)
	// 	showConfirmPopup(app)
	// 	app.Rerender(true)
	// }()

	fmt.Println(app.Run())
}
