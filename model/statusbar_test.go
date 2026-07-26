package model

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/util"
)

func TestDefaultStatusBarKeepsLongBreadcrumbTitleOnOneLine(t *testing.T) {
	a := &App{windowWidth: 80, windowHeight: 24}
	m := &Main{
		menuTitle: &MenuItem{Title: "Startup Animation Gallery"},
		menuStack: &util.Stack{},
	}

	segments := computeBreadcrumbSegments(m)
	if got, want := segments[0].DisplayTitle, "Startup Animation Gallery"; got != want {
		t.Fatalf("breadcrumb title = %q, want %q", got, want)
	}

	bar := (&DefaultStatusBar{}).View(a, m)
	if got := lipgloss.Height(bar); got != 1 {
		t.Fatalf("status bar height = %d, want 1", got)
	}
}

// TestStatusBarTopDoesNotLeaveBottomGap verifies that when StatusBarPosition=StatusBarTop,
// the total rendered view height equals windowHeight without off-by-one overflow that
// produces an extra blank line at the bottom.
// Regression test for the layout bug where targetHeight did not account for the status bar
// already being included in body, causing lipgloss.Height(targetHeight) to fill to h instead of h-1.
func TestStatusBarTopDoesNotLeaveBottomGap(t *testing.T) {
	windowH := 24
	a := &App{windowWidth: 80, windowHeight: windowH}
	m := NewMain(a, &Options{
		StatusBar:         &DefaultStatusBar{},
		StatusBarPosition: StatusBarTop,
		MainMenu:          &testMenu{items: []MenuItem{{Title: "Item"}}},
		MainMenuTitle:     &MenuItem{Title: "Test Menu"},
	})
	m.menuTitle = &MenuItem{Title: "Test Menu"}

	rendered := m.View(a)
	actualH := lipgloss.Height(rendered)

	if actualH != windowH {
		t.Errorf("rendered view height = %d, want %d (windowHeight) — status bar at top leaves bottom gap",
			actualH, windowH)
	}
}
