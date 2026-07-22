package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/style"
	"github.com/anhoder/foxful-cli/util"
)

type Options struct {
	StartupOptions
	ProgressOptions

	AppName             string
	WhetherDisplayTitle bool
	LoadingText         string
	PrimaryColor        string
	DualColumn          bool // The menu list is displayed as a dual column
	DynamicRowCount     bool // If true, the number of entries per page can be greater than 10
	MaxMenuStartRow     int  // Max number of rows occupied by the title section before the menu. 0 means no limit.
	BottomHeight        int  // Height of the bottom area reserved for components (e.g. spectrum, lyrics, progress bar). Only effective when DynamicRowCount is true. 0 means use the default.
	CenterEverything    bool // If true, everything will be centered. Otherwise, use default layout.
	HideMenu    bool
	DarkTheme   style.Theme // Dark variant for adaptive theme pair. If zero-valued, DefaultTheme is used.
	LightTheme  style.Theme // Light variant for adaptive theme pair. If zero-valued, DefaultTheme is used.

	TeaOptions []tea.ProgramOption // Tea program options

	InitPage        InitPage
	MainMenuTitle   *MenuItem
	Ticker          Ticker          // Ticker for render
	MainMenu        Menu            // Entry menu of app
	LocalSearchMenu LocalSearchMenu // Local search result menu
	Components      []Component     // Custom Extra components
	StatusBar       StatusBar       // Custom status bar, nil = no status bar

	GlobalKeyHandlers map[string]GlobalKeyHandler
	KBControllers     []KeyboardController
	MouseControllers  []MouseController

	InitHook  func(a *App)
	CloseHook func(a *App)

	AltScreen bool
	MouseMode tea.MouseMode
}

type StartupOptions struct {
	EnableStartup     bool
	LoadingDuration   time.Duration
	TickDuration      time.Duration
	ProgressOutBounce bool
	Welcome           string
}

func DefaultOptions() *Options {
	return &Options{
		StartupOptions: StartupOptions{
			EnableStartup:     true,
			LoadingDuration:   time.Second * 2,
			TickDuration:      time.Millisecond * 16,
			ProgressOutBounce: true,
			Welcome:           util.PkgName,
		},
		ProgressOptions: ProgressOptions{
			EmptyCharWhenFirst: '.',
			EmptyChar:          '.',
			EmptyCharWhenLast:  '.',
			FirstEmptyChar:     '.',
			FullCharWhenFirst:  '#',
			FullChar:           '#',
			FullCharWhenLast:   '#',
			LastFullChar:       '#',
		},
		WhetherDisplayTitle: true,
		DualColumn:          true,
		DynamicRowCount:     false,
		MaxMenuStartRow:     0,
		CenterEverything:    false,
		AppName:             util.PkgName,
		LoadingText:         util.LoadingText,
		PrimaryColor:        util.RandomColor,
		MainMenu:            &DefaultMenu{},
		AltScreen:           true,
		MouseMode:           tea.MouseModeAllMotion,
	}
}

type WithOption func(options *Options)

func WithHook(init, close func(a *App)) WithOption {
	return func(opts *Options) {
		opts.InitHook = init
		opts.CloseHook = close
	}
}

func WithMainMenu(mainMenu Menu, mainMenuTitle *MenuItem) WithOption {
	return func(opts *Options) {
		opts.MainMenu = mainMenu
		opts.MainMenuTitle = mainMenuTitle
	}
}

func WithGlobalKeyHandlers(m map[string]GlobalKeyHandler) WithOption {
	return func(options *Options) {
		options.GlobalKeyHandlers = m
	}
}

// WithThemePair sets a dark/light theme pair for adaptive appearance.
// The appropriate theme is selected automatically based on terminal
// background detection. Usage:
//
//	app.With(model.WithThemePair(style.DefaultDarkTheme(), style.DefaultLightTheme()))
func WithThemePair(dark, light style.Theme) WithOption {
	return func(options *Options) {
		options.DarkTheme = dark
		options.LightTheme = light
	}
}
