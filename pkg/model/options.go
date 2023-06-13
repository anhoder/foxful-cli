package model

import (
	"time"

	"github.com/anhoder/foxful-cli/pkg/constants"
	"github.com/anhoder/foxful-cli/pkg/util"
	tea "github.com/charmbracelet/bubbletea"
)

type Options struct {
	StartupOptions
	util.ProgressOptions

	InitPage            InitPage
	AppName             string // 应用名
	WhetherDisplayTitle bool   // 是否显示标题
	LoadingText         string // 主页面加载中提示
	PrimaryColor        string // 主题色
	MainMenu            Menu   // 主菜单
	DoubleColumn        bool   // 是否双列显示
	Components          []Component
	ProgramOptions      []tea.ProgramOption

	ScrollTimer Timer
	InitHook    func(a *App)
	CloseHook   func(a *App)
}

type StartupOptions struct {
	LoadingDuration   time.Duration
	TickDuration      time.Duration
	ProgressOutBounce bool
	Welcome           string
}

func (o *StartupOptions) WhetherDisplayStartup() bool {
	return o.LoadingDuration > 0
}

func DefaultOptions() *Options {
	return &Options{
		StartupOptions: StartupOptions{
			LoadingDuration:   time.Second * 2,
			TickDuration:      time.Millisecond * 16,
			ProgressOutBounce: true,
			Welcome:           constants.PkgName,
		},
		ProgressOptions: util.ProgressOptions{
			FirstEmptyChar: '.',
			EmptyChar:      '.',
			LastEmptyChar:  '.',
			FirstFullChar:  '#',
			FullChar:       '#',
			LastFullChar:   '#',
		},
		WhetherDisplayTitle: true,
		DoubleColumn:        true,
		AppName:             constants.PkgName,
		LoadingText:         constants.LoadingText,
		PrimaryColor:        constants.RandomColor,
		MainMenu:            &DefaultMenu{},
	}
}
