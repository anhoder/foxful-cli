package main

import (
	"fmt"
	"time"

	"github.com/anhoder/foxful-cli/pkg/model"
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
		{Title: "一级菜单1"},
		{Title: "一级菜单2"},
		{Title: "一级菜单3"},
		{Title: "一级菜单4"},
		{Title: "一级菜单5"},
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
		{Title: "二级菜单1"},
		{Title: "二级菜单2"},
		{Title: "二级菜单3"},
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

func main() {
	var ops = model.DefaultOptions()
	ops.MainMenu = mainMenu
	ops.Components = []model.Component{}

	var app = model.NewApp(ops)

	fmt.Println(app.Run())
}
