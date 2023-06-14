package model

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anhoder/foxful-cli/util"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

type Main struct {
	options *Options

	app *App

	doubleColumn bool // 是否双列显示

	menuTitle            *MenuItem // 菜单标题
	menuTitleStartRow    int       // 菜单标题开始行
	menuTitleStartColumn int       // 菜单标题开始列

	menuStartRow    int // 菜单开始行
	menuStartColumn int // 菜单开始列
	menuBottomRow   int // 菜单最底部所在行

	menuCurPage  int // 菜单当前页
	menuPageSize int // 菜单每页大小

	menuList      []MenuItem  // 菜单列表
	menuStack     *util.Stack // 菜单栈
	selectedIndex int         // 当前选中的菜单index

	inSearching bool            // 搜索菜单
	searchInput textinput.Model // 搜索输入框

	menu       Menu // 当前菜单
	components []Component
}

type tickMainMsg struct{}

func NewMain(app *App, options *Options) (m *Main) {
	m = &Main{
		app:          app,
		options:      options,
		menuTitle:    &MenuItem{Title: options.AppName},
		menu:         options.MainMenu,
		menuStack:    &util.Stack{},
		menuCurPage:  1,
		menuPageSize: 10,
		searchInput:  textinput.New(),
		components:   options.Components,
	}
	m.menuList = m.menu.MenuViews()
	m.searchInput.Placeholder = " 搜索"
	m.searchInput.Prompt = util.GetFocusedPrompt()
	m.searchInput.TextStyle = util.GetPrimaryFontStyle()
	m.searchInput.CharLimit = 32

	return
}

func (main *Main) RefreshMenuList() {
	main.menuList = main.menu.MenuViews()
}

func (main *Main) RefreshMenuTitle() {
	main.menu.FormatMenuItem(main.menuTitle)
}

func (main *Main) IgnoreQuitKeyMsg(_ tea.KeyMsg) bool {
	return main.inSearching
}

func (main *Main) Type() PageType {
	return PtMain
}

func (main *Main) Msg() tea.Msg {
	return tickMainMsg{}
}

func (main *Main) Init(a *App) tea.Cmd {
	return a.Tick(time.Nanosecond)
}

func (main *Main) Update(msg tea.Msg, a *App) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return main.keyMsgHandle(msg, a)
	case tea.MouseMsg:
		return main.mouseMsgHandle(msg, a)
	case tickMainMsg:
		return main, nil
	case tea.WindowSizeMsg:
		main.doubleColumn = msg.Width >= 75 && main.options.DualColumn

		// 菜单开始行、列
		main.menuStartRow = msg.Height / 3
		if !main.options.WhetherDisplayTitle && main.menuStartRow > 1 {
			main.menuStartRow--
		}
		if main.doubleColumn {
			main.menuStartColumn = (msg.Width - 60) / 2
			main.menuBottomRow = main.menuStartRow + int(math.Ceil(float64(main.menuPageSize)/2)) - 1
		} else {
			main.menuStartColumn = (msg.Width - 20) / 2
			main.menuBottomRow = main.menuStartRow + main.menuPageSize - 1
		}

		// 菜单标题开始行、列
		main.menuTitleStartColumn = main.menuStartColumn
		if main.options.WhetherDisplayTitle && main.menuStartRow > 2 {
			if main.menuStartRow > 4 {
				main.menuTitleStartRow = main.menuStartRow - 3
			} else {
				main.menuTitleStartRow = 2
			}
		} else if !main.options.WhetherDisplayTitle && main.menuStartRow > 1 {
			if main.menuStartRow > 3 {
				main.menuTitleStartRow = main.menuStartRow - 3
			} else {
				main.menuTitleStartRow = 2
			}
		}

		// 组件更新
		for _, component := range main.components {
			if component == nil {
				continue
			}
			component.Update(msg, a)
		}
		return main, a.rerenderTrigger(true)
	}

	return main, nil
}

func (main *Main) View(a *App) string {
	var windowHeight, windowWidth = a.WindowHeight(), a.WindowWidth()
	if windowHeight <= 0 || windowWidth <= 0 {
		return ""
	}

	var (
		builder strings.Builder
		top     int // 距离顶部的行数
	)

	// title
	if main.options.WhetherDisplayTitle {
		builder.WriteString(main.titleView(a, &top))
	} else {
		top++
	}

	if !main.options.HideMenu {
		// menu title
		builder.WriteString(main.menuTitleView(a, &top, nil))

		// menu list
		builder.WriteString(main.menuListView(a, &top))

		// search input
		builder.WriteString(main.searchInputView(a, &top))
	} else {
		builder.WriteString("\n\n\n")
		top += 2
	}

	// components view
	for _, component := range main.components {
		if component == nil {
			continue
		}
		builder.WriteString(component.View(a, main, &top))
	}

	if top < windowHeight {
		builder.WriteString(strings.Repeat("\n", windowHeight-top-1))
	}

	return builder.String()
}

func (main *Main) MenuTitleStartColumn() int {
	return main.menuTitleStartColumn
}

func (main *Main) MenuTitleStartRow() int {
	return main.menuTitleStartRow
}

func (main *Main) MenuStartColumn() int {
	return main.menuStartColumn
}

func (main *Main) MenuStartRow() int {
	return main.menuStartRow
}

// title view
func (main *Main) titleView(a *App, top *int) string {
	var (
		titleBuilder strings.Builder
		windowWidth  = a.WindowWidth()
	)
	titleLen := utf8.RuneCountInString(main.options.AppName) + 2
	prefixLen := (windowWidth - titleLen) / 2
	suffixLen := windowWidth - prefixLen - titleLen
	if prefixLen > 0 {
		titleBuilder.WriteString(strings.Repeat("─", prefixLen))
	}
	titleBuilder.WriteString(" ")
	titleBuilder.WriteString(main.options.AppName)
	titleBuilder.WriteString(" ")
	if suffixLen > 0 {
		titleBuilder.WriteString(strings.Repeat("─", suffixLen))
	}

	*top++

	return util.SetFgStyle(titleBuilder.String(), util.GetPrimaryColor())
}

// menu title
func (main *Main) menuTitleView(a *App, top *int, menuTitle *MenuItem) string {
	var (
		menuTitleBuilder strings.Builder
		title            string
		windowWidth      = a.WindowWidth()
		maxLen           = windowWidth - main.menuTitleStartColumn
	)

	if menuTitle == nil {
		menuTitle = main.menuTitle
	}

	realString := menuTitle.OriginString()
	formatString := menuTitle.String()
	if runewidth.StringWidth(realString) > maxLen {
		var menuTmp = *menuTitle
		titleLen := runewidth.StringWidth(menuTmp.Title)
		subTitleLen := runewidth.StringWidth(menuTmp.Subtitle)
		if titleLen >= maxLen-1 {
			menuTmp.Title = runewidth.Truncate(menuTmp.Title, maxLen-1, "")
			menuTmp.Subtitle = ""
		} else if subTitleLen >= maxLen-titleLen-1 {
			menuTmp.Subtitle = runewidth.Truncate(menuTmp.Subtitle, maxLen-titleLen-1, "")
		}
		title = menuTmp.String()
	} else {
		formatLen := runewidth.StringWidth(formatString)
		realLen := runewidth.StringWidth(realString)
		title = runewidth.FillRight(menuTitle.String(), maxLen+formatLen-realLen)
	}

	if top != nil && main.menuTitleStartRow-*top > 0 {
		menuTitleBuilder.WriteString(strings.Repeat("\n", main.menuTitleStartRow-*top))
	}
	if main.menuTitleStartColumn > 0 {
		menuTitleBuilder.WriteString(strings.Repeat(" ", main.menuTitleStartColumn))
	}
	menuTitleBuilder.WriteString(util.SetFgStyle(title, termenv.ANSIBrightGreen))

	if top != nil {
		*top = main.menuTitleStartRow
	}

	return menuTitleBuilder.String()
}

// 菜单列表
func (main *Main) menuListView(a *App, top *int) string {
	var menuListBuilder strings.Builder
	menus := main.getCurPageMenus()
	var lines, maxLines int
	if main.doubleColumn {
		lines = int(math.Ceil(float64(len(menus)) / 2))
		maxLines = int(math.Ceil(float64(main.menuPageSize) / 2))
	} else {
		lines = len(menus)
		maxLines = main.menuPageSize
	}

	if main.menuStartRow > *top {
		menuListBuilder.WriteString(strings.Repeat("\n", main.menuStartRow-*top))
	}

	var str string
	for i := 0; i < lines; i++ {
		str = main.menuLineView(a, i)
		menuListBuilder.WriteString(str)
		menuListBuilder.WriteString("\n")
	}

	// 补全空白
	if maxLines > lines {
		var windowWidth = a.WindowWidth()
		if windowWidth-main.menuStartColumn > 0 {
			menuListBuilder.WriteString(strings.Repeat(" ", windowWidth-main.menuStartColumn))
		}
		menuListBuilder.WriteString(strings.Repeat("\n", maxLines-lines))
	}

	*top = main.menuBottomRow

	return menuListBuilder.String()
}

// 菜单Line
func (main *Main) menuLineView(a *App, line int) string {
	var (
		menuLineBuilder strings.Builder
		index           int
		windowWidth     = a.WindowWidth()
	)
	if main.doubleColumn {
		index = line*2 + (main.menuCurPage-1)*main.menuPageSize
	} else {
		index = line + (main.menuCurPage-1)*main.menuPageSize
	}
	if index > len(main.menuList)-1 {
		index = len(main.menuList) - 1
	}
	if main.menuStartColumn > 4 {
		menuLineBuilder.WriteString(strings.Repeat(" ", main.menuStartColumn-4))
	}
	menuItemStr, menuItemLen := main.menuItemView(a, index)
	menuLineBuilder.WriteString(menuItemStr)
	if main.doubleColumn {
		var secondMenuItemLen int
		if index < len(main.menuList)-1 {
			var secondMenuItemStr string
			secondMenuItemStr, secondMenuItemLen = main.menuItemView(a, index+1)
			menuLineBuilder.WriteString(secondMenuItemStr)
		} else {
			menuLineBuilder.WriteString("    ")
			secondMenuItemLen = 4
		}
		if windowWidth-menuItemLen-secondMenuItemLen-main.menuStartColumn > 0 {
			menuLineBuilder.WriteString(strings.Repeat(" ", windowWidth-menuItemLen-secondMenuItemLen-main.menuStartColumn))
		}
	}

	return menuLineBuilder.String()
}

// 菜单Item
func (main *Main) menuItemView(a *App, index int) (string, int) {
	var (
		menuItemBuilder strings.Builder
		menuTitle       string
		itemMaxLen      int
		menuName        string
		windowWidth     = a.WindowWidth()
	)

	isSelected := !main.inSearching && index == main.selectedIndex

	if isSelected {
		menuTitle = fmt.Sprintf(" => %d. %s", index, main.menuList[index].Title)
	} else {
		menuTitle = fmt.Sprintf("    %d. %s", index, main.menuList[index].Title)
	}
	if len(main.menuList[index].Subtitle) != 0 {
		menuTitle += " "
	}

	if main.doubleColumn {
		if windowWidth <= 88 {
			itemMaxLen = (windowWidth - main.menuStartColumn - 4) / 2
		} else {
			if index%2 == 0 {
				itemMaxLen = 44
			} else {
				itemMaxLen = windowWidth - main.menuStartColumn - 44
			}
		}
	} else {
		itemMaxLen = windowWidth - main.menuStartColumn
	}

	menuTitleLen := runewidth.StringWidth(menuTitle)
	menuSubtitleLen := runewidth.StringWidth(main.menuList[index].Subtitle)

	var tmp string
	if menuTitleLen > itemMaxLen {
		tmp = runewidth.Truncate(menuTitle, itemMaxLen, "")
		tmp = runewidth.FillRight(tmp, itemMaxLen) // fix: 切割中文后缺少字符导致未对齐
		if isSelected {
			menuName = util.SetFgStyle(tmp, util.GetPrimaryColor())
		} else {
			menuName = util.SetNormalStyle(tmp)
		}
	} else if menuTitleLen+menuSubtitleLen > itemMaxLen {
		var r = []rune(main.menuList[index].Subtitle)
		r = append(r, []rune("   ")...)
		var i int
		if main.options.ScrollTimer != nil {
			i = int(main.options.ScrollTimer.PassedTime().Milliseconds()/500) % len(r)
		}
		var s = make([]rune, 0, itemMaxLen-menuTitleLen)
		for j := i; j < i+itemMaxLen-menuTitleLen; j++ {
			s = append(s, r[j%len(r)])
		}
		tmp = runewidth.Truncate(string(s), itemMaxLen-menuTitleLen, "")
		tmp = runewidth.FillRight(tmp, itemMaxLen-menuTitleLen)
		if isSelected {
			menuName = util.SetFgStyle(menuTitle, util.GetPrimaryColor()) + util.SetFgStyle(tmp, termenv.ANSIBrightBlack)
		} else {
			menuName = util.SetNormalStyle(menuTitle) + util.SetFgStyle(tmp, termenv.ANSIBrightBlack)
		}
	} else {
		tmp = runewidth.FillRight(main.menuList[index].Subtitle, itemMaxLen-menuTitleLen)
		if isSelected {
			menuName = util.SetFgStyle(menuTitle, util.GetPrimaryColor()) + util.SetFgStyle(tmp, termenv.ANSIBrightBlack)
		} else {
			menuName = util.SetNormalStyle(menuTitle) + util.SetFgStyle(tmp, termenv.ANSIBrightBlack)
		}
	}

	menuItemBuilder.WriteString(menuName)

	return menuItemBuilder.String(), itemMaxLen
}

// 菜单搜索
func (main *Main) searchInputView(app *App, top *int) string {
	if !main.inSearching {
		*top++
		return "\n"
	}

	var (
		builder     strings.Builder
		windowWidth = app.WindowWidth()
	)
	builder.WriteString("\n")
	*top++

	inputs := []textinput.Model{
		main.searchInput,
	}

	var startColumn int
	if main.menuStartColumn > 2 {
		startColumn = main.menuStartColumn - 2
	}
	for i, input := range inputs {
		if startColumn > 0 {
			builder.WriteString(strings.Repeat(" ", startColumn))
		}

		builder.WriteString(input.View())

		var valueLen int
		if input.Value() == "" {
			valueLen = runewidth.StringWidth(input.Placeholder)
		} else {
			valueLen = runewidth.StringWidth(input.Value())
		}
		if spaceLen := windowWidth - startColumn - valueLen - 3; spaceLen > 0 {
			builder.WriteString(strings.Repeat(" ", spaceLen))
		}

		*top++

		if i < len(inputs)-1 {
			builder.WriteString("\n\n")
			*top++
		}
	}
	return builder.String()
}

// 获取当前页的菜单
func (main *Main) getCurPageMenus() []MenuItem {
	start := (main.menuCurPage - 1) * main.menuPageSize
	end := int(math.Min(float64(len(main.menuList)), float64(main.menuCurPage*main.menuPageSize)))

	return main.menuList[start:end]
}

// key handle
func (main *Main) keyMsgHandle(msg tea.KeyMsg, a *App) (Page, tea.Cmd) {
	if main.inSearching {
		switch msg.String() {
		case "esc":
			main.inSearching = false
			main.searchInput.Blur()
			main.searchInput.Reset()
			return main, a.rerenderTrigger(true)
		case "enter":
			main.searchMenuHandle()
			return main, a.rerenderTrigger(true)
		}

		var cmd tea.Cmd
		main.searchInput, cmd = main.searchInput.Update(msg)

		return main, tea.Batch(cmd)
	}

	key := msg.String()
	switch key {
	case "j", "J", "down":
		main.moveDown()
	case "k", "K", "up":
		main.moveUp()
	case "h", "H", "left":
		main.moveLeft()
	case "l", "L", "right":
		main.moveRight()
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		num, _ := strconv.Atoi(key)
		start := (main.menuCurPage - 1) * main.menuPageSize
		if start+num < len(main.menuList) {
			main.selectedIndex = start + num
		}
	case "g":
		main.moveTop()
	case "G":
		main.moveBottom()
	case "n", "N", "enter":
		main.enterMenu(nil, nil)
	case "b", "B", "esc":
		main.backMenu()
	case "r", "R":
		return main, main.app.rerenderTrigger(true)
	case "/", "／":
		if main.menu.IsSearchable() {
			main.inSearching = true
			main.searchInput.Focus()
		}
	}

	// TODO

	return main, main.app.Tick(time.Nanosecond)
}

// mouse handle
func (main *Main) mouseMsgHandle(msg tea.MouseMsg, a *App) (Page, tea.Cmd) {
	// TODO
	return main, a.Tick(time.Nanosecond)
}

func (main *Main) searchMenuHandle() {
	main.inSearching = false
	main.enterMenu(NewSearchMenu(main.menu, main.searchInput.Value()), &MenuItem{Title: "搜索结果", Subtitle: main.searchInput.Value()})
	main.searchInput.Blur()
	main.searchInput.Reset()
}

type menuStackItem struct {
	menuList      []MenuItem
	selectedIndex int
	menuCurPage   int
	menuTitle     *MenuItem
	menu          Menu
}

// 上移
func (main *Main) moveUp() {
	topHook := main.menu.TopOutHook()
	if main.doubleColumn {
		if main.selectedIndex-2 < 0 && topHook != nil {
			loading := NewLoading(main)
			loading.start()
			if res := topHook(main); !res {
				loading.complete()
				return
			}
			// 更新菜单UI
			main.menuList = main.menu.MenuViews()
			loading.complete()
		}
		if main.selectedIndex-2 < 0 {
			return
		}
		main.selectedIndex -= 2
	} else {
		if main.selectedIndex-1 < 0 && topHook != nil {
			loading := NewLoading(main)
			loading.start()
			if res := topHook(main); !res {
				loading.complete()
				return
			}
			main.menuList = main.menu.MenuViews()
			loading.complete()
		}
		if main.selectedIndex-1 < 0 {
			return
		}
		main.selectedIndex--
	}
	if main.selectedIndex < (main.menuCurPage-1)*main.menuPageSize {
		main.prePage()
	}
}

// 下移
func (main *Main) moveDown() {
	bottomHook := main.menu.BottomOutHook()
	if main.doubleColumn {
		if main.selectedIndex+2 > len(main.menuList)-1 && bottomHook != nil {
			loading := NewLoading(main)
			loading.start()
			if res := bottomHook(main); !res {
				loading.complete()
				return
			}
			main.menuList = main.menu.MenuViews()
			loading.complete()
		}
		if main.selectedIndex+2 > len(main.menuList)-1 {
			return
		}
		main.selectedIndex += 2
	} else {
		if main.selectedIndex+1 > len(main.menuList)-1 && bottomHook != nil {
			loading := NewLoading(main)
			loading.start()
			if res := bottomHook(main); !res {
				loading.complete()
				return
			}
			main.menuList = main.menu.MenuViews()
			loading.complete()
		}
		if main.selectedIndex+1 > len(main.menuList)-1 {
			return
		}
		main.selectedIndex++
	}
	if main.selectedIndex >= main.menuCurPage*main.menuPageSize {
		main.nextPage()
	}
}

// 左移
func (main *Main) moveLeft() {
	if !main.doubleColumn || main.selectedIndex%2 == 0 || main.selectedIndex-1 < 0 {
		return
	}
	main.selectedIndex--
}

// 右移
func (main *Main) moveRight() {
	if !main.doubleColumn || main.selectedIndex%2 != 0 {
		return
	}
	if bottomHook := main.menu.BottomOutHook(); main.selectedIndex >= len(main.menuList)-1 && bottomHook != nil {
		loading := NewLoading(main)
		loading.start()
		if res := bottomHook(main); !res {
			loading.complete()
			return
		}
		main.menuList = main.menu.MenuViews()
		loading.complete()
	}
	if main.selectedIndex >= len(main.menuList)-1 {
		return
	}
	main.selectedIndex++
}

// 上移到顶部
func (main *Main) moveTop() {
	if main.doubleColumn {
		main.selectedIndex = main.selectedIndex % 2
	} else {
		main.selectedIndex = 0
	}
	main.menuCurPage = 1
}

// 下移到底部
func (main *Main) moveBottom() {
	if main.doubleColumn && len(main.menuList)%2 == 0 {
		main.selectedIndex = len(main.menuList) + (main.selectedIndex%2 - 2)
	} else if main.doubleColumn && main.selectedIndex%2 != 0 {
		main.selectedIndex = len(main.menuList) - 2
	} else {
		main.selectedIndex = len(main.menuList) - 1
	}
	main.menuCurPage = int(math.Ceil(float64(len(main.menuList)) / float64(main.menuPageSize)))
	if main.doubleColumn && main.selectedIndex%2 != 0 && len(main.menuList)%main.menuPageSize == 1 {
		main.menuCurPage -= 1
	}
}

// 切换到上一页
func (main *Main) prePage() {
	if prePageHook := main.menu.BeforePrePageHook(); prePageHook != nil {
		loading := NewLoading(main)
		loading.start()
		if res := prePageHook(main); !res {
			loading.complete()
			return
		}
		loading.complete()
	}

	if main.menuCurPage <= 1 {
		return
	}
	main.menuCurPage--
}

// 切换到下一页
func (main *Main) nextPage() {
	if nextPageHook := main.menu.BeforeNextPageHook(); nextPageHook != nil {
		loading := NewLoading(main)
		loading.start()
		if res := nextPageHook(main); !res {
			loading.complete()
			return
		}
		loading.complete()
	}
	if main.menuCurPage >= int(math.Ceil(float64(len(main.menuList))/float64(main.menuPageSize))) {
		return
	}

	main.menuCurPage++
}

// 进入菜单
func (main *Main) enterMenu(newMenu Menu, newTitle *MenuItem) {
	if (newMenu == nil || newTitle == nil) && main.selectedIndex >= len(main.menuList) {
		return
	}

	if newMenu == nil {
		newMenu = main.menu.SubMenu(main.app, main.selectedIndex)
	}
	if newTitle == nil {
		newTitle = &main.menuList[main.selectedIndex]
	}

	stackItem := &menuStackItem{
		menuList:      main.menuList,
		selectedIndex: main.selectedIndex,
		menuCurPage:   main.menuCurPage,
		menuTitle:     main.menuTitle,
		menu:          main.menu,
	}
	main.menuStack.Push(stackItem)

	if newMenu == nil {
		main.menuStack.Pop()
		return
	}

	if enterMenuHook := newMenu.BeforeEnterMenuHook(); enterMenuHook != nil {
		loading := NewLoading(main)
		loading.start()
		if res := enterMenuHook(main); !res {
			loading.complete()
			main.menuStack.Pop() // 压入的重新弹出
			return
		}

		loading.complete()
	}

	if newMenu != nil {
		newMenu.FormatMenuItem(newTitle)
	}

	menuList := newMenu.MenuViews()

	main.menu = newMenu
	main.menuList = menuList
	main.menuTitle = newTitle
	main.selectedIndex = 0
	main.menuCurPage = 1
}

// 菜单返回
func (main *Main) backMenu() {

	if main.menuStack.Len() <= 0 {
		return
	}

	stackItem := main.menuStack.Pop()
	if backMenuHook := main.menu.BeforeBackMenuHook(); backMenuHook != nil {
		loading := NewLoading(main)
		loading.start()
		if res := backMenuHook(main); !res {
			loading.complete()
			main.menuStack.Push(stackItem) // 弹出的重新压入
			return
		}
		loading.complete()
	}
	main.menu.FormatMenuItem(main.menuTitle) // 重新格式化

	stackMenu, ok := stackItem.(*menuStackItem)
	if !ok {
		return
	}

	main.menuList = stackMenu.menuList
	main.menu = stackMenu.menu
	main.menuTitle = stackMenu.menuTitle
	main.menu.FormatMenuItem(main.menuTitle)
	main.selectedIndex = stackMenu.selectedIndex
	main.menuCurPage = stackMenu.menuCurPage
}
