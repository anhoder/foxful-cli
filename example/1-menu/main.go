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
		{Title: "每日推荐歌曲"},
		{Title: "每日推荐歌单"},
		{Title: "我的歌单"},
		{Title: "私人FM"},
		{Title: "专辑列表"},
		{Title: "搜索"},
		{Title: "排行榜"},
		{Title: "精选歌单"},
		{Title: "热门歌手"},
		{Title: "最近播放歌曲"},
		{Title: "云盘"},
		{Title: "主播电台"},
		{Title: "LastFM"},
		{Title: "帮助"},
		{Title: "检查更新"},
	}

	return m
}

func (m *MainMenu) IsSearchable() bool {
	return true
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
		{Title: "测试1"},
		{Title: "测试2"},
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
	var app = model.NewApp(model.DefaultOptions())
	app.With(func(options *model.Options) {
		options.MainMenu = mainMenu
	})

	fmt.Println(app.Run())
}
