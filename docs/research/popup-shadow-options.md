# 弹窗/模态框阴影可行性研究

> **结论（2026-07-22）**：可以为弹窗增加"阴影"以强化其"悬浮/抬升"的层级感，但**终端没有 CSS `box-shadow` 那样的原生扩散阴影**；只能用字符+背景色近似。推荐做法是**新增一个独立的、z 序位于基础页面之上且低于对应弹窗、向右下偏移 (+1,+1) 的"阴影专用" Compositor 图层**，而不是给 `PopupBase` 加 margin。原因：`Popup.Render` 的输出尺寸会被 `App.compositePopups` 直接用于定位与 `SetBounds`，再被鼠标命中/按钮几何计算消费；给 `PopupBase` 加右/下 margin 会连带放大可点击边界并移动按钮坐标，属于回归。**规范/上游**：[W3C CSS Backgrounds & Borders L3 `box-shadow`](https://www.w3.org/TR/css-backgrounds-3/#box-shadow)、[Lip Gloss v2.0.4 `layer.go`](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/layer.go)；**本地**：`model/popup.go:233-346`、`model/app.go:432-453`、`style/theme.go:777-781`。

> **状态更新（2026-07-22）**：Popup 已完成 Surface API 重构：`NewPopup(PopupSpec)` 取代旧构造器，正文 ANSI 背景会在 cell 级别统一为 `Popup.Surface`，旧 `PopupBase` / `PopupDim` / `PopupButton` API 已删除。下文的本地源码引用记录的是重构前快照；若实施阴影方案，应基于新的私有 `Popup.render`、精确 action bounds 和 `PopupStyleSet` 重新落点。

## 范围、方法与证据规则

本报告只检查：本仓库源码、随仓库 vendored 的 `charm.land/lipgloss/v2`、`charm.land/bubbletea/v2`（fork）与 `github.com/charmbracelet/ultraviolet` 源码，以及协议/规范所有者（W3C、Unicode、Material Design）的官方文档。没有使用博客、教程或第三方经验帖。

下文对每条结论区分两类断言：

- **【事实】**：可由本 checkout 源码或官方规范直接证实。每条给出 **规范/上游**（拥有 API/协议的 canonical URL）与 **本地**（本 checkout 中的 `path:line`）。
- **【建议】**：基于事实的设计推断，不是既成事实。

行号以调研时工作树为准；URL 是便于复核的 canonical 来源，而非对本地 fork 的替代。

## 1. 现状：弹窗如何构建与合成

### 1.1 弹窗渲染

- **【事实】** `Popup.Render` 把标题、正文、按钮行用 `lipgloss.JoinVertical(lipgloss.Center, …)` 纵向拼接，再交给 `styles.PopupBase.Render(inner)` 包一层带边框的容器返回。它返回的是一个多行字符串，不含任何"阴影"字符。**规范/上游**：[Lip Gloss v2 `Style.Render`](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/style.go)、[`JoinVertical`](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/join.go)；**本地**：`model/popup.go:233-346`（拼接见 `:327-346`）。
- **【事实】** `PopupBase` 当前样式为：圆角边框、边框色取 `Border`（回退 `Accent`）、背景色取 `Popup.Bg`（回退 `Surface`）、四周 1 格 padding。没有 margin、没有阴影。**规范/上游**：[Lip Gloss v2 `borders.go`（`RoundedBorder`）](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/borders.go)；**本地**：`style/theme.go:777-781`、边框色/背景色解析 `style/theme.go:644-646`。
- **【事实】** 主题已定义 `Surface`（"抬升元素（卡片、面板、弹窗）的背景"）与 `Border` 两个语义 token，且各内置主题都给了具体值（如 dark: `Surface #242424` / `Border #333333`）。因此"用背景色表达层级"已是既有设计词汇。**规范/上游**：（无外部依赖，仓库自有 token）；**本地**：`style/theme.go:69-70`、`style/theme.go:165-167,189-191,238-241,259-262,280-283`。

### 1.2 合成与定位

- **【事实】** `App.compositePopups` 把基础页面作为最底层 (`layout.NewLayer(baseContent)`)，然后对 `popupStack` 中每个弹窗调用 `p.Render(...)`，**用渲染结果的行数/最大可视宽度算出 `popupW/popupH`**，再 `computePosition` 得到 `(x,y)`，`SetBounds(x,y,popupW,popupH)`，最后追加 `layout.NewLayer(popupContent).X(x).Y(y)`。其最终绘制优先级由 `Compositor` 的 z 排序决定。**规范/上游**：[Lip Gloss v2 `layer.go`（`Compositor`/`Layer`）](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/layer.go)；**本地**：`model/app.go:432-453`。
- **【事实】** `Compositor` 按绝对 z 值从低到高绘制；其实现使用 Go 的 `slices.SortFunc`，而该 API **不保证**相等元素的原始顺序。当前基础页和弹窗层都使用默认 z 值，现有 append 顺序表达了"后入栈弹窗在前"的意图，但不应把同 z 的加入顺序作为新阴影层的正确性契约。**规范/上游**：[Lip Gloss v2 `layer.go`（`Compositor.flatten`/`Draw`）](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/layer.go)、[Go `slices.SortFunc`](https://pkg.go.dev/slices#SortFunc)；**本地**：`model/app.go:435-452`、`vendor/charm.land/lipgloss/v2/layer.go:224-283`。
- **【建议】** 阴影实现应为每个栈索引 `i` 明确分配 `shadowZ = 2*i+1`、`popupZ = 2*i+2`（基础页面保留 z=0）。这使阴影始终覆盖页面、被自己的弹窗盖住，并保持较新弹窗及其阴影位于较旧弹窗之上。

### 1.3 一个已存在但未接线的 token

- **【事实】** `StyleSet.PopupDim`（"变暗弹窗背后的内容"）在主题里已定义并解析，但在 `model/` 中**没有任何调用点**——即当前实现并未对弹窗背后的页面做变暗处理。这与 W3C 模态对话框模式建议的"惰性内容应被视觉遮蔽/变暗"存在差距，且与阴影是互补而非替代关系。**规范/上游**：[W3C WAI-ARIA APG Dialog (Modal) Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/)；**本地**：`style/theme.go:378-380,802`（定义/解析），`model/`（无引用，`compositePopups` 直接叠加不变暗，见 `model/app.go:432-453`）。

## 2. 阴影在终端里意味着什么（技术可行性）

### 2.1 为什么没有"真正的" box-shadow

- **【事实】** CSS `box-shadow` 是一个图形化属性：它按 `<offset-x> <offset-y> <blur-radius> <spread-radius> <color>` 在盒子边界外绘制可带**模糊/扩散**的阴影，属于逐像素的光栅合成。这依赖像素画布，终端的字符网格没有等价能力。**规范/上游**：[W3C CSS Backgrounds & Borders L3 §`box-shadow`](https://www.w3.org/TR/css-backgrounds-3/#box-shadow)。
- **【事实】** 终端 UI 只能操作"字符 cell + 前景色/背景色/属性"。本项目的渲染链是 Lip Gloss 把样式字符串交给 Ultraviolet 的 cell buffer，再由 fork 的 `foxful` renderer 按文本行截断/补空格重绘。没有 alpha 合成，也没有子 cell 精度。因此"阴影"只能是**近似**：偏移一格的深色 cell（可选半块字符 `░▒▓█` / `▄▀▌▐`）。**规范/上游**：[Unicode Block Elements (U+2580–U+259F) 码表](https://www.unicode.org/charts/PDF/U2580.pdf)、[Ultraviolet cell buffer](https://github.com/charmbracelet/ultraviolet)；**本地**：`vendor/github.com/charmbracelet/ultraviolet/styled.go:50-61`、`vendor/github.com/charmbracelet/ultraviolet/cell.go:14-18`、`model/app.go:322-324`（强制 foxful renderer）。
- **【建议】** 结论：能给弹窗做一个"硬边、单格偏移"的投影（drop shadow）来强化 Material Design 所说的"elevation（抬升）→ 用阴影表达层级"的观感，但**不要**在文档或注释里把它称作 `box-shadow` 或"扩散阴影"，那会误导。**规范/上游**：[Material Design 3 – Elevation](https://m3.material.io/styles/elevation/overview)。

### 2.2 两条候选实现路径

| 方案 | 做法 | 对 `Render()` 尺寸的影响 | 对鼠标/按钮几何的影响 | 结论 |
|---|---|---|---|---|
| **A. margin 伪阴影** | 给 `PopupBase` 加右/下 1 格 `Margin` + `MarginBackground`(深色) + `MarginChar` | **会**把阴影格计入渲染宽高 | **会**放大 `SetBounds` 的 `w/h`，`buttonAt` 的 `p.width-8`/`p.y+p.height-3` 随之偏移 → 命中区域与按钮坐标错位 | **不推荐**（除非同时改输入几何） |
| **B. 独立阴影图层** | 在 `compositePopups` 里，为每个弹窗额外 push 一个 z 序高于基础页、低于该弹窗的深色实心块 `Layer`，位置 `X(x+1).Y(y+1)` | **不影响** `p.Render()` 输出，`popupW/popupH` 与 `SetBounds` 不变 | **无影响**：输入几何仍基于未加阴影的弹窗尺寸 | **推荐** |

- **【事实】** 方案 A 依赖的 API 确实存在：`Style.MarginBackground(color.Color)` 与 `Style.MarginChar(rune)` 均在 vendored Lip Gloss 中。但 margin 会进入 `Style.Render` 的输出字符串，从而增大 `lipgloss.Height`/可视宽度——这正是 `compositePopups` 用来算 `popupW/popupH` 并传给 `SetBounds` 的量。**规范/上游**：[Lip Gloss v2 `set.go`（`MarginBackground`/`MarginChar`）](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/set.go)；**本地**：`vendor/charm.land/lipgloss/v2/set.go:452-465`、`model/app.go:439-450`。
- **【事实】** 鼠标命中与按钮几何强依赖 `p.width/p.height`：命中框为 `mouse.X ∈ [p.x, p.x+p.width)`、`mouse.Y ∈ [p.y, p.y+p.height)`；按钮行取 `buttonY = p.y + p.height - 3`，按钮宽取 `(p.width - 8) / len(Buttons)`。若方案 A 把阴影计入 `p.width/p.height`，这些坐标都会漂移。**规范/上游**：（仓库自有几何逻辑）；**本地**：`model/popup.go:475-478`（命中框）、`model/popup.go:572-593`（`buttonAt` 几何）。

## 3. 推荐实现（最小、与仓库风格一致）

目标：**不改弹窗自身的渲染尺寸与输入几何**，只在合成阶段"垫一层阴影"。这样 `Popup.Render`、`SetBounds`、`buttonAt`、拖拽等逻辑全部零改动。

### 3.1 新增一个语义化 token（与既有 token 并列，不新造第二套约定）

- 在 `style.Theme` 增加 `Shadow color.Color`（"抬升元素的投影色"，nil 时回退到一个比 `Background` 更暗的值或直接复用现有暗色），在 `StyleSet` 增加 `PopupShadow lipgloss.Style`，解析逻辑放在现有 Popup 解析块附近（`style/theme.go:644-656` 与 `style/theme.go:777-802` 一致的写法）。这样与 `Surface`/`Border`/`PopupDim` 属于同一套"弹窗视觉 token"，不引入新风格。**本地锚点**：`style/theme.go:69-70`（Theme 颜色区）、`style/theme.go:354-380`（StyleSet Popup 区）、`style/theme.go:644-656,777-802`（解析/默认区）。

### 3.2 在合成阶段插入阴影层

在 `model/app.go` 的 `compositePopups` 内，对每个索引为 `i` 的弹窗，先算出未加阴影的 `popupW/popupH`（保持现有代码不动），再构造一个"实心深色块"作为**z 序高于基础页面、低于该弹窗**的阴影层，位置 `X(x+1).Y(y+1)`：

```go
// compositePopups 内，保持现有 popupW/popupH/x/y/SetBounds 不变。
// 在 append 弹窗层之前，先垫一层阴影（低于对应弹窗、高于基础页）。
// 使用显式 z；不能依赖 slices.SortFunc 对相同 z 元素的顺序。
shadowZ := 2*i + 1
shadowStyle := style.CurrentStyleSet().PopupShadow // 背景=Shadow 色的实心块
shadowBlock := shadowStyle.Width(popupW).Height(popupH).Render("")
layers = append(layers,
    layout.NewLayer(shadowBlock).X(x+1).Y(y+1).Z(shadowZ), // 阴影：右下偏移，位于页面与弹窗之间
)
layers = append(layers,
    layout.NewLayer(popupContent).X(x).Y(y).Z(shadowZ+1),  // 弹窗：原样，覆盖重叠区
)
```

- **【事实】** 该写法只用到已存在的 `Layer.X/Y/Z` 与 `Compositor` 的低 z→高 z 绘制能力；`layout` 包已 re-export `NewLayer`/`NewCompositor`。Go 的 `slices.SortFunc` 对相同 z 不保证稳定排序，故 §3.2 的显式 z 是正确分层的必要条件。阴影块的空格 cell 只会落在弹窗右/下外沿一格（重叠区被更高 z 的弹窗层覆盖），因此得到"右下投影"。**规范/上游**：[Lip Gloss v2 `layer.go`（`Layer.Z`、`Compositor` 按 z 排序）](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/layer.go)、[Go `slices.SortFunc`](https://pkg.go.dev/slices#SortFunc)；**本地**：`layout/layout.go:58-62`、`vendor/charm.land/lipgloss/v2/layer.go:69-72,224-283`。
- **【建议】** 若希望阴影只出现在右侧+底部两条边（更贴近真实 drop shadow，而非整块背衬），可把阴影层内容改为"右 1 列 + 下 1 行"的 L 形块，或用半块字符 `▓/░`；但整块深色背衬实现最简单、跨终端最稳。宽字符/emoji 不可用于阴影格，避免宽度错位。**规范/上游**：[Unicode Block Elements 码表](https://www.unicode.org/charts/PDF/U2580.pdf)。
- **【建议】** 阴影应可关闭/主题可控：`Shadow` 为 nil（或等于 `Background`）时不 append 阴影层，保持"零视觉变化"的降级路径，尊重低对比/纯文本终端。

### 3.3 精确改动清单（供实施者）

- `style/theme.go`：`Theme` 加 `Shadow color.Color`（约 `:69-70` 区）；`StyleSet` 加 `PopupShadow lipgloss.Style`（约 `:354-380` 区）；在 `:644-656` 解析 `shadowColor := or(theme.Shadow, <darker fallback>)`，在 `:777-802` 处 `base.PopupShadow = lipgloss.NewStyle().Background(shadowColor)`。
- `model/app.go`：`compositePopups`（`:432-453`）将基础页设为 z=0；循环改为取得索引 `i`，在 append 弹窗层前 append z=`2*i+1` 的阴影层，并将原弹窗层设为 z=`2*i+2`；**不改** `popupW/popupH/computePosition/SetBounds`。
- **不改**：`model/popup.go` 全文（渲染、几何、鼠标、拖拽均无需变动）。

## 4. 权衡：视觉 / 可访问性 / 性能

### 4.1 视觉

- **【建议】** 收益：右下投影强化"弹窗浮于页面之上"的层级感，与主题已有的 `Surface`（抬升背景）语义一致，符合 Material Design 用 elevation/阴影表达层级的思路。**规范/上游**：[Material Design 3 – Elevation](https://m3.material.io/styles/elevation/overview)。
- **【建议】** 风险：阴影会**多占 1 行 1 列**的屏幕空间。在贴边/贴底 anchor（如 `AnchorBottom*`/`AnchorRight*`）或窄终端下，阴影可能被裁掉或紧贴屏幕边缘而不自然。实施时应在阴影层坐标 `x+1,y+1` 超出 `WindowWidth/WindowHeight` 时省略对应边（钳制）。**本地**：`model/popup.go:369-418`（anchor/`computePosition`/`anchorOrigin`）。

### 4.2 可访问性

- **【事实】** 阴影是纯装饰，不承载语义；W3C 明确要求"不要用背景/装饰作为传达重要信息的唯一手段"。因此阴影**不能**替代模态语义或焦点管理。**规范/上游**：[W3C CSS Backgrounds L3（不得以背景图作为传达信息的唯一手段，援引 WCAG F3）](https://www.w3.org/TR/css-backgrounds-3/#the-background-image)、[WAI-ARIA APG Dialog (Modal) Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/)。
- **【建议】** 更高价值的可访问性改进是把已定义却未接线的 `PopupDim` 用起来（变暗背后内容），这正是 APG 对模态"惰性内容应被视觉遮蔽/变暗"的建议；阴影与变暗是互补的两件事。低对比/纯文本终端下阴影应可关闭（§3.2 降级路径）。**规范/上游**：[WAI-ARIA APG Dialog (Modal) Pattern（inert 内容应被视觉 obscured/dimmed）](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/)；**本地**：`style/theme.go:378-380,802`、`model/app.go:432-453`。

### 4.3 性能

- **【建议】** 每帧每个弹窗多合成一层 `popupW×popupH` 的实心块，成本是常量级的 cell 写入，远小于整屏重绘；对本项目按行重绘的 foxful renderer 无实质影响（阴影层大多是静态背景色块，diff 后基本不重画）。这是基于 renderer 逐行截断/补空格算法的推断，而非对终端性能的实测承诺。**规范/上游**：[fork `foxful_renderer.go`](https://github.com/go-musicfox/bubbletea/blob/v2.0.8-foxful-1.2/foxful_renderer.go)；**本地**：`model/app.go:322-324`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-166`。

## 5. 可验证的测试计划（scoped）

实施后按下述**范围内**验证（不跑全仓库套件、不跑 formatter/linter）：

1. **单测（几何不回归，核心断言）**：新增 `model/popup_shadow_test.go`，构造 `App{windowWidth,windowHeight}` + 一个 `NewConfirmPopup`，调用 `compositePopups`，断言 `p`（经 `SetBounds` 后）的 `width/height` 与"未启用阴影"时**完全相等**，且 `buttonAt(...)` 在同一坐标返回同一按钮索引。这直接守护"阴影不得改动输入几何"这一契约。参照现有 `model/statusbar_test.go` 的 `App{...}` 直接构造 + `lipgloss.Height` 断言风格。**本地**：`model/statusbar_test.go:10-26`、`model/popup.go:420-426,566-594`。
   - 命令（仅本包）：`go test ./model/ -run 'Popup.*Shadow'`
2. **快照/可视断言**：对 `compositePopups` 的输出字符串断言：在弹窗右外沿列（`x+popupW` 处）与下外沿行（`y+popupH` 处）出现阴影背景色的 cell，而弹窗矩形内不变。可用 `lipgloss.Width`/按行切分定位。
3. **降级路径**：`Shadow == nil`（或等于 `Background`）时，`compositePopups` 输出与改动前**逐字节一致**（不 append 阴影层）。
4. **手动冒烟**：`go run ./example/popup`，触发 Info/Confirm/堆叠/各 anchor 弹窗，肉眼确认阴影出现在右下、按钮点击命中正确、拖拽后阴影跟随、贴边 anchor 不溢出。**本地**：`example/popup/main.go:21-95,116-145`。

> 说明：本报告提交时未改动任何产品代码/配置/既有文档；以上代码片段与改动清单均为**建议**，供后续实施与复核。
