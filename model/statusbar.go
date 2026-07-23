package model

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/style"
	"github.com/charmbracelet/x/ansi"
)

// StatusBar is the interface for the bottom status bar.
// Downstream apps implement this to show playback status, progress, etc.
type StatusBar interface {
	View(a *App, m *Main) string
}

// DefaultStatusBar shows a "PATH" nugget, the breadcrumb path on bar background,
// and the current time on the right, in a lipgloss nugget-style status bar.
type DefaultStatusBar struct{}

const maxBreadcrumbSegmentWidth = 32

func (d *DefaultStatusBar) View(a *App, m *Main) string {
	w := a.WindowWidth()
	if w <= 0 {
		return ""
	}

	ss := style.CurrentStyleSet()

	// Left: "PATH" label nugget (Primary bg + white text)
	pathLabel := ss.StatusBarNuggetLabel.Render(" » ")

	// Breadcrumb path as a nugget block (same bg as time nugget, with padding)
	path := buildBreadcrumbPath(m, ss)
	var breadcrumbBlock string
	if path != "" {
		breadcrumbBlock = lipgloss.NewStyle().
			Inherit(ss.StatusBarBreadcrumbBg).
			Padding(0, 1).
			Render(path)
	}

	// Right: time nugget
	now := time.Now().Format("15:04")
	timeNugget := ss.StatusBarTime.Render(" ⏱ " + now + " ")

	// Compose: PATH label + breadcrumb nugget + filler (Surface bg) + time nugget
	labelW := lipgloss.Width(pathLabel)
	bcrumbW := lipgloss.Width(breadcrumbBlock)
	timeW := lipgloss.Width(timeNugget)
	fillerW := w - labelW - bcrumbW - timeW
	if fillerW < 0 {
		fillerW = 0
	}

	filler := lipgloss.NewStyle().
		Inherit(ss.StatusBarText).
		Width(fillerW).
		Render("")

	bar := lipgloss.JoinHorizontal(lipgloss.Top, pathLabel, breadcrumbBlock, filler, timeNugget)
	return ss.StatusBar.Width(w).Render(bar)
}

// breadcrumbSegmentInfo describes a single segment in the breadcrumb display.
type breadcrumbSegmentInfo struct {
	// DisplayTitle is the title after truncation (may include ellipsis suffix).
	DisplayTitle string
	// DisplayWidth is the visual width of the title segment.
	DisplayWidth int
	// DepthIndex is the index into the full breadcrumb path (0 = root).
	DepthIndex int
	// IsLast is true for the current (deepest) menu level.
	IsLast bool
	// IsEllipsis is true when this segment is the "..." truncation placeholder.
	IsEllipsis bool
}

// computeBreadcrumbSegments returns the display-ready breadcrumb segments
// without rendering. Shared between the status bar view and mouse hit-testing.
func computeBreadcrumbSegments(m *Main) []breadcrumbSegmentInfo {
	stackItems := m.menuStack.ToSlice()
	var fullPath []string
	for _, item := range stackItems {
		if stackItem, ok := item.(*menuStackItem); ok {
			fullPath = append(fullPath, stackItem.menuTitle.Title)
		}
	}

	// Avoid duplicating the current title when the stack was just pushed but
	// the deferred tick hasn't updated m.menuTitle yet (enterMenuWithLoading).
	lastIdx := len(fullPath) - 1
	if lastIdx < 0 || fullPath[lastIdx] != m.menuTitle.Title {
		fullPath = append(fullPath, m.menuTitle.Title)
	}

	if len(fullPath) <= 0 {
		return nil
	}

	// Limit to last 3 levels
	var display []string
	if len(fullPath) > 3 {
		display = append(display, "...")
		display = append(display, fullPath[len(fullPath)-3:]...)
	} else {
		display = fullPath
	}

	// Compute segments with truncated titles
	truncated := len(fullPath) > 3
	segments := make([]breadcrumbSegmentInfo, 0, len(display))
	for i, title := range display {
		isLast := i == len(display)-1
		isDots := title == "..."

		var displayTitle string
		if isDots {
			displayTitle = title
		} else {
			displayTitle = title
			if lipgloss.Width(displayTitle) > maxBreadcrumbSegmentWidth {
				displayTitle = ansi.Truncate(displayTitle, maxBreadcrumbSegmentWidth, "…")
			}
		}

		// Map display index back to full-path index
		var depthIdx int
		if isDots {
			depthIdx = 0 // unused — ellipsis is not clickable
		} else if truncated {
			// After "...", segments align to the last N entries of fullPath
			depthIdx = len(fullPath) - (len(display) - i)
		} else {
			depthIdx = i
		}

		segments = append(segments, breadcrumbSegmentInfo{
			DisplayTitle: displayTitle,
			DisplayWidth: lipgloss.Width(displayTitle),
			DepthIndex:   depthIdx,
			IsLast:       isLast,
			IsEllipsis:   isDots,
		})
	}

	return segments
}

// buildBreadcrumbPath builds the styled breadcrumb path string for the status bar.
// Applies hover/click effects based on m.hoveredBreadcrumbIdx.
func buildBreadcrumbPath(m *Main, ss style.StyleSet) string {
	segments := computeBreadcrumbSegments(m)
	if len(segments) == 0 {
		return ""
	}

	separator := lipgloss.NewStyle().
		Inherit(ss.StatusBarBreadcrumbSep).
		Foreground(ss.StatusBarBreadcrumbSep.GetForeground()).
		Render(" / ")
	breadcrumbBase := ss.StatusBarBreadcrumb

	parts := make([]string, 0, len(segments))
	for i, seg := range segments {
		isHovered := !seg.IsLast && !seg.IsEllipsis && i == m.hoveredBreadcrumbIdx

		var styled string
		switch {
		case seg.IsEllipsis:
			styled = breadcrumbBase.Render(seg.DisplayTitle)
		case seg.IsLast:
			styled = breadcrumbBase.Bold(true).Render(seg.DisplayTitle)
		case isHovered:
			// Hovered ancestor: use BreadcrumbHover style (underline + brighter)
			styled = ss.StatusBarBreadcrumbHover.Render(seg.DisplayTitle)
		default:
			styled = breadcrumbBase.Render(seg.DisplayTitle)
		}
		parts = append(parts, styled)
	}

	return strings.Join(parts, separator)
}
