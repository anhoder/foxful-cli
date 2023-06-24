package model

import (
	"time"

	"github.com/anhoder/foxful-cli/util"
	tea "github.com/charmbracelet/bubbletea"
)

type Options struct {
	StartupOptions
	util.ProgressOptions

	AppName             string
	WhetherDisplayTitle bool
	LoadingText         string
	PrimaryColor        string
	DualColumn          bool // The menu list is displayed as a dual column
	HideMenu            bool

	InitPage   InitPage
	MainMenu   Menu                // Entry menu of app
	TeaOptions []tea.ProgramOption // Tea program options
	Ticker     Ticker              // Ticker for render
	Components []Component         // Custom Extra components

	InitHook  func(a *App)
	CloseHook func(a *App)
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
			Welcome:           util.PkgName,
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
		DualColumn:          true,
		AppName:             util.PkgName,
		LoadingText:         util.LoadingText,
		PrimaryColor:        util.RandomColor,
		MainMenu:            &DefaultMenu{},
	}
}
