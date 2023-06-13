package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anhoder/foxful-cli/pkg/model"
	"github.com/anhoder/foxful-cli/pkg/util"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

var (
	mainMenu      = NewMainMenu()
	secondaryMenu = NewSecondaryMenu()
)

type MainMenu struct {
	model.DefaultMenu
	menus []model.MenuItem
}

func NewMainMenu() *MainMenu {
	m := &MainMenu{}
	m.menus = []model.MenuItem{
		{Title: "Main Menu 1", Subtitle: "Sub Title"},
		{Title: "Main Menu 2", Subtitle: "very long long long long long long long long long long long long subtitle"},
		{Title: "Main Menu 3"},
		{Title: "Main Menu 4"},
		{Title: "Main Menu 5"},
	}

	return m
}

func (m *MainMenu) GetMenuKey() string {
	return "main_menu"
}

func (m *MainMenu) MenuViews() []model.MenuItem {
	return m.menus
}

func (m *MainMenu) SubMenu(_ *model.App, index int) model.Menu {
	if index >= len(m.menus) {
		return nil
	}
	return secondaryMenu
}

type SecondaryMenu struct {
	model.DefaultMenu
	menus []model.MenuItem
}

func NewSecondaryMenu() *SecondaryMenu {
	m := &SecondaryMenu{}
	m.menus = []model.MenuItem{
		{Title: "Sub Menu 1"},
		{Title: "Sub Menu 2"},
		{Title: "Sub Menu 3"},
	}

	return m
}

func (m *SecondaryMenu) GetMenuKey() string {
	return "secondary_menu"
}

func (m *SecondaryMenu) MenuViews() []model.MenuItem {
	return m.menus
}

func (m *SecondaryMenu) SubMenu(_ *model.App, _ int) model.Menu {
	return nil
}

func (m *SecondaryMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) bool {
		time.Sleep(time.Millisecond * 200)
		return true
	}
}

type Component1 struct {
	app       *model.App
	startTime time.Time
	t         time.Time
}

func NewComponent1(app *model.App) *Component1 {
	p := &Component1{
		app: app,
	}
	go func() {
		p.startTime = time.Now()
		for p.t = range time.Tick(time.Millisecond * 200) {
			app.Rerender(false)
		}
	}()

	return p
}

func (p *Component1) Update(msg tea.Msg, a *model.App) {
	return
}

func (p *Component1) View(_ *model.App, main *model.Main, top *int) string {
	var builder strings.Builder
	builder.WriteString(strings.Repeat(" ", main.MenuStartColumn()))
	builder.WriteString(util.SetFgStyle(strconv.Itoa(p.t.Hour())+"h line1\n", termenv.ANSIBrightBlue))
	builder.WriteString(strings.Repeat(" ", main.MenuStartColumn()))
	builder.WriteString(util.SetFgStyle(strconv.Itoa(p.t.Minute())+"m line2\n", termenv.ANSIBrightCyan))
	builder.WriteString(strings.Repeat(" ", main.MenuStartColumn()))
	builder.WriteString(util.SetFgStyle(strconv.Itoa(p.t.Second())+"s line3", termenv.ANSIBrightYellow))

	*top += 3
	return builder.String()
}

func (p *Component1) PassedTime() time.Duration {
	return p.t.Sub(p.startTime)
}

func main() {
	var ops = model.DefaultOptions()
	ops.MainMenu = mainMenu
	var app = model.NewApp(ops)

	app.With(func(options *model.Options) {
		progress := NewComponent1(app)
		ops.Components = []model.Component{
			progress,
		}
		ops.ScrollTimer = progress
	})

	fmt.Println(app.Run())
}
