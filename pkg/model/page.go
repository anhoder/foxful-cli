package model

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Page interface {
	IgnoreQuitKeyMsg(msg tea.KeyMsg) bool
	Type() PageType
	Update(msg tea.Msg, a *App) (Page, tea.Cmd)
	View(a *App) string
	Msg() tea.Msg
}

type InitPage interface {
	Page
	Init(a *App) tea.Cmd
}

// PageType 显示模型的类型
type PageType string

const (
	PtStartup PageType = "startup" // 启动页
	PtMain    PageType = "main"    // 主页面
)
