package model

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

type App struct {
	windowWidth  int
	windowHeight int
	options      *Options

	program *tea.Program

	// 当前页面
	page Page
}

// NewApp create application
func NewApp(options *Options) (a *App) {
	a = &App{
		options: options,
		page:    options.InitPage,
	}

	runewidth.DefaultCondition.EastAsianWidth = false

	return
}

type WithOption func(options *Options)

func (a *App) With(w ...WithOption) *App {
	for _, item := range w {
		if item != nil {
			item(a.options)
		}
	}
	return a
}

func (a *App) Init() tea.Cmd {
	if a.options.InitHook != nil {
		a.options.InitHook(a)
	}
	if a.options.Ticker != nil {
		go func() {
			for range a.options.Ticker.Ticker() {
				a.Rerender(false)
			}
		}()
		if err := a.options.Ticker.Start(); err != nil {
			panic("Fail to start ticker: " + err.Error())
		}
	}
	if initPage, ok := a.page.(InitPage); ok {
		return initPage.Init(a)
	}
	return nil
}

func (a *App) Close() {
	if a.options.CloseHook != nil {
		a.options.CloseHook(a)
	}
	if closer, ok := a.page.(Closer); ok {
		_ = closer.Close()
	}
	if a.options.Ticker != nil {
		_ = a.options.Ticker.Close()
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Make sure these keys always quit
	switch msgWithType := msg.(type) {
	case tea.KeyMsg:
		var k = msgWithType.String()
		if k != "q" && k != "Q" && k != "ctrl+c" {
			break
		}
		if a.page != nil && a.page.IgnoreQuitKeyMsg(msgWithType) {
			break
		}
		a.Close()
		return a, tea.Quit
	case tea.WindowSizeMsg:
		a.windowHeight = msgWithType.Height
		a.windowWidth = msgWithType.Width
	}

	page, cmd := a.page.Update(msg, a)
	a.page = page
	return a, cmd
}

func (a *App) View() string {
	if a.WindowHeight() <= 0 || a.WindowWidth() <= 0 {
		return ""
	}

	return a.page.View(a)
}

func (a *App) Run() error {
	if a.page == nil {
		var main = NewMain(a, a.options)
		if a.options.InitPage == nil {
			if a.options.WhetherDisplayStartup() {
				a.options.InitPage = NewStartup(&a.options.StartupOptions, main)
			} else {
				a.options.InitPage = main
			}
		}
		a.page = a.options.InitPage
	}
	a.program = tea.ReplaceWithFoxfulRenderer(tea.NewProgram(a, a.options.TeaOptions...))
	_, err := a.program.Run()
	return err
}

func (a *App) Rerender(cleanScreen bool) {
	if a.program == nil {
		return
	}
	a.program.Send(a.RerenderCmd(cleanScreen))
}

func (a *App) RerenderCmd(cleanScreen bool) tea.Cmd {
	return func() tea.Msg {
		if cleanScreen {
			a.program.Send(tea.ClearScreen())
		}
		return a.page.Msg()
	}
}

func (a *App) Tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return a.page.Msg()
	})
}

func (a *App) WindowWidth() int {
	return a.windowWidth
}

func (a *App) WindowHeight() int {
	return a.windowHeight
}
