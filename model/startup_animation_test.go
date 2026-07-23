package model

import (
	"strings"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/layout"
	"github.com/anhoder/foxful-cli/util"
)

func TestStartupAnimationProgress(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		loaded   time.Duration
		want     float64
	}{
		{name: "zero duration is complete", want: 1},
		{name: "clamps below zero", duration: time.Second, loaded: -time.Second, want: 0},
		{name: "half complete", duration: time.Second, loaded: 500 * time.Millisecond, want: .5},
		{name: "clamps after completion", duration: time.Second, loaded: 2 * time.Second, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := StartupPage{options: &StartupOptions{LoadingDuration: tt.duration}, loadedDuration: tt.loaded}
			if got := s.animationProgress(); got != tt.want {
				t.Fatalf("animationProgress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypewriterRevealMovesThroughOneLetterFromLeftToRight(t *testing.T) {
	logo := util.GetAlphaAscii("AB")
	firstLetterWidth := layout.Width(util.GetAlphaAscii("A"))
	fullWidth := layout.Width(logo)

	quarter := typewriterRevealWidth(logo, "AB", .25)
	if quarter <= 0 || quarter >= firstLetterWidth {
		t.Fatalf("quarter progress should be inside the first letter: got %d, letter width %d", quarter, firstLetterWidth)
	}

	half := typewriterRevealWidth(logo, "AB", .5)
	if half != firstLetterWidth {
		t.Fatalf("half progress should finish only the first letter: got %d, want %d", half, firstLetterWidth)
	}

	threeQuarter := typewriterRevealWidth(logo, "AB", .75)
	if threeQuarter <= firstLetterWidth || threeQuarter >= fullWidth {
		t.Fatalf("three-quarter progress should be inside the second letter: got %d, range (%d, %d)", threeQuarter, firstLetterWidth, fullWidth)
	}
}

func TestTypewriterMaskKeepsLogoWidth(t *testing.T) {
	logo := util.GetAlphaAscii("AB")
	masked := maskStartupLogoColumns(logo, typewriterRevealWidth(logo, "AB", .25))
	if got, want := layout.Width(masked), layout.Width(logo); got != want {
		t.Fatalf("masked logo width = %d, want stable width %d", got, want)
	}
	if strings.Contains(masked, "█") && masked == logo {
		t.Fatalf("early typewriter frame unexpectedly rendered full logo: %q", masked)
	}
}

func TestStartupAnimationDisablesMotionWhenRequested(t *testing.T) {
	s := StartupPage{options: &StartupOptions{ReducedMotion: true}}
	if s.animationEnabled() {
		t.Fatal("ReducedMotion must disable startup animation")
	}
}

func TestRenderStartupLogoFadeStartsBlank(t *testing.T) {
	if got := renderStartupLogo("AB", logoFade, 0, 0); strings.Contains(got, "A") || strings.Contains(got, "B") {
		t.Fatalf("zero-progress fade rendered logo: %q", got)
	}
}

func TestSpecialForegroundIncludesProgress(t *testing.T) {
	s := StartupPage{
		options: &StartupOptions{
			LoadingDuration: time.Second,
			TickDuration:    50 * time.Millisecond,
			Animation:       StartupAnimationMatrixRain,
			Welcome:         "FOX",
		},
		loadedDuration: 500 * time.Millisecond,
	}
	a := &App{windowWidth: 80, windowHeight: 24, options: DefaultOptions()}
	if got := s.specialForeground(a, 1, 1); !strings.Contains(got, "50%") {
		t.Fatalf("special foreground omitted progress: %q", got)
	}
}

func TestIndentLines(t *testing.T) {
	if got, want := indentLines("A\nB", 2), "  A\n  B"; got != want {
		t.Fatalf("indentLines() = %q, want %q", got, want)
	}
}

func TestStartupStatusPercent(t *testing.T) {
	for _, tt := range []struct {
		progress float64
		want     string
	}{
		{0, "0%"},
		{.375, "38%"},
		{1, "100%"},
	} {
		if got := formatStartupPercent(tt.progress); got != tt.want {
			t.Errorf("formatStartupPercent(%v) = %q, want %q", tt.progress, got, tt.want)
		}
	}
}
