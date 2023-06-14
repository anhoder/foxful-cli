package model

import (
	"time"

	"github.com/anhoder/foxful-cli/constants"
	"github.com/anhoder/foxful-cli/util"
	tea "github.com/charmbracelet/bubbletea"
)

type Options struct {
	StartupOptions
	util.ProgressOptions

	InitPage            InitPage
	AppName             string
	WhetherDisplayTitle bool
	LoadingText         string
	PrimaryColor        string
	MainMenu            Menu                // Entry menu of app
	DualColumn          bool                // The menu list is displayed as a dual column
	Components          []Component         // Custom Extra components
	ProgramOptions      []tea.ProgramOption // Tea program options

	ScrollTimer Timer // Timer for subtitle scrolling display

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
		DualColumn:          true,
		AppName:             constants.PkgName,
		LoadingText:         constants.LoadingText,
		PrimaryColor:        constants.RandomColor,
		MainMenu:            &DefaultMenu{},
	}
}
