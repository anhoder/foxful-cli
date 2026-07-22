package model

import (
	"image/color"
	"strings"

	"github.com/anhoder/foxful-cli/style"
)

type ProgressOptions struct {
	EmptyCharWhenFirst rune
	EmptyChar          rune
	EmptyCharWhenLast  rune
	FirstEmptyChar     rune
	FullCharWhenFirst  rune
	FullChar           rune
	FullCharWhenLast   rune
	LastFullChar       rune
}

func Progress(options *ProgressOptions, width, fullSize int, progressRamp []color.Color) string {
	var fullCells strings.Builder
	for i := 0; i < fullSize && i < len(progressRamp); i++ {
		if i == 0 {
			fullCells.WriteString(style.FG(string(options.FullCharWhenFirst), progressRamp[i]))
		} else if i >= width-1 {
			fullCells.WriteString(style.FG(string(options.FullCharWhenLast), progressRamp[i]))
		} else if i == fullSize-1 {
			fullCells.WriteString(style.FG(string(options.LastFullChar), progressRamp[i]))
		} else {
			fullCells.WriteString(style.FG(string(options.FullChar), progressRamp[i]))
		}
	}

	var (
		emptySize  = width - fullSize
		emptyCells strings.Builder
	)
	if emptySize > 0 {
		if fullSize == 0 {
			emptyCells.WriteRune(options.EmptyCharWhenFirst)
			emptySize--
		}
		emptySize--
		if emptySize > 0 {
			emptyCells.WriteString(string(options.FirstEmptyChar))
			if emptySize > 1 {
				emptyCells.WriteString(strings.Repeat(string(options.EmptyChar), emptySize-1))
			}
		}
		if fullSize < width {
			emptyCells.WriteRune(options.EmptyCharWhenLast)
		}
	}
	return fullCells.String() + style.CurrentStyleSet().ProgressEmpty.Render(emptyCells.String())
}
