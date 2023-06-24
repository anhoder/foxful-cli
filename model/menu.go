package model

import (
	"time"

	"github.com/anhoder/foxful-cli/util"
	"github.com/muesli/termenv"
)

type Hook func(main *Main) bool

type MenuItem struct {
	Title    string
	Subtitle string
}

func (item *MenuItem) OriginString() string {
	if item.Subtitle == "" {
		return item.Title
	}
	return item.Title + " " + item.Subtitle
}

func (item *MenuItem) String() string {
	if item.Subtitle == "" {
		return item.Title
	}
	return item.Title + " " + util.SetFgStyle(item.Subtitle, termenv.ANSIBrightBlack)
}

// Menu menu interface
type Menu interface {
	// IsSearchable 当前菜单是否可搜索
	IsSearchable() bool

	// RealDataIndex 索引转换
	RealDataIndex(index int) int

	// GetMenuKey 菜单唯一Key
	GetMenuKey() string

	// MenuViews 获取子菜单View
	MenuViews() []MenuItem

	// FormatMenuItem 进入前的格式化
	FormatMenuItem(item *MenuItem)

	// SubMenu 根据下标获取菜单Model
	SubMenu(app *App, index int) Menu

	// BeforePrePageHook 切换上一页前的Hook
	BeforePrePageHook() Hook

	// BeforeNextPageHook 切换下一页前的Hook
	BeforeNextPageHook() Hook

	// BeforeEnterMenuHook 进入菜单项前的Hook
	BeforeEnterMenuHook() Hook

	// BeforeBackMenuHook 菜单返回前的Hook
	BeforeBackMenuHook() Hook

	// BottomOutHook 触底的Hook
	BottomOutHook() Hook

	// TopOutHook 触顶Hook
	TopOutHook() Hook
}

type DefaultMenu struct {
}

func (e *DefaultMenu) IsSearchable() bool {
	return false
}

func (e *DefaultMenu) RealDataIndex(index int) int {
	return index
}

func (e *DefaultMenu) GetMenuKey() string {
	panic("implement me")
}

func (e *DefaultMenu) MenuViews() []MenuItem {
	return nil
}

func (e *DefaultMenu) FormatMenuItem(_ *MenuItem) {
}

func (e *DefaultMenu) SubMenu(_ *App, _ int) Menu {
	return nil
}

func (e *DefaultMenu) BeforePrePageHook() Hook {
	return nil
}

func (e *DefaultMenu) BeforeNextPageHook() Hook {
	return nil
}

func (e *DefaultMenu) BeforeEnterMenuHook() Hook {
	return nil
}

func (e *DefaultMenu) BeforeBackMenuHook() Hook {
	return nil
}

func (e *DefaultMenu) BottomOutHook() Hook {
	return nil
}

func (e *DefaultMenu) TopOutHook() Hook {
	return nil
}

type Closer interface {
	Close() error
}

type Ticker interface {
	Closer
	Start() error
	Ticker() <-chan time.Time
	PassedTime() time.Duration
}

type defaultTicker struct {
	startTime time.Time
	t         time.Time
	ticker    *time.Ticker
	stop      chan struct{}
	pipeline  chan time.Time
}

func DefaultTicker(duration time.Duration) Ticker {
	return &defaultTicker{
		ticker:   time.NewTicker(duration),
		stop:     make(chan struct{}),
		pipeline: make(chan time.Time),
	}
}

func (d *defaultTicker) Start() error {
	d.startTime = time.Now()
	go func() {
		for {
			select {
			case <-d.stop:
				break
			case d.t = <-d.ticker.C:
				select {
				case d.pipeline <- d.t:
				default:
				}
			}
		}
	}()
	return nil
}

func (d *defaultTicker) Ticker() <-chan time.Time {
	return d.pipeline
}

func (d *defaultTicker) PassedTime() time.Duration {
	return d.t.Sub(d.startTime)
}

func (d *defaultTicker) Close() error {
	close(d.stop)
	d.ticker.Stop()
	return nil
}
