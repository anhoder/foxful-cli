package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type markerComponent struct{}

func (markerComponent) Update(tea.Msg, *App) {}

func (markerComponent) View(*App, *Main) (string, int) {
	return "component-marker", 1
}

func TestMainViewKeepsComponentRowOnPartialLastPage(t *testing.T) {
	items := make([]MenuItem, 11)
	for i := range items {
		items[i] = MenuItem{Title: "item"}
	}

	options := DefaultOptions()
	options.WhetherDisplayTitle = false
	options.DualColumn = false
	options.MainMenu = &mockMenu{key: "main", items: items}
	options.MainMenuTitle = &MenuItem{Title: "Main"}
	options.Components = []Component{markerComponent{}}

	app := NewApp(options)
	app.windowWidth = 80
	app.windowHeight = 30
	main := NewMain(app, options)
	app.main = main
	_, _ = main.Update(tea.WindowSizeMsg{Width: 80, Height: 30}, app)

	firstPageMarkerRow := rowContaining(t, main.View(app), "component-marker")

	main.menuCurPage = 2
	main.selectedIndex = 10
	lastPageMarkerRow := rowContaining(t, main.View(app), "component-marker")

	if lastPageMarkerRow != firstPageMarkerRow {
		t.Fatalf("component row changed on partial last page: first page row = %d, last page row = %d", firstPageMarkerRow, lastPageMarkerRow)
	}
}

func rowContaining(t *testing.T, view, marker string) int {
	t.Helper()
	for row, line := range strings.Split(ansi.Strip(view), "\n") {
		if strings.Contains(line, marker) {
			return row
		}
	}
	t.Fatalf("view does not contain %q", marker)
	return -1
}
