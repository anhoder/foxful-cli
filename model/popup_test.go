package model

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/style"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestNormalizePopupSurfaceOverridesContentBackground(t *testing.T) {
	surface := color.NRGBA{R: 28, G: 31, B: 38, A: 255}
	content := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")).
		Background(lipgloss.Color("#FF0000")).
		Render("A") + "\x1b[7mB\x1b[0m"

	normalized := normalizePopupSurface(content, surface)
	screen := popupStyledScreen(normalized)
	for _, line := range screen.Lines {
		for _, cell := range line {
			if cell.IsZero() {
				continue
			}
			if !samePopupColor(cell.Style.Bg, surface) {
				t.Fatalf("cell %q background = %#v, want popup surface %#v", cell.Content, cell.Style.Bg, surface)
			}
			if cell.Style.Attrs&uv.AttrReverse != 0 {
				t.Fatalf("cell %q retained reverse video", cell.Content)
			}
		}
	}
}

func TestPopupRenderUsesSurfaceForScrolledContent(t *testing.T) {
	surface := color.NRGBA{R: 24, G: 28, B: 36, A: 255}
	theme := style.DefaultDarkTheme()
	theme.Popup.Surface = surface
	styles := style.NewStyleSet(theme).Popup
	content := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000")).Render("one") + "\n" +
		lipgloss.NewStyle().Background(lipgloss.Color("#00FF00")).Render("two")
	if !samePopupColor(styles.ScrollTrack.GetBackground(), surface) || !samePopupColor(styles.ScrollThumb.GetBackground(), surface) {
		t.Fatal("scrollbar styles must use the popup surface background")
	}

	popup, err := NewPopup(PopupSpec{
		Title:     "Surface",
		Content:   content,
		MaxHeight: 6,
		Actions:   []PopupAction{{ID: "ok", Label: "OK"}},
	})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}

	rendered := popup.render(styles)
	if !popup.isContentScrollable() {
		t.Fatal("expected popup body to scroll")
	}
	screen := popupStyledScreen(rendered.content)
	for _, line := range screen.Lines {
		for _, cell := range line {
			if cell.Content != "o" && cell.Content != "n" && cell.Content != "e" {
				continue
			}
			if !samePopupColor(cell.Style.Bg, surface) {
				t.Fatalf("visible content cell %q background = %#v, want popup surface %#v", cell.Content, cell.Style.Bg, surface)
			}
		}
	}
}

func TestPopupMaxWidthIncludesFrame(t *testing.T) {
	popup, err := NewPopup(PopupSpec{
		Content:  "this content must be constrained by the popup width",
		MaxWidth: 12,
		Actions:  []PopupAction{{ID: "ok", Label: "OK"}},
	})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}

	rendered := popup.render(style.NewStyleSet(style.DefaultDarkTheme()).Popup)
	if got, want := lipgloss.Width(rendered.content), 12; got > want {
		t.Fatalf("popup width = %d, want at most %d including frame", got, want)
	}
}

func TestPopupFrameFillsContentRowAfterStyledContent(t *testing.T) {
	surface := color.NRGBA{R: 30, G: 34, B: 42, A: 255}
	theme := style.DefaultDarkTheme()
	theme.Popup.Surface = surface
	popup, err := NewPopup(PopupSpec{Content: lipgloss.NewStyle().Background(lipgloss.Color("#FF0000")).Render("X")})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}

	rendered := popup.render(style.NewStyleSet(theme).Popup)
	screen := popupStyledScreen(rendered.content)
	for y := 1; y < len(screen.Lines)-1; y++ {
		for x := 1; x < len(screen.Lines[y])-1; x++ {
			cell := screen.CellAt(x, y)
			if !samePopupColor(cell.Style.Bg, surface) {
				t.Fatalf("interior cell (%d, %d) %q background = %#v, want popup surface %#v", x, y, cell.Content, cell.Style.Bg, surface)
			}
		}
	}
}

func TestPopupFrameFillsBorderCellsWithSurface(t *testing.T) {
	surface := color.NRGBA{R: 30, G: 34, B: 42, A: 255}
	theme := style.DefaultDarkTheme()
	theme.Popup.Surface = surface
	popup, err := NewPopup(PopupSpec{Content: "content"})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}

	screen := popupStyledScreen(popup.render(style.NewStyleSet(theme).Popup).content)
	for y, line := range screen.Lines {
		for x := range line {
			if x != 0 && x != len(line)-1 && y != 0 && y != len(screen.Lines)-1 {
				continue
			}
			cell := screen.CellAt(x, y)
			if !samePopupColor(cell.Style.Bg, surface) {
				t.Fatalf("border cell (%d, %d) %q background = %#v, want popup surface %#v", x, y, cell.Content, cell.Style.Bg, surface)
			}
		}
	}
}

func TestPopupInteriorHasNoUnstyledGaps(t *testing.T) {
	surface := color.NRGBA{R: 30, G: 34, B: 42, A: 255}
	theme := style.DefaultDarkTheme()
	theme.Popup.Surface = surface
	popup, err := NewPopup(PopupSpec{
		Title:   "Anchor: Bottom-Center",
		Content: "This popup is anchored at Bottom-Center.\nOffset: (0, 0)\n\nPress ESC to dismiss.",
		Actions: []PopupAction{
			{ID: "confirm", Label: "Confirm"},
			{ID: "cancel", Label: "Cancel", IsCancel: true},
		},
	})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}

	screen := popupStyledScreen(popup.render(style.NewStyleSet(theme).Popup).content)
	for y := range screen.Lines {
		for x := range screen.Lines[y] {
			cell := screen.CellAt(x, y)
			if cell == nil || cell.IsZero() {
				continue
			}
			if cell.Style.Bg == nil {
				t.Fatalf("interior cell (%d, %d) %q has no background; popup surface must fill every cell", x, y, cell.Content)
			}
		}
	}
}

func TestAppCompositePopupsPreservesPopupSurface(t *testing.T) {
	surface := color.NRGBA{R: 30, G: 34, B: 42, A: 255}
	theme := style.DefaultDarkTheme()
	theme.Popup.Surface = surface
	previousStyles := style.CurrentStyleSet()
	style.SetStyleSet(style.NewStyleSet(theme))
	t.Cleanup(func() { style.SetStyleSet(previousStyles) })

	popup, err := NewPopup(PopupSpec{Content: lipgloss.NewStyle().Background(lipgloss.Color("#FF0000")).Render("X")})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}
	app := &App{windowWidth: 80, windowHeight: 24}
	app.ShowPopup(popup)
	base := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000")).Width(80).Height(24).Render("")
	screen := popupStyledScreen(app.compositePopups(base))

	contentFound := false
	for _, line := range screen.Lines {
		for _, cell := range line {
			if cell.Content != "X" {
				continue
			}
			contentFound = true
			if !samePopupColor(cell.Style.Bg, surface) {
				t.Fatalf("composited content background = %#v, want popup surface %#v", cell.Style.Bg, surface)
			}
		}
	}
	if !contentFound {
		t.Fatal("composite output omitted popup content")
	}
	border := screen.CellAt(popup.bounds.x, popup.bounds.y)
	if !samePopupColor(border.Style.Bg, surface) {
		t.Fatalf("composited border background = %#v, want popup surface %#v", border.Style.Bg, surface)
	}
}
func TestPopupActionBoundsMatchRenderedActions(t *testing.T) {
	popup, err := NewPopup(PopupSpec{
		Content: "Choose an action",
		Actions: []PopupAction{
			{ID: "confirm", Label: "Confirm"},
			{ID: "cancel", Label: "Cancel", IsCancel: true},
		},
	})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}

	rendered := popup.render(style.NewStyleSet(style.DefaultDarkTheme()).Popup)
	if got, want := len(rendered.actionBounds), 2; got != want {
		t.Fatalf("action bounds = %d, want %d", got, want)
	}
	popup.setBounds(7, 11, lipgloss.Width(rendered.content), lipgloss.Height(rendered.content), rendered.actionBounds)
	for index, bounds := range popup.actionBounds {
		if got := popup.actionAt(bounds.x, bounds.y); got != index {
			t.Fatalf("actionAt(%d, %d) = %d, want %d", bounds.x, bounds.y, got, index)
		}
	}
}

func TestNewPopupRejectsInvalidActions(t *testing.T) {
	_, err := NewPopup(PopupSpec{Actions: []PopupAction{{ID: "same", Label: "One"}, {ID: "same", Label: "Two"}}})
	if err == nil {
		t.Fatal("NewPopup() accepted duplicate action IDs")
	}

	_, err = NewPopup(PopupSpec{Actions: []PopupAction{{ID: "escape", Label: "Esc", IsCancel: true}, {ID: "outside", Label: "Outside", IsCancel: true}}})
	if err == nil {
		t.Fatal("NewPopup() accepted multiple cancel actions")
	}
}

func TestPopupEscapeSelectsCancelAction(t *testing.T) {
	popup, err := NewPopup(PopupSpec{Actions: []PopupAction{{ID: "cancel", Label: "Cancel", IsCancel: true}}})
	if err != nil {
		t.Fatalf("NewPopup() error = %v", err)
	}

	popup.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	result := popup.consumeResult()
	if result == nil {
		t.Fatal("escape did not dismiss popup")
	}
	if result.ActionID != "cancel" || result.Cause != PopupDismissEscape {
		t.Fatalf("escape result = %+v, want cancel action with escape cause", *result)
	}
}

func samePopupColor(got, want color.Color) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()
	return gr == wr && gg == wg && gb == wb && ga == wa
}
