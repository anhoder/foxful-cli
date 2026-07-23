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
