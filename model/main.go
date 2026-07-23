package model

import (
	"fmt"
	"math"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/layout"
	"github.com/anhoder/foxful-cli/style"
	"github.com/anhoder/foxful-cli/util"
	"github.com/mattn/go-runewidth"
)

type Main struct {
	options *Options

	app *App

	isDualColumn bool

	menuTitle *MenuItem

	menuStartRow     int
	menuStartColumn  int
	menuBottomRow    int
	menuListStartRow int // actual row where the first menu item renders

	menuCurPage  int
	menuPageSize int

	menuList      []MenuItem
	menuStack     *util.Stack
	selectedIndex int

	// local search
	inSearching bool
	searchInput textinput.Model

	loadingTips string // transient: set by MenuTips.DisplayTips, cleared by Recover

	// Deferred menu entry: instead of running the BeforeEnterMenuHook
	// synchronously (which blocks the Update cycle and prevents the
	// loadingTips from being rendered), the hook is deferred to the
	// next tick. This allows the current View() cycle to render the
	// loading text before the hook executes.
	pendingEnterMenu *enterMenuDeferred

	menu Menu // current menu

	components []Component

	kbCtrls    []KeyboardController
	mouseCtrls []MouseController

	statusBar StatusBar

	// Mouse click tracking for double-click detection
	lastClickTime time.Time
	lastClickX    int
	lastClickY    int

	// Mouse hover tracking for breadcrumb segments
	hoveredBreadcrumbIdx int // -1 = none, 0+ = display index in breadcrumbSegments

	// Mouse hover tracking for menu list items
	hoveredMenuItemIdx int // -1 = none, 0+ = index in menuList

	// hoveredBackButton tracks whether the mouse is hovering over the back
	// button shown before the menu title when inside a submenu.
	hoveredBackButton bool

	// hoverPointerActive tracks whether the mouse is currently over a clickable
	// element. When true, the terminal mouse pointer is set to "pointer" (hand
	// cursor) via OSC 22. When false, it's reset to "default".
	hoverPointerActive bool
}

type tickMainMsg struct{}

// enterMenuDeferred holds the state for a deferred submenu entry.
// Instead of executing BeforeEnterMenuHook synchronously (which blocks the
// Update cycle), the hook is deferred to the next tick. This gives the
// View() cycle a chance to render loadingTips before the hook runs.
type enterMenuDeferred struct {
	newMenu   Menu
	newTitle  *MenuItem
	loading   *Loading
	stackItem *menuStackItem
}

func NewMain(app *App, options *Options) (m *Main) {
	var mainMenuTitle *MenuItem
	if options.MainMenuTitle != nil {
		mainMenuTitle = options.MainMenuTitle
	} else {
		mainMenuTitle = &MenuItem{Title: options.AppName}
	}

	m = &Main{
		app:                  app,
		options:              options,
		menuTitle:            mainMenuTitle,
		menu:                 options.MainMenu,
		menuStack:            &util.Stack{},
		menuCurPage:          1,
		menuPageSize:         10,
		searchInput:          textinput.New(),
		components:           options.Components,
		kbCtrls:              options.KBControllers,
		mouseCtrls:           options.MouseControllers,
		statusBar:            options.StatusBar,
		hoveredBreadcrumbIdx: -1,
		hoveredMenuItemIdx:   -1,
		hoveredBackButton:    false,
		hoverPointerActive:   false,
	}
	m.menuList = m.menu.MenuViews()
	m.searchInput.Placeholder = " " + SearchPlaceholder
	m.searchInput.Prompt = util.GetFocusedPrompt()
	s := textinput.DefaultStyles(true)
	s.Focused.Text = util.GetPrimaryFontStyle(true)
	m.searchInput.SetStyles(s)
	m.searchInput.CharLimit = 32

	return
}

func (m *Main) RefreshMenuList() {
	m.menuList = m.menu.MenuViews()
}

func (m *Main) RefreshMenuTitle() {
	m.menu.FormatMenuItem(m.menuTitle)
}

func (m *Main) IgnoreQuitKeyMsg(_ tea.KeyMsg) bool {
	return m.inSearching
}

func (m *Main) Type() PageType {
	return PtMain
}

func (m *Main) Msg() tea.Msg {
	return tickMainMsg{}
}

func (m *Main) Init(a *App) tea.Cmd {
	return a.Tick(time.Nanosecond)
}

func (m *Main) computeTitleStartRow() int {
	titleStartRow := 0
	if m.options.WhetherDisplayTitle && m.menuStartRow > 2 {
		if m.menuStartRow > 4 {
			titleStartRow = m.menuStartRow - 3
		} else {
			titleStartRow = 2
		}
	} else if !m.options.WhetherDisplayTitle && m.menuStartRow > 1 {
		if m.menuStartRow > 3 {
			titleStartRow = m.menuStartRow - 3
		} else {
			titleStartRow = 2
		}
	}
	return titleStartRow
}

func (m *Main) Update(msg tea.Msg, a *App) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.keyMsgHandle(msg, a)
	case tea.MouseMsg:
		return m.mouseMsgHandle(msg, a)
	case tickMainMsg:
		if m.pendingEnterMenu != nil {
			p := m.pendingEnterMenu
			m.pendingEnterMenu = nil

			var res bool
			var newPage Page
			if hook := p.newMenu.BeforeEnterMenuHook(); hook != nil {
				if res, newPage = hook(m); !res {
					p.loading.Complete()
					m.menuStack.Pop()
					if newPage != nil {
						return newPage, func() tea.Msg { return newPage.Msg() }
					}
					return m, nil
				}
			}
			p.loading.Complete()

			if p.newMenu != nil {
				p.newMenu.FormatMenuItem(p.newTitle)
			}
			m.hoveredMenuItemIdx = -1
			menuList := p.newMenu.MenuViews()
			m.menu = p.newMenu
			m.menuList = menuList
			m.menuTitle = p.newTitle
			m.selectedIndex = 0
			m.menuCurPage = 1

			if newPage != nil {
				return newPage, func() tea.Msg { return newPage.Msg() }
			}
			return m, a.RerenderCmd(true)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.isDualColumn = msg.Width >= 75 && m.options.DualColumn
		m.menuStartRow = msg.Height / 3
		if m.options.MaxMenuStartRow > 0 {
			if m.menuStartRow > m.options.MaxMenuStartRow {
				m.menuStartRow = m.options.MaxMenuStartRow
			}
		}
		if !m.options.WhetherDisplayTitle && m.menuStartRow > 1 {
			m.menuStartRow--
		}
		if m.isDualColumn {
			switch {
			case msg.Width < 100:
				m.menuStartColumn = msg.Width / 5
			case msg.Width < 150:
				m.menuStartColumn = msg.Width / 4
			default:
				m.menuStartColumn = msg.Width / 3
			}
		} else {
			if msg.Width < 100 {
				m.menuStartColumn = msg.Width / 3
			} else {
				m.menuStartColumn = msg.Width * 2 / 5
			}
		}
		if m.menuStartColumn < 5 {
			m.menuStartColumn = 5
		}

		bottomHeight := 13
		if m.options.BottomHeight > 0 {
			bottomHeight = m.options.BottomHeight
		}
		if m.options.DynamicRowCount {
			maxEntries := (msg.Height - m.menuStartRow - bottomHeight) * m.getNumColumns()
			if maxEntries > 10 {
				m.menuPageSize = maxEntries
			} else {
				m.menuPageSize = 10
			}
		} else {
			m.menuPageSize = 10
		}

		// Compute actual menu list start row and bottom row for hit-testing
		titleStartRow := m.computeTitleStartRow()

		// Leading rows before the menu list:
		// - Title bar (if displayed): 1 row
		// - Filler blank lines: strings.Repeat("\n", titleStartRow-1) → getLines produces titleStartRow parts
		// - Menu title: 1 row
		// - Gap "\n": getLines splits into 2 empty parts → 2 rows
		leadingRows := 0
		if m.options.WhetherDisplayTitle {
			leadingRows++
		}
		if titleStartRow > 1 {
			leadingRows += titleStartRow
		}
		leadingRows += 3 // menu title (1) + gap "\n" → getLines produces 2 parts

		m.menuListStartRow = leadingRows

		menuDisplayLines := m.menuPageSize
		if m.isDualColumn {
			menuDisplayLines = int(math.Ceil(float64(m.menuPageSize) / 2))
		}
		m.menuBottomRow = m.menuListStartRow + menuDisplayLines

		if m.menuCurPage > 0 {
			maxPage := int(math.Ceil(float64(len(m.menuList)) / float64(m.menuPageSize)))
			if m.menuCurPage > maxPage {
				m.menuCurPage = maxPage
			}
		}
		return m, a.RerenderCmd(true)
	}

	return m, nil
}

func (m *Main) View(a *App) string {
	w, h := a.WindowWidth(), a.WindowHeight()
	if w <= 0 || h <= 0 {
		return ""
	}

	var sections []string

	// ── 1. Title bar ──
	if m.options.WhetherDisplayTitle {
		sections = append(sections, m.TitleView(a))
	}

	// ── 2. Menu sections ──
	if !m.options.HideMenu {
		titleStartRow := m.computeTitleStartRow()

		// Inject loading tips into a copy of the menu title
		mt := m.menuTitle
		if m.loadingTips != "" {
			tmp := *mt
			if tmp.Subtitle != "" {
				tmp.Subtitle = tmp.Subtitle + " " + m.loadingTips
			} else {
				tmp.Subtitle = m.loadingTips
			}
			mt = &tmp
		}

		// Vertical gap to menu title row.
		if titleStartRow > 1 {
			sections = append(sections, strings.Repeat("\n", max(0, titleStartRow-1)))
		}
		sections = append(sections, m.menuTitleViewContent(a, mt))

		// Vertical gap: title row → menu start row
		sections = append(sections, "\n")
		sections = append(sections, m.menuListView(a))
		sections = append(sections, m.searchInputView(a))
	} else {
		sections = append(sections, "\n\n\n")
	}

	// ── 3. Components (natural flow) ──
	for _, component := range m.components {
		if component == nil {
			continue
		}
		view, _ := component.View(a, m)
		if view != "" {
			sections = append(sections, view)
		}
	}

	// ── 4. Compose vertically ──
	body := layout.JoinVertical(lipgloss.Left, sections...)

	// ── 5. Status bar at bottom ──
	statusBarView := ""
	statusBarH := 0
	if m.statusBar != nil {
		statusBarView = m.statusBar.View(a, m)
		statusBarH = lipgloss.Height(statusBarView)
	}

	// Height-fill: pad content to fill space before status bar
	if lipgloss.Height(body) < h-statusBarH {
		body = lipgloss.NewStyle().Height(h - statusBarH).Render(body)
	}

	// Combine body + status bar, then wrap with AppBackground.
	// AppBackground is transparent by default (terminal bg shows through).
	ss := style.CurrentStyleSet()
	var content string
	if statusBarView != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, body, statusBarView)
	} else {
		content = body
	}
	return ss.AppBackground.Width(w).Render(content)
}

// MenuTitleStartColumn returns the horizontal column where the menu title starts.
func (m *Main) MenuTitleStartColumn() int {
	return m.menuStartColumn
}

// MenuTitleStartRow returns the row where the menu title starts.
// Computed dynamically in View() since it was removed as a struct field.
// Returns 0 as a sensible default; downstream should use lipgloss layout instead.
func (m *Main) MenuTitleStartRow() int {
	return 0
}

// MenuStartColumn returns the horizontal column where menu items start.
func (m *Main) MenuStartColumn() int {
	return m.menuStartColumn
}

func (m *Main) MenuStartRow() int {
	return m.menuListStartRow
}

func (m *Main) MenuBottomRow() int {
	return m.menuBottomRow
}

func (m *Main) IsDualColumn() bool {
	return m.isDualColumn
}

func (m *Main) CenterEverything() bool {
	return m.options.CenterEverything
}

func (m *Main) MenuTitle() *MenuItem {
	return m.menuTitle
}

func (m *Main) CurMenu() Menu {
	return m.menu
}

func (m *Main) CurPage() int {
	return m.menuCurPage
}

func (m *Main) PageSize() int {
	return m.menuPageSize
}

func (m *Main) SelectedIndex() int {
	return m.selectedIndex
}

func (m *Main) SetSelectedIndex(i int) {
	m.selectedIndex = i
}

// TitleView renders the app name as a decorative bar with dashes on both sides.
func (m *Main) TitleView(a *App) string {
	appName := " " + m.options.AppName + " "
	w := a.WindowWidth()
	titleLen := layout.Width(appName)
	prefixLen := (w - titleLen) / 2
	suffixLen := w - prefixLen - titleLen

	var b strings.Builder
	if prefixLen > 0 {
		b.WriteString(strings.Repeat("─", prefixLen))
	}
	b.WriteString(appName)
	if suffixLen > 0 {
		b.WriteString(strings.Repeat("─", suffixLen))
	}
	return style.CurrentStyleSet().Title.Render(b.String())
}

// backButtonIcon returns the styled back button icon suitable for prepending
// to the menu title when inside a submenu.
func (m *Main) backButtonIcon() string {
	ss := style.CurrentStyleSet()
	if m.hoveredBackButton {
		return ss.BackButtonHover.Render("←")
	}
	return ss.BackButton.Render("←")
}

// menuTitleViewContent renders the menu title content string, left-aligned
// at menuStartColumn to match the menu items' horizontal position.
// The loading tips should be injected by the caller (see View).
func (m *Main) menuTitleViewContent(a *App, menuTitle *MenuItem) string {
	if menuTitle == nil {
		menuTitle = m.menuTitle
	}
	windowWidth := a.WindowWidth()
	startCol := m.menuStartColumn
	ss := style.CurrentStyleSet()

	// When in a submenu, show a back button to the left of the title.
	// The back button is positioned at startCol - backButtonWidth so the
	// title itself remains at its original startCol position.
	showBack := m.menuStack.Len() > 0

	maxLen := windowWidth - startCol
	realString := menuTitle.OriginString()
	formatString := menuTitle.String()

	var titleText string
	if lipgloss.Width(realString) > maxLen {
		// Truncate long titles: prioritize title, clip subtitle if needed
		tmp := *menuTitle
		titleLen := lipgloss.Width(tmp.Title)
		subTitleLen := lipgloss.Width(tmp.Subtitle)
		if titleLen >= maxLen-1 {
			tmp.Title = lipgloss.NewStyle().Width(maxLen - 1).MaxWidth(maxLen - 1).Render(tmp.Title)
			tmp.Subtitle = ""
		} else if subTitleLen >= maxLen-titleLen-1 {
			tmp.Subtitle = lipgloss.NewStyle().Width(maxLen - titleLen - 1).MaxWidth(maxLen - titleLen - 1).Render(tmp.Subtitle)
		}
		titleText = tmp.String()
	} else {
		titleText = lipgloss.NewStyle().Width(maxLen).Render(formatString)
	}

	// Style the title independently — back button must NOT affect its color.
	styledTitle := ss.MenuTitle.Render(titleText)

	if showBack {
		// Back button at startCol - backButtonWidth, title unchanged at startCol.
		// Layout: [padding]←[space][title...]
		backIcon := m.backButtonIcon()
		padding := startCol - backButtonWidth
		if padding < 0 {
			padding = 0
		}
		return strings.Repeat(" ", padding) + backIcon + " " + styledTitle
	}

	// No back button: original padding + title
	if startCol > 0 {
		styledTitle = lipgloss.NewStyle().PaddingLeft(startCol).Render(styledTitle)
	}
	return styledTitle
}

// MenuTitleView menu title
func (m *Main) MenuTitleView(a *App) string {
	return m.menuTitleViewContent(a, m.menuTitle)
}

func (m *Main) MenuList() []MenuItem {
	return m.menuList
}

func (m *Main) getNumColumns() int {
	if m.isDualColumn {
		return 2
	}
	return 1
}

func (m *Main) forceEntryLength(item *MenuItem, targetLength int) string {
	// Case 1:
	// Only enough space for the main title. Not enough width for subtitle.
	titleWidth := layout.Width(item.Title)
	minSubtitleWidth := 5
	if titleWidth >= targetLength-minSubtitleWidth {
		return lipgloss.NewStyle().
			Width(targetLength).
			Render(item.Title)
	}
	// Case 2:
	// Enough space for everything.
	fullWidth := layout.Width(item.OriginString())
	if fullWidth <= targetLength {
		return lipgloss.NewStyle().Width(targetLength).Render(item.OriginString())
	}
	// Case 3:
	// Enough space for main title. Need to scroll subtitle.
	subtitleSpace := targetLength - titleWidth - 1
	// Need 2 extra spaces for visual separation between end of subtitle and beginning.
	r := []rune(item.Subtitle + "  ")
	s := make([]rune, 0, subtitleSpace)
	indexStart := 0
	if m.options.Ticker != nil {
		indexStart = int(m.options.Ticker.PassedTime().Milliseconds() / 500 % int64(len(r)))
	}
	currentWidth := 0
	for i := indexStart; currentWidth < subtitleSpace; i = (i + 1) % len(r) {
		rw := layout.Width(string(r[i]))
		if currentWidth+rw > subtitleSpace {
			break
		}
		s = append(s, r[i])
		currentWidth += rw
	}
	subtitle := lipgloss.NewStyle().Width(subtitleSpace).MaxWidth(subtitleSpace).Render(string(s))
	return item.Title + " " + style.CurrentStyleSet().Subtitle.Render(subtitle)
}

func (m *Main) formatEntry(item *MenuItem, index int, targetLength int) string {
	if item == nil {
		return lipgloss.NewStyle().Width(targetLength).Render("")
	}
	var fmtStart string
	if !m.inSearching && index == m.selectedIndex {
		fmtStart = " => "
	} else {
		fmtStart = "    "
	}
	titleLength := targetLength - m.getMaxIndexWidth() - 6
	songEntry := fmt.Sprintf(
		fmt.Sprintf("%s%%%dd. %%s", fmtStart, m.getMaxIndexWidth()),
		index,
		m.forceEntryLength(item, titleLength))
	if m.isSelected(index) {
		return style.CurrentStyleSet().SelectedItem.Render(songEntry)
	}
	return songEntry
}

func (m *Main) centeredMenuView(a *App, lines int) string {
	var allSongs []*MenuItem
	startIndex := m.getPageStartIndex()
	endIndex := startIndex + lines
	if m.isDualColumn {
		endIndex = startIndex + lines*2
	}
	var titleLengths []int
	for i := startIndex; i < endIndex; i++ {
		if i < len(m.menuList) {
			menuItem := m.menuList[i]
			length := layout.Width(menuItem.OriginString())
			titleLengths = append(titleLengths, length)
			allSongs = append(allSongs, &menuItem)
		} else {
			allSongs = append(allSongs, nil)
		}
	}
	allSongs = append(allSongs, nil)

	slices.Sort(titleLengths)
	maxSongTitleLength := 0
	if len(titleLengths) > 0 {
		maxSongTitleLength = titleLengths[len(titleLengths)-1]
	}
	if len(titleLengths) >= 6 && maxSongTitleLength >= 30 {
		// Drop the longest 30% of all titles to prevent the menu from being stretched too long due to outliers
		maxSongTitleLength = titleLengths[int32(0.7*float32(len(titleLengths)))]
		if maxSongTitleLength < 30 {
			maxSongTitleLength = 30
		}
	}

	// Songs have 4 spaces built-in at the front, so we need 4 columns on the right side to balance spaces
	remainingWindowWidth := a.windowWidth - 4

	// Extra padding applied to every segment.
	// If the window is wide, we want more padding.
	extraPadding := (a.windowWidth - 40) / 5
	if extraPadding < 0 {
		extraPadding = 0
	}
	remainingWindowWidth -= extraPadding

	itemMaxLength := remainingWindowWidth / m.getNumColumns()

	entryLength := maxSongTitleLength + 6 + m.getMaxIndexWidth()
	if entryLength > itemMaxLength {
		entryLength = itemMaxLength
	}

	var rows []string
	for i := 0; i < lines; i++ {
		index := i * m.getNumColumns()
		menuIndex := m.getPageStartIndex() + index
		left := m.formatEntry(allSongs[index], menuIndex, entryLength)
		if m.isDualColumn {
			right := m.formatEntry(allSongs[index+1], menuIndex+1, entryLength)
			row := layout.JoinHorizontal(lipgloss.Center, left, right)
			rows = append(rows, lipgloss.NewStyle().Width(a.windowWidth).Align(lipgloss.Center).Render(row))
		} else {
			rows = append(rows, lipgloss.NewStyle().Width(a.windowWidth).Align(lipgloss.Center).Render(left))
		}
	}
	return layout.JoinVertical(lipgloss.Left, rows...)
}

func (m *Main) menuListView(a *App) string {
	var menuListBuilder strings.Builder
	if m.options.DynamicRowCount {
		m.menuCurPage = m.selectedIndex/m.menuPageSize + 1
	}
	menus := m.getCurPageMenus()
	var lines, maxLines int
	if m.isDualColumn {
		lines = int(math.Ceil(float64(len(menus)) / 2))
		maxLines = int(math.Ceil(float64(m.menuPageSize) / 2))
	} else {
		lines = len(menus)
		maxLines = m.menuPageSize
	}

	if m.options.CenterEverything {
		menuListBuilder.WriteString(m.centeredMenuView(a, lines))
	} else {
		var menuLines []string
		for i := 0; i < lines; i++ {
			menuLines = append(menuLines, m.menuLineView(a, i))
		}
		menuListBuilder.WriteString(lipgloss.JoinVertical(lipgloss.Left, menuLines...))
	}

	// fill blanks to maintain fixed page size
	if maxLines > lines {
		var fillLines []string
		blankLine := lipgloss.NewStyle().Width(a.WindowWidth() - m.menuStartColumn).Render("")
		for i := lines; i < maxLines; i++ {
			fillLines = append(fillLines, blankLine)
		}
		menuListBuilder.WriteString(lipgloss.JoinVertical(lipgloss.Left, fillLines...))
	}

	return menuListBuilder.String()
}

// truncateVisualWidth truncates s to fit within maxWidth visual cells,
// handling CJK characters (width 2) and other wide runes correctly.
// Returns the original string if it already fits.
func truncateVisualWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(s)
	var width int
	for i, r := range runes {
		rw := runewidth.RuneWidth(r)
		if width+rw > maxWidth {
			return string(runes[:i])
		}
		width += rw
	}
	return s
}

func (m *Main) menuItemView(a *App, index int) (string, int) {
	var (
		menuItemBuilder strings.Builder
		menuTitle       string
		itemMaxLen      int
		menuName        string
		windowWidth     = a.WindowWidth()
		maxIndexWidth   = m.getMaxIndexWidth()
	)

	isSelected := m.isSelected(index)
	isHovered := !m.inSearching && index == m.hoveredMenuItemIdx

	// Resolve title style based on selection + hover state
	ss := style.CurrentStyleSet()
	titleStyle := ss.MenuItem
	switch {
	case isHovered && isSelected:
		titleStyle = ss.SelectedItemHover
	case isHovered:
		titleStyle = ss.MenuItemHover
	case isSelected:
		titleStyle = ss.SelectedItem
	}

	if isSelected {
		menuTitle = fmt.Sprintf(fmt.Sprintf(" => %%%dd. %%s", maxIndexWidth), index, m.menuList[index].Title)
	} else {
		menuTitle = fmt.Sprintf(fmt.Sprintf("    %%%dd. %%s", maxIndexWidth), index, m.menuList[index].Title)
	}
	// if len(m.menuList[index].Subtitle) != 0 {
	menuTitle += " "
	// }

	if m.isDualColumn {
		if windowWidth <= 88 {
			itemMaxLen = (windowWidth - m.menuStartColumn - 4) / 2
		} else {
			if index%2 == 0 {
				itemMaxLen = 44
			} else {
				itemMaxLen = windowWidth - m.menuStartColumn - 44
			}
		}
	} else {
		itemMaxLen = windowWidth - m.menuStartColumn
	}

	menuTitleLen := lipgloss.Width(menuTitle)
	menuSubtitleLen := lipgloss.Width(m.menuList[index].Subtitle)

	var tmp string
	if menuTitleLen > itemMaxLen {
		// Title too long — manually truncate to visual width to avoid
		// line wrapping that would break the fixed-row UI layout.
		truncated := truncateVisualWidth(menuTitle, itemMaxLen)
		menuName = titleStyle.Render(truncated)
	} else if menuTitleLen+menuSubtitleLen > itemMaxLen {
		r := []rune(m.menuList[index].Subtitle + "   ")
		s := make([]rune, 0, itemMaxLen-menuTitleLen)
		indexStart := 0
		if m.options.Ticker != nil {
			indexStart = int(m.options.Ticker.PassedTime().Milliseconds() / 500 % int64(len(r)))
		}
		currentWidth := 0
		for i := indexStart; currentWidth < itemMaxLen-menuTitleLen; i = (i + 1) % len(r) {
			rw := lipgloss.Width(string(r[i]))
			if currentWidth+rw > itemMaxLen-menuTitleLen {
				break
			}
			s = append(s, r[i])
			currentWidth += rw
		}
		tmp = lipgloss.NewStyle().Width(itemMaxLen - menuTitleLen).MaxWidth(itemMaxLen - menuTitleLen).Render(string(s))
		menuName = titleStyle.Render(menuTitle) + ss.Subtitle.Render(tmp)
	} else {
		tmp = lipgloss.NewStyle().
			Width(itemMaxLen - menuTitleLen).
			Render(m.menuList[index].Subtitle)
		menuName = titleStyle.Render(menuTitle) + ss.Subtitle.Render(tmp)
	}

	menuItemBuilder.WriteString(menuName)

	return menuItemBuilder.String(), itemMaxLen
}

func (m *Main) menuLineView(a *App, line int) string {
	var index int
	if m.isDualColumn {
		index = line*2 + m.getPageStartIndex()
	} else {
		index = line + m.getPageStartIndex()
	}
	if index >= len(m.menuList) {
		return "" // beyond menu bounds — empty row
	}

	menuItemStr, _ := m.menuItemView(a, index)

	var row string
	if m.isDualColumn {
		var secondMenuItemStr string
		if index+1 < len(m.menuList) {
			secondMenuItemStr, _ = m.menuItemView(a, index+1)
		} else {
			secondMenuItemStr = "" // last item has no second column
		}
		// Fixed 4-space gap between columns
		row = menuItemStr + "    " + secondMenuItemStr
	} else {
		row = menuItemStr
	}
	// Left-align row at menuStartColumn (offset by -4 to account for " => "/"    " prefix)
	if m.menuStartColumn > 4 {
		row = lipgloss.NewStyle().PaddingLeft(m.menuStartColumn - 4).Render(row)
	}
	return row
}

func (m *Main) getPageStartIndex() int {
	return (m.menuCurPage - 1) * m.menuPageSize
}

func (m *Main) getMaxIndexWidth() int {
	return int(math.Log10(float64((m.menuPageSize*m.menuCurPage)-1))) + 1
}

// backButtonWidth is the display width of the back button icon including padding.
const backButtonWidth = 2 // "←"

// menuTitleY returns the 0-indexed Y position of the menu title row in the
// rendered output. Used for mouse hit-testing the back button.
func (m *Main) menuTitleY() int {
	y := m.computeTitleStartRow()
	if m.options.WhetherDisplayTitle {
		y++
	}
	return y
}

// isOverBackButton checks if the given screen position falls within the back
// button area shown before the menu title when inside a submenu.
func (m *Main) isOverBackButton(x, y int, _ *App) bool {
	if m.menuStack.Len() <= 0 {
		return false
	}
	if y != m.menuTitleY() {
		return false
	}
	// The back icon "←" is rendered at column (menuStartColumn - backButtonWidth).
	iconCol := m.menuStartColumn - backButtonWidth
	return iconCol >= 0 && x >= iconCol && x < iconCol+backButtonWidth
}

// mouseInMenuArea checks if the given Y coordinate falls within the menu list bounds.
func (m *Main) mouseInMenuArea(y int) bool {
	return y >= m.menuListStartRow && y < m.menuBottomRow
}

// menuItemAt maps a screen (x, y) coordinate to a menuList index.
// Returns -1 if the coordinate is outside any menu item.
func (m *Main) menuItemAt(x, y int) int {
	row := y - m.menuListStartRow
	numCols := m.getNumColumns()

	maxLines := m.menuPageSize
	if m.isDualColumn {
		maxLines = int(math.Ceil(float64(m.menuPageSize) / 2))
	}
	if row < 0 || row >= maxLines {
		return -1
	}

	col := 0
	if m.isDualColumn {
		if m.options.CenterEverything {
			// In centered mode, columns are centered — split at midpoint
			if x > m.app.WindowWidth()/2 {
				col = 1
			}
		} else {
			// In left-aligned mode, columns start at menuStartColumn-4.
			// The left column occupies up to leftItemWidth, then 4-space gap,
			// then the right column.
			leftItemWidth := 44
			if m.app.WindowWidth() <= 88 {
				leftItemWidth = (m.app.WindowWidth() - m.menuStartColumn - 4) / 2
			}
			splitX := m.menuStartColumn - 4 + leftItemWidth + 2
			if x >= splitX {
				col = 1
			}
		}
	}

	idx := m.getPageStartIndex() + row*numCols + col
	if idx < 0 || idx >= len(m.menuList) {
		return -1
	}
	return idx
}

func (m *Main) isSelected(index int) bool {
	return !m.inSearching && index == m.selectedIndex
}

func (m *Main) searchInputView(app *App) string {
	var (
		windowWidth = app.WindowWidth()
		ss          = style.CurrentStyleSet()
	)

	if !m.inSearching {
		// Help hint bar: shows per-menu keyboard shortcuts when search is inactive.
		// Each Menu can override HelpHints() to customize the displayed shortcuts.
		hints := m.menu.HelpHints()
		if len(hints) == 0 {
			return "" // menu opted out of help bar
		}
		var parts []string
		for _, h := range hints {
			parts = append(parts,
				ss.HintKey.Render("  "+h.Key)+ss.Muted.Render(" "+h.Desc),
			)
		}
		hint := layout.JoinHorizontal(layout.Top, parts...)
		return lipgloss.NewStyle().
			Width(windowWidth).
			Align(lipgloss.Center).
			PaddingTop(1).
			Render(hint)
	}

	// Search input: left-aligned with menu, same row as the help bar.
	inputView := m.searchInput.View()
	inputView = lipgloss.NewStyle().
		Width(windowWidth).
		PaddingLeft(m.menuStartColumn).
		PaddingTop(1).
		Render(inputView)

	return inputView
}

func (m *Main) getCurPageMenus() []MenuItem {
	start := m.getPageStartIndex()
	end := int(math.Min(float64(len(m.menuList)), float64(m.menuCurPage*m.menuPageSize)))

	return m.menuList[start:end]
}

// key handle
func (m *Main) keyMsgHandle(msg tea.KeyMsg, a *App) (Page, tea.Cmd) {
	if m.inSearching {
		switch msg.String() {
		case "esc":
			m.inSearching = false
			m.searchInput.Blur()
			m.searchInput.Reset()
			return m, a.RerenderCmd(true)
		case "enter":
			m.searchMenuHandle()
			return m, a.RerenderCmd(true)
		}
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, tea.Batch(cmd)
	}

	var (
		key             = msg.String()
		newPage         Page
		lastCmd         tea.Cmd
		stopPropagation bool
	)
	for _, c := range m.kbCtrls {
		stopPropagation, newPage, lastCmd = c.KeyMsgHandle(msg, a)
		if stopPropagation {
			if newPage != nil {
				return newPage, func() tea.Msg { return newPage.Msg() }
			}
			if lastCmd == nil {
				lastCmd = a.Tick(time.Nanosecond)
			}
			return m, lastCmd
		}
	}

	switch key {
	case "j", "J", "down":
		newPage = m.MoveDown()
	case "k", "K", "up":
		newPage = m.MoveUp()
	case "h", "H", "left":
		newPage = m.MoveLeft()
	case "l", "L", "right":
		newPage = m.MoveRight()
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		num, _ := strconv.Atoi(key)
		start := m.getPageStartIndex()
		if start+num >= len(m.menuList) {
			break
		}
		target := start + num
		if m.selectedIndex == target {
			newPage = m.enterMenuWithLoading(nil, nil)
			if m.pendingEnterMenu != nil {
				return m, a.RerenderCmd(true)
			}
		} else {
			m.selectedIndex = target
		}
	case "g":
		newPage = m.MoveTop()
	case "G":
		newPage = m.MoveBottom()
	case "n", "N", "enter":
		if m.selectedIndex < 0 {
			break
		}
		// Check for custom action first; fall through to submenu if none.
		if actionPage, actionCmd := m.menu.Action(m.app, m.selectedIndex); actionPage != nil || actionCmd != nil {
			return actionPage, actionCmd
		}
		newPage = m.enterMenuWithLoading(nil, nil)
		if m.pendingEnterMenu != nil {
			return m, a.RerenderCmd(true)
		}
	case "b", "B", "esc":
		newPage = m.BackMenu()
	case "r", "R":
		return m, a.RerenderCmd(true)
	case "/", "／", "、":
		if m.menu.IsSearchable() {
			m.inSearching = true
			m.searchInput.Focus()
		}
	}

	if newPage != nil {
		return newPage, func() tea.Msg { return newPage.Msg() }
	}
	return m, a.Tick(time.Nanosecond)
}

// mouse handle
func (m *Main) mouseMsgHandle(msg tea.MouseMsg, a *App) (Page, tea.Cmd) {
	mouse := msg.Mouse()

	// External controllers — run first, they have priority
	var (
		newPage         Page
		lastCmd         tea.Cmd
		stopPropagation bool
	)
	for _, c := range m.mouseCtrls {
		stopPropagation, newPage, lastCmd = c.MouseMsgHandle(msg, a)
		if stopPropagation {
			break
		}
	}
	if stopPropagation {
		if newPage != nil {
			return newPage, func() tea.Msg { return newPage.Msg() }
		}
		if lastCmd == nil {
			lastCmd = a.Tick(time.Nanosecond)
		}
		return m, lastCmd
	}

	// --- DEBUG: log all mouse click events ---
	// if f, err := os.OpenFile("/tmp/foxful-mouse-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
	// 	inMenu := m.mouseInMenuArea(mouse.Y)
	// 	log.New(f, "", log.LstdFlags|log.Lmicroseconds).Printf(
	// 		"button=%-10s x=%-3d y=%-3d inMenu=%v searching=%v",
	// 		mouse.Button.String(), mouse.X, mouse.Y, inMenu, m.inSearching,
	// 	)
	// 	f.Close()
	// }
	// --- END DEBUG ---

	// Only process concrete message types for built-in menu handling.
	// Bubbletea v2 sends both MouseClickMsg + MouseReleaseMsg per physical click;
	// we must ignore MouseReleaseMsg to avoid false double-click detection.
	switch msg.(type) {
	case tea.MouseClickMsg:
		return m.mouseClickHandle(mouse, a)
	case tea.MouseMotionMsg:
		return m.mouseMotionHandle(mouse, a)
	case tea.MouseReleaseMsg:
		// Ignore — hover and pointer state are driven by mouseMotionHandle.
		// Clearing them here would flicker the pointer when clicking a menu
		// item (mouse still over clickable area after release).
		return m, a.Tick(time.Nanosecond)
	case tea.MouseWheelMsg:
		return m.mouseWheelHandle(mouse, a)
	}

	// Nothing handled — tick to keep the event loop alive
	return m, a.Tick(time.Nanosecond)
}

// mouseClickHandle processes mouse click events for the built-in menu.
func (m *Main) mouseClickHandle(mouse tea.Mouse, a *App) (Page, tea.Cmd) {
	if m.inSearching {
		return m, a.Tick(time.Nanosecond)
	}

	switch mouse.Button {
	case tea.MouseLeft:
		// Check back button click (navigate back to parent menu)
		if m.isOverBackButton(mouse.X, mouse.Y, a) {
			newPage := m.BackMenu()
			if newPage != nil {
				return newPage, a.RerenderCmd(true)
			}
			return m, a.RerenderCmd(true)
		}

		// Check menu area click (existing behavior)
		if m.mouseInMenuArea(mouse.Y) {
			idx := m.menuItemAt(mouse.X, mouse.Y)
			if idx < 0 || idx >= len(m.menuList) {
				break
			}

			now := time.Now()
			doubleClickInterval := m.doubleClickInterval()

			// Position tolerance (±2px)
			deltaX := mouse.X - m.lastClickX
			if deltaX < 0 {
				deltaX = -deltaX
			}
			deltaY := mouse.Y - m.lastClickY
			if deltaY < 0 {
				deltaY = -deltaY
			}

			// Double-click: within interval AND close position
			if now.Sub(m.lastClickTime) <= doubleClickInterval &&
				deltaX <= 2 && deltaY <= 2 {
				// Double-click → enter submenu or execute custom action
				m.selectedIndex = idx
				m.lastClickTime = time.Time{} // reset

				// Check for custom action first; fall through to submenu if none.
				if actionPage, actionCmd := m.menu.Action(m.app, m.selectedIndex); actionPage != nil || actionCmd != nil {
					return actionPage, actionCmd
				}

				newPage := m.enterMenuWithLoading(nil, nil)
				if m.pendingEnterMenu != nil {
					return m, a.RerenderCmd(true)
				}
				if newPage != nil {
					return newPage, nil
				}
				// No submenu — stay on current page
				return m, a.RerenderCmd(true)
			}

			// Single click → just focus/select, never enter
			m.selectedIndex = idx
			m.lastClickTime = now
			m.lastClickX = mouse.X
			m.lastClickY = mouse.Y
			return m, a.RerenderCmd(true)
		}

		// Check status bar breadcrumb click
		if newPage := m.handleBreadcrumbClick(mouse.X, mouse.Y, a); newPage != nil {
			return newPage, a.RerenderCmd(true)
		}

	case tea.MouseBackward:
		if !m.mouseInMenuArea(mouse.Y) {
			break
		}
		// Back button: return to parent menu
		newPage := m.BackMenu()
		if newPage != nil {
			return newPage, a.RerenderCmd(true)
		}
		return m, a.RerenderCmd(true)

	case tea.MouseForward:
		if !m.mouseInMenuArea(mouse.Y) {
			break
		}
		// Forward button: enter selected item's submenu
		newPage := m.enterMenuWithLoading(nil, nil)
		if m.pendingEnterMenu != nil {
			return m, a.RerenderCmd(true)
		}
		if newPage != nil {
			return newPage, nil
		}
		return m, a.RerenderCmd(true)

	case tea.MouseMiddle:
		if !m.mouseInMenuArea(mouse.Y) {
			break
		}
		newPage := m.BackMenu()
		if newPage != nil {
			return newPage, a.RerenderCmd(true)
		}
		return m, a.RerenderCmd(true)


	case tea.MouseRight:
		if !m.mouseInMenuArea(mouse.Y) {
			break
		}
		idx := m.menuItemAt(mouse.X, mouse.Y)
		if idx < 0 || idx >= len(m.menuList) {
			break
		}
		// Fetch context menu items for this menu item
		items := m.menu.ContextMenuItems(a, idx)
		if len(items) == 0 {
			break
		}
		// Highlight the right-clicked item
		m.selectedIndex = idx
		// Create and show context menu at mouse position
		contextMenu := NewContextMenu(m.menu, idx, items, mouse.X, mouse.Y)
		a.pushModal(contextMenu)
		return m, a.RerenderCmd(true)

	}

	return m, a.Tick(time.Nanosecond)
}

// mouseWheelHandle processes mouse wheel events for the built-in menu.
func (m *Main) mouseWheelHandle(mouse tea.Mouse, a *App) (Page, tea.Cmd) {
	if m.inSearching {
		return m, a.Tick(time.Nanosecond)
	}

	switch mouse.Button {
	case tea.MouseWheelUp:
		if !m.mouseInMenuArea(mouse.Y) {
			break
		}
		newPage := m.MoveUp()
		if newPage != nil {
			return newPage, a.RerenderCmd(true)
		}
		return m, a.RerenderCmd(true)
	case tea.MouseWheelDown:
		if !m.mouseInMenuArea(mouse.Y) {
			break
		}
		newPage := m.MoveDown()
		if newPage != nil {
			return newPage, a.RerenderCmd(true)
		}
		return m, a.RerenderCmd(true)
	}

	return m, a.Tick(time.Nanosecond)
}

// doubleClickInterval returns the OS-specific double-click interval threshold.
func (m *Main) doubleClickInterval() time.Duration {
	switch runtime.GOOS {
	case "darwin":
		return 400 * time.Millisecond
	case "windows":
		return 500 * time.Millisecond
	default:
		return 300 * time.Millisecond
	}
}

func (m *Main) searchMenuHandle() {
	m.inSearching = false
	searchMenu := m.options.LocalSearchMenu
	if m.options.LocalSearchMenu == nil {
		searchMenu = DefaultSearchMenu()
	}
	searchMenu.Search(m.menu, m.searchInput.Value())
	m.EnterMenu(searchMenu, &MenuItem{Title: SearchResult, Subtitle: m.searchInput.Value()})
	m.searchInput.Blur()
	m.searchInput.Reset()
}

type menuStackItem struct {
	menuList      []MenuItem
	selectedIndex int
	menuCurPage   int
	menuTitle     *MenuItem
	menu          Menu
}

func (m *Main) MoveUp() Page {
	var (
		topHook = m.menu.TopOutHook()
		newPage Page
		res     bool
	)
	if m.isDualColumn {
		if m.selectedIndex-2 < 0 && topHook != nil {
			loading := NewLoading(m)
			loading.Start()
			if res, newPage = topHook(m); !res {
				loading.Complete()
				return newPage
			}
			// update menu ui
			m.menuList = m.menu.MenuViews()
			loading.Complete()
		}
		if m.selectedIndex-2 < 0 {
			return nil
		}
		m.selectedIndex -= 2
	} else {
		if m.selectedIndex-1 < 0 && topHook != nil {
			loading := NewLoading(m)
			loading.Start()
			if res, newPage = topHook(m); !res {
				loading.Complete()
				return newPage
			}
			m.menuList = m.menu.MenuViews()
			loading.Complete()
		}
		if m.selectedIndex-1 < 0 {
			return nil
		}
		m.selectedIndex--
	}
	if m.selectedIndex < m.getPageStartIndex() {
		newPage = m.PrePage()
	}
	return newPage
}

func (m *Main) MoveDown() Page {
	var (
		bottomHook = m.menu.BottomOutHook()
		newPage    Page
		res        bool
	)
	// Initial state: no item selected — select first item
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
		return nil
	}
	if m.isDualColumn {
		if m.selectedIndex+2 > len(m.menuList)-1 && bottomHook != nil {
			loading := NewLoading(m)
			loading.Start()
			if res, newPage = bottomHook(m); !res {
				loading.Complete()
				return newPage
			}
			m.menuList = m.menu.MenuViews()
			loading.Complete()
		}
		if m.selectedIndex+2 > len(m.menuList)-1 {
			return nil
		}
		m.selectedIndex += 2
	} else {
		if m.selectedIndex+1 > len(m.menuList)-1 && bottomHook != nil {
			loading := NewLoading(m)
			loading.Start()
			if res, newPage = bottomHook(m); !res {
				loading.Complete()
				return newPage
			}
			m.menuList = m.menu.MenuViews()
			loading.Complete()
		}
		if m.selectedIndex+1 > len(m.menuList)-1 {
			return nil
		}
		m.selectedIndex++
	}
	if m.selectedIndex >= m.menuCurPage*m.menuPageSize {
		newPage = m.NextPage()
	}
	return newPage
}

func (m *Main) MoveLeft() Page {
	if !m.isDualColumn || m.selectedIndex%2 == 0 || m.selectedIndex-1 < 0 {
		return nil
	}
	m.selectedIndex--
	return nil
}

func (m *Main) MoveRight() Page {
	if !m.isDualColumn || m.selectedIndex%2 != 0 {
		return nil
	}
	var (
		newPage Page
		res     bool
	)
	if bottomHook := m.menu.BottomOutHook(); m.selectedIndex >= len(m.menuList)-1 && bottomHook != nil {
		loading := NewLoading(m)
		loading.Start()
		if res, newPage = bottomHook(m); !res {
			loading.Complete()
			return newPage
		}
		m.menuList = m.menu.MenuViews()
		loading.Complete()
	}
	if m.selectedIndex >= len(m.menuList)-1 {
		return nil
	}
	m.selectedIndex++
	return newPage
}

func (m *Main) MoveTop() Page {
	if m.isDualColumn {
		m.selectedIndex = m.selectedIndex % 2
	} else {
		m.selectedIndex = 0
	}
	m.menuCurPage = 1
	return nil
}

func (m *Main) MoveBottom() Page {
	if m.isDualColumn && len(m.menuList)%2 == 0 {
		m.selectedIndex = len(m.menuList) + (m.selectedIndex%2 - 2)
	} else if m.isDualColumn && m.selectedIndex%2 != 0 {
		m.selectedIndex = len(m.menuList) - 2
	} else {
		m.selectedIndex = len(m.menuList) - 1
	}
	m.menuCurPage = int(math.Ceil(float64(len(m.menuList)) / float64(m.menuPageSize)))
	if m.isDualColumn && m.selectedIndex%2 != 0 && len(m.menuList)%m.menuPageSize == 1 {
		m.menuCurPage -= 1
	}
	return nil
}

func (m *Main) PrePage() Page {
	var (
		newPage Page
		res     bool
	)
	if prePageHook := m.menu.BeforePrePageHook(); prePageHook != nil {
		loading := NewLoading(m)
		loading.Start()
		if res, newPage = prePageHook(m); !res {
			loading.Complete()
			return newPage
		}
		loading.Complete()
	}
	if m.menuCurPage <= 1 {
		return nil
	}
	m.menuCurPage--
	m.hoveredMenuItemIdx = -1
	return newPage
}

func (m *Main) NextPage() Page {
	var (
		res     bool
		newPage Page
	)
	if nextPageHook := m.menu.BeforeNextPageHook(); nextPageHook != nil {
		loading := NewLoading(m)
		loading.Start()
		if res, newPage = nextPageHook(m); !res {
			loading.Complete()
			return newPage
		}
		loading.Complete()
	}
	if m.menuCurPage >= int(math.Ceil(float64(len(m.menuList))/float64(m.menuPageSize))) {
		return nil
	}

	m.menuCurPage++
	m.hoveredMenuItemIdx = -1
	return newPage
}

// enterMenuWithLoading initiates a deferred submenu entry. Unlike EnterMenu
// which runs the BeforeEnterMenuHook synchronously, this method defers the hook
// to the next tickMainMsg cycle. This gives the current View() cycle a chance
// to render the loadingTips text before the (potentially slow) API call in the
// hook blocks the event loop.
//
// Returns nil on success (the actual menu transition happens in the tick handler).
// Returns a non-nil Page on immediate failure (e.g., login required).
func (m *Main) enterMenuWithLoading(newMenu Menu, newTitle *MenuItem) Page {
	if m.pendingEnterMenu != nil {
		return nil // already pending, wait for completion
	}

	if newMenu == nil {
		newMenu = m.menu.SubMenu(m.app, m.selectedIndex)
	}
	if newTitle == nil && m.selectedIndex >= 0 && m.selectedIndex < len(m.menuList) {
		newTitle = &m.menuList[m.selectedIndex]
	}

	if newMenu == nil || newTitle == nil {
		return nil
	}

	m.hoveredBreadcrumbIdx = -1
	m.hoveredMenuItemIdx = -1
	m.hoveredBackButton = false

	stackItem := &menuStackItem{
		menuList:      m.menuList,
		selectedIndex: m.selectedIndex,
		menuCurPage:   m.menuCurPage,
		menuTitle:     m.menuTitle,
		menu:          m.menu,
	}
	m.menuStack.Push(stackItem)

	loading := NewLoading(m)
	loading.Start() // sets m.loadingTips so the next View() shows progress

	m.pendingEnterMenu = &enterMenuDeferred{
		newMenu:   newMenu,
		newTitle:  newTitle,
		loading:   loading,
		stackItem: stackItem,
	}

	return nil
}

func (m *Main) EnterMenu(newMenu Menu, newTitle *MenuItem) Page {
	if (newMenu == nil || newTitle == nil) && m.selectedIndex >= len(m.menuList) {
		return nil
	}

	if newMenu == nil {
		newMenu = m.menu.SubMenu(m.app, m.selectedIndex)
	}
	if newTitle == nil {
		if m.selectedIndex >= 0 {
			newTitle = &m.menuList[m.selectedIndex]
		}
	}

	m.hoveredBreadcrumbIdx = -1
	m.hoveredMenuItemIdx = -1
	m.hoveredBackButton = false

	stackItem := &menuStackItem{
		menuList:      m.menuList,
		selectedIndex: m.selectedIndex,
		menuCurPage:   m.menuCurPage,
		menuTitle:     m.menuTitle,
		menu:          m.menu,
	}
	m.menuStack.Push(stackItem)

	if newMenu == nil {
		m.menuStack.Pop()
		return nil
	}

	var (
		res     bool
		newPage Page
	)
	if enterMenuHook := newMenu.BeforeEnterMenuHook(); enterMenuHook != nil {
		loading := NewLoading(m)
		loading.Start()
		if res, newPage = enterMenuHook(m); !res {
			loading.Complete()
			m.menuStack.Pop()
			return newPage
		}
		loading.Complete()
	}
	if newMenu != nil {
		newMenu.FormatMenuItem(newTitle)
	}

	menuList := newMenu.MenuViews()

	m.menu = newMenu
	m.menuList = menuList
	m.menuTitle = newTitle
	m.selectedIndex = 0
	m.menuCurPage = 1

	return newPage
}

func (m *Main) BackMenu() Page {
	if m.menuStack.Len() <= 0 {
		return nil
	}

	m.hoveredBreadcrumbIdx = -1
	m.hoveredMenuItemIdx = -1
	m.hoveredBackButton = false

	var (
		stackItem = m.menuStack.Pop()
		newPage   Page
		res       bool
	)
	if backMenuHook := m.menu.BeforeBackMenuHook(); backMenuHook != nil {
		loading := NewLoading(m)
		loading.Start()
		if res, newPage = backMenuHook(m); !res {
			loading.Complete()
			m.menuStack.Push(stackItem)
			return newPage
		}
		loading.Complete()
	}
	m.menu.FormatMenuItem(m.menuTitle)

	stackMenu, ok := stackItem.(*menuStackItem)
	if !ok {
		return nil
	}

	m.menuList = stackMenu.menuList
	m.menu = stackMenu.menu
	m.menuTitle = stackMenu.menuTitle
	m.menu.FormatMenuItem(m.menuTitle)
	m.selectedIndex = stackMenu.selectedIndex
	m.menuCurPage = stackMenu.menuCurPage

	return newPage
}

// BackToMenu pops count levels from the menu stack (or until the stack is
// empty). The current menu's BeforeBackMenuHook is called first; intermediate
// menus that are skipped over do NOT get their hooks called. Must only be
// called from the mouse click handler after hit-testing breadcrumb segments.
func (m *Main) BackToMenu(count int) Page {
	if count <= 0 {
		return nil
	}
	if m.menuStack.Len() <= 0 {
		return nil
	}

	m.hoveredBreadcrumbIdx = -1
	m.hoveredMenuItemIdx = -1
	m.hoveredBackButton = false

	// Call hook on the current (deepest) menu once
	var newPage Page
	if backMenuHook := m.menu.BeforeBackMenuHook(); backMenuHook != nil {
		loading := NewLoading(m)
		loading.Start()
		var res bool
		if res, newPage = backMenuHook(m); !res {
			loading.Complete()
			// Hook refused — don't pop anything
			if newPage == nil {
				return nil
			}
			return newPage
		}
		loading.Complete()
	}
	m.menu.FormatMenuItem(m.menuTitle)

	// Pop count levels, keeping the last popped item as the target state.
	var targetStackItem *menuStackItem
	for i := 0; i < count; i++ {
		if m.menuStack.Len() <= 0 {
			break
		}
		item := m.menuStack.Pop()
		if si, ok := item.(*menuStackItem); ok {
			targetStackItem = si
		}
	}
	if targetStackItem == nil {
		return nil
	}

	// Restore the target state
	m.menuList = targetStackItem.menuList
	m.menu = targetStackItem.menu
	m.menuTitle = targetStackItem.menuTitle
	m.menu.FormatMenuItem(m.menuTitle)
	m.selectedIndex = targetStackItem.selectedIndex
	m.menuCurPage = targetStackItem.menuCurPage

	return newPage
}

// breadcrumbSegmentAt returns the breadcrumb segment display index and depth
// index at the given screen position (x, y). Returns (-1, 0, false) when no
// clickable ancestor segment is at that position. Only works for
// DefaultStatusBar layout — returns false for other status bars.
func (m *Main) breadcrumbSegmentAt(x, y int, a *App) (segIdx int, depthIdx int, ok bool) {
	if m.menuStack.Len() <= 0 {
		return -1, 0, false
	}
	if m.statusBar == nil {
		return -1, 0, false
	}
	if _, ok := m.statusBar.(*DefaultStatusBar); !ok {
		return -1, 0, false
	}

	// Status bar occupies the last rows (DefaultStatusBar is single-row).
	h := a.WindowHeight()
	if y < h-1 {
		return -1, 0, false
	}

	segments := computeBreadcrumbSegments(m)
	if len(segments) == 0 {
		return -1, 0, false
	}

	ss := style.CurrentStyleSet()
	pathLabel := ss.StatusBarNuggetLabel.Render(" » ")
	labelW := lipgloss.Width(pathLabel)
	segStartX := labelW + 1

	for i, seg := range segments {
		if seg.IsEllipsis {
			segStartX += seg.DisplayWidth + 3
			continue
		}
		if seg.IsLast {
			break
		}

		segEndX := segStartX + seg.DisplayWidth
		if x >= segStartX && x < segEndX {
			return i, seg.DepthIndex, true
		}

		segStartX = segEndX + 3 // " / " = 3 chars
	}

	return -1, 0, false
}

// isOverClickableElement returns true if the given screen position is over
// an interactive/clickable element that should show a pointer cursor.
func (m *Main) isOverClickableElement(x, y int, a *App) bool {
	// 1. Back button (clickable to navigate back to parent menu)
	if m.isOverBackButton(x, y, a) {
		return true
	}

	// 2. Breadcrumb ancestor segment (clickable to navigate back)
	if _, _, ok := m.breadcrumbSegmentAt(x, y, a); ok {
		return true
	}

	// 3. Menu list area (single-click selects, double-click enters)
	if !m.inSearching && m.mouseInMenuArea(y) {
		idx := m.menuItemAt(x, y)
		if idx >= 0 && idx < len(m.menuList) {
			return true
		}
	}

	return false
}

// mouseMotionHandle processes mouse motion events for hover effects and
// terminal mouse pointer shape changes. It updates both the breadcrumb
// hover rendering state and the global pointer cursor.
func (m *Main) mouseMotionHandle(mouse tea.Mouse, a *App) (Page, tea.Cmd) {
	oldBreadcrumbHover := m.hoveredBreadcrumbIdx
	oldPointerActive := m.hoverPointerActive
	oldBackButtonHover := m.hoveredBackButton

	if m.inSearching {
		stateChanged := false
		if oldBreadcrumbHover != -1 {
			m.hoveredBreadcrumbIdx = -1
			stateChanged = true
		}
		if m.hoveredMenuItemIdx != -1 {
			m.hoveredMenuItemIdx = -1
			stateChanged = true
		}
		if m.hoveredBackButton {
			m.hoveredBackButton = false
			stateChanged = true
		}
		if oldPointerActive {
			m.hoverPointerActive = false
			stateChanged = true
		}
		if stateChanged {
			return m, tea.Sequence(a.RerenderCmd(true), a.SetMousePointer("default"))
		}
		return m, nil
	}

	// Update breadcrumb hover (visual underline effect)
	segIdx, _, bcOk := m.breadcrumbSegmentAt(mouse.X, mouse.Y, a)
	if bcOk {
		m.hoveredBreadcrumbIdx = segIdx
	} else {
		m.hoveredBreadcrumbIdx = -1
	}

	// Update menu item hover
	oldMenuItemHover := m.hoveredMenuItemIdx
	if !m.inSearching && m.mouseInMenuArea(mouse.Y) {
		idx := m.menuItemAt(mouse.X, mouse.Y)
		if idx >= 0 && idx < len(m.menuList) {
			m.hoveredMenuItemIdx = idx
		} else {
			m.hoveredMenuItemIdx = -1
		}
	} else {
		m.hoveredMenuItemIdx = -1
	}

	// Update back button hover
	m.hoveredBackButton = m.isOverBackButton(mouse.X, mouse.Y, a)

	// Update global pointer state
	m.hoverPointerActive = m.isOverClickableElement(mouse.X, mouse.Y, a)

	// Compute commands for state changes
	var cmds []tea.Cmd
	if m.hoveredBreadcrumbIdx != oldBreadcrumbHover || m.hoveredMenuItemIdx != oldMenuItemHover || m.hoveredBackButton != oldBackButtonHover {
		cmds = append(cmds, a.RerenderCmd(true))
	}
	if m.hoverPointerActive != oldPointerActive {
		if m.hoverPointerActive {
			cmds = append(cmds, a.SetMousePointer("pointer"))
		} else {
			cmds = append(cmds, a.SetMousePointer("default"))
		}
	}

	if len(cmds) > 0 {
		return m, tea.Sequence(cmds...)
	}
	return m, nil
}

// handleBreadcrumbClick handles a left-click on the status bar breadcrumb.
// Navigates back to the clicked menu level. Returns nil if no clickable
// breadcrumb segment was hit.
func (m *Main) handleBreadcrumbClick(x, y int, a *App) Page {
	_, depthIdx, ok := m.breadcrumbSegmentAt(x, y, a)
	if !ok {
		return nil
	}

	// Compute full path length for pop count
	fullPathLen := m.menuStack.Len()
	if m.menuStack.Len() > 0 {
		stackItems := m.menuStack.ToSlice()
		lastItem := stackItems[len(stackItems)-1].(*menuStackItem)
		if lastItem.menuTitle.Title != m.menuTitle.Title {
			fullPathLen++
		}
	} else {
		fullPathLen = 1
	}

	popCount := fullPathLen - 1 - depthIdx
	return m.BackToMenu(popCount)
}

func TickMain(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return tickMainMsg{}
	})
}
