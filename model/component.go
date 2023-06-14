package model

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Component interface {
	Update(msg tea.Msg, a *App)
	View(a *App, main *Main, top *int) string
}
