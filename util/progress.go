package util

import (
	"strings"

	"github.com/muesli/termenv"
)

type ProgressOptions struct {
	FirstEmptyChar rune // 进度条第一个未加载字符
	EmptyChar      rune // 进度条未加载字符
	LastEmptyChar  rune // 进度条最后一个未加载字符
	FirstFullChar  rune // 进度条第一个已加载字符
	FullChar       rune // 进度条已加载字符
	LastFullChar   rune // 进度条最后一个已加载字符
}

func Progress(options *ProgressOptions, width, fullSize int, progressRamp []string) string {
	var fullCells strings.Builder
	for i := 0; i < fullSize && i < len(progressRamp); i++ {
		if i == 0 {
			fullCells.WriteString(termenv.String(string(options.FirstFullChar)).Foreground(TermProfile.Color(progressRamp[i])).String())
		} else if i >= width-1 {
			fullCells.WriteString(termenv.String(string(options.LastFullChar)).Foreground(TermProfile.Color(progressRamp[i])).String())
		} else {
			fullCells.WriteString(termenv.String(string(options.FullChar)).Foreground(TermProfile.Color(progressRamp[i])).String())
		}
	}

	var (
		emptySize  = width - fullSize
		emptyCells strings.Builder
	)
	if emptySize > 0 {
		if fullSize == 0 {
			emptyCells.WriteRune(options.FirstEmptyChar)
			emptySize--
		}
		emptySize--
		if emptySize > 0 {
			emptyCells.WriteString(strings.Repeat(string(options.EmptyChar), emptySize))
		}
		if fullSize < width {
			emptyCells.WriteRune(options.LastEmptyChar)
		}
	}
	return fullCells.String() + SetFgStyle(emptyCells.String(), termenv.ANSIBrightBlack)
}
