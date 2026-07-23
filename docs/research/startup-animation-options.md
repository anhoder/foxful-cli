# 启动页终端动画可行性研究

> **结论（2026-07-22）**：可以，而且无需先引入图形协议就能把现有启动页显著升级。首选是一个**以字符为基础、按终端色彩能力降级、可关闭/低动态**的「徽标分段揭示 + 低频色带扫光 + 语义化加载指示器」；它与当前 Bubble Tea 页面/消息模型直接匹配。终端图片应是后续、严格 opt-in 的实验能力，不应成为默认路径：现有 fork 的行渲染器会按文本行截断、填充并重绘，而不是维护图片单元的位置模型。**规范/上游**：[fork `foxful_renderer.go`](https://github.com/go-musicfox/bubbletea/blob/v2.0.8-foxful-1.2/foxful_renderer.go)；**本地**：`model/startup.go:65-143`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-166`。

## 范围、方法与证据规则

本报告只检查仓库、随仓库 vendored 的源码、Charmbracelet/Charm 的官方源码与文档，以及协议所有者的规范；没有使用博客、教程或兼容性排行榜。下文每个事实性结论同时给出：

- **规范/上游**：拥有 API 或协议的 canonical URL；
- **本地**：本 checkout 中支撑该结论的 `path:line`。行号以调研时工作树为准；URL 是便于复核的 canonical 来源，而非对本地 fork 的替代。

## 1. 现状：已有动画基础，但视觉状态只有一个维度

### 1.1 实现与生命周期

- `StartupPage` 已是独立 `Page`：`Init` 立即投递一次 tick；每个 `tickStartupMsg` 将累计时长加上配置的 `TickDuration`、计算百分比、可应用 `ease.OutBounce`，随后再安排一个 `tea.Tick`；达到时长后跳至 `nextPage` 并要求清屏重绘。本质上已经是适合 frame animation 的 Elm/Bubble Tea 状态机，而非阻塞式 `sleep`。Bubble Tea 的官方 `Tick` 文档明确说明它只发送一次消息、需要在收到消息后再次返回 `Tick` 才会循环。**规范/上游**：[Bubble Tea v2.0.8 `commands.go`](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/commands.go#L116-L164)；**本地**：`model/startup.go:31-81`、`vendor/charm.land/bubbletea/v2/commands.go:116-164`。
- 默认启动为 2 秒、16 ms tick（理论约 125 帧）、启用 bounce，且默认交互使用备用屏幕。`App.Run` 在没有自定义初始页时创建 Startup→Main 链；`App.View` 将 `Options.AltScreen` 直接声明给 Tea view。**规范/上游**：[仓库 `model/options.go`](https://github.com/anhoder/foxful-cli/blob/master/model/options.go)、[Bubble Tea v2 `View`/alt-screen 升级说明](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/UPGRADE_GUIDE_V2.md#declarative-view-properties)；**本地**：`model/options.go:49-87`、`model/app.go:251-268`、`model/app.go:294-324`。
- 当前视图固定纵向拼接「7 行字库 logo、空行、倒计时、空行、整窗口宽进度条」，再置于终端正中。logo 每帧会重新由 `Welcome` 生成，但只有单一前景色，视觉上不随进度改变。字库只覆盖预定义字符，未收录的 rune 被忽略。**规范/上游**：[Lip Gloss v2 README（样式、尺寸与对齐）](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/README.md)；**本地**：`model/startup.go:84-143`、`util/alpha_ascii.go:542-570`。
- 进度条已是逐 cell 的颜色坡度：宽度变化时缓存 `MakeRamp`，已完成 cell 逐个调用 `style.FG`；空白部分用主题 `ProgressEmpty`。因此「移动高光/分段/字符变体」不需要新渲染后端，只需要把渲染输入从单一 `fullSize` 扩为动画状态。**规范/上游**：[Lip Gloss v2 色彩 profiles 与降级说明](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/README.md#colors)；**本地**：`model/startup.go:17-29,134-143`、`model/progress.go:10-55`、`style/theme.go:765-770`。

### 1.2 依赖版本与渲染现实

- 直接依赖为 `charm.land/bubbletea/v2 v2.0.8`、`bubbles/v2 v2.1.0`、`lipgloss/v2 v2.0.4`、`colorprofile v0.4.3`；但 Bubble Tea 被替换为 `github.com/go-musicfox/bubbletea/v2 v2.0.8-foxful-1.2`，并且 vendor 清单只包含 Bubbles 的 `cursor`、`key`、`textinput`，没有 `spinner` 或 `progress` 包。故「直接复用 Bubbles spinner/progress」不是零改动：需要更新依赖/vendor；而现有简单进度条足以实现第一版。**规范/上游**：[仓库 `go.mod`](https://github.com/anhoder/foxful-cli/blob/master/go.mod)、[Bubbles 官方仓库](https://github.com/charmbracelet/bubbles)；**本地**：`go.mod:5-51`、`vendor/modules.txt:1-11`。
- 本应用强制 `tea.WithFoxfulRenderer()`。该 fork renderer 将内容按换行切成文本行、按 terminal width 截断、以空格补至整行、再用游标移动重画；它的 `setColorProfile` 是空实现。换言之，普通字符帧最契合；原始图片 escape sequence 虽可能被送到 stdout，却不受该 renderer 的行宽/占位/清除管理。**规范/上游**：[fork `foxful_renderer.go`](https://github.com/go-musicfox/bubbletea/blob/v2.0.8-foxful-1.2/foxful_renderer.go)、[fork `options.go`](https://github.com/go-musicfox/bubbletea/blob/v2.0.8-foxful-1.2/options.go)；**本地**：`model/app.go:322-324`、`vendor/charm.land/bubbletea/v2/options.go:104-109`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:70-166,204`。
- Bubble Tea 启动时检测 `colorprofile` 并把 `ColorProfileMsg` 发给 model；官方 profile 枚举包括 NoTTY、ASCII、16 色、256 色和 TrueColor，也支持以 `RGB`/`Tc` termcap 请求升级。不过当前应用没有处理该消息，且 fork renderer 的 profile setter 不工作。因此第一版动画应主动把 profile 当成输入并保守降级，不能只假设 24-bit 色彩。**规范/上游**：[Bubble Tea `profile.go`](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/profile.go)、[`termcap.go`](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/termcap.go)、[Colorprofile `profile.go`](https://github.com/charmbracelet/colorprofile/blob/v0.4.3/profile.go)；**本地**：`vendor/charm.land/bubbletea/v2/tea.go:1088-1096`、`vendor/charm.land/bubbletea/v2/profile.go:5-14`、`vendor/charm.land/bubbletea/v2/termcap.go:7-33`、`vendor/github.com/charmbracelet/colorprofile/profile.go:10-25,57-100`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:204`。
- 已有背景色查询和亮/暗主题更新：应用启动时调用 `lipgloss.HasDarkBackground`，初始化时还请求 `tea.RequestBackgroundColor` 并监听 `BackgroundColorMsg`/DEC 2031。Lip Gloss 官方说明：在 Bubble Tea 中应监听 `BackgroundColorMsg`；独立查询失败时 `HasDarkBackground` 返回 `true`。这适合为动画选取对比色，但不是对「动画是否可接受」或「是否有真彩」的检测。**规范/上游**：[Lip Gloss `query.go`](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/query.go)、[Bubble Tea `color.go`](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/color.go)；**本地**：`model/app.go:52-70,128-144,294-304`、`vendor/charm.land/lipgloss/v2/query.go:28-91`、`vendor/charm.land/bubbletea/v2/color.go:9-74`。

## 2. 可行的动画家族

下表的「可行」指当前的 page/tick/text renderer 能承载；「默认」表示适合作为跨终端、无检测失败灾难的默认体验。

| 家族 | 可行性/默认性 | 可做的效果 | 降级策略与主要限制 |
|---|---|---|---|
| 字符帧 / ASCII | **高 / 是** | 徽标逐列或逐行 reveal、轮廓→实心、2–4 帧极简“呼吸”、小型轨道 spinner | ASCII/NoTTY 只保留静态欢迎文字或一帧；不用 emoji 作关键帧，避免宽度差异。当前 renderer 正是行文本重绘。**规范/上游**：[Bubble Tea Tick](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/commands.go#L116-L164)；**本地**：`model/startup.go:65-77,112-117`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-147`。 |
| 色彩 / gradient | **高 / 是（但 profile-aware）** | 真彩 logo 虹彩/扫光、填充条“彗星头”、主题主色到强调色渐变 | TrueColor 才逐 cell；ANSI256 减少为 4–8 色 band；ANSI 为主题主/辅两色；ASCII 无颜色。官方 Colorprofile 会将低 profile 的 color 转换为空/低色，且 Tea 可报告 profile。**规范/上游**：[Colorprofile](https://github.com/charmbracelet/colorprofile/blob/v0.4.3/profile.go#L10-L25)；**本地**：`vendor/github.com/charmbracelet/colorprofile/profile.go:10-25,57-100`、`vendor/charm.land/bubbletea/v2/profile.go:5-14`。 |
| 运动 / reveal | **高 / 是** | 进度驱动裁切 reveal、以 3–5 个离散 phase 改变 logo、标题/副标题淡入的字符近似 | 不应做“真实 alpha 淡入”（终端文字没有可靠 alpha 合成）；以可读的 phase 替代，并为 reduced-motion 直接显示最终帧。Lip Gloss 可进行块布局和普通 ANSI 文字格式，但这不等于逐像素合成。**规范/上游**：[Lip Gloss Style.Render](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/style.go#L267-L310)；**本地**：`vendor/charm.land/lipgloss/v2/style.go:267-310`、`model/startup.go:90-103`。 |
| 进度变体 | **高 / 是** | 当前 determinate bar 改为圆角/方块/细线、移动高光、阶段标签、无确定进度时 spinner | 保留“加载中”文本和百分比/阶段，不能只用颜色或运动。现有 `ProgressOptions` 已提供 8 个边界/填充字符 seam。**规范/上游**：[Go strings Builder（标准库）](https://pkg.go.dev/strings#Builder)；**本地**：`model/progress.go:10-55`、`model/options.go:66-75`。 |
| Sixel / Kitty / iTerm2 图形 | **有条件 / 否** | 静态 raster logo、像素 shader、短 GIF/帧动画（协议本身相关） | 只有检测成功、专门定位/清除实现和 ASCII fallback 都具备时才可 opt-in。当前 renderer 不管理图像占位；Sixel 的 DA1 属性 `4` 可检测，Kitty/iTerm2 是不同专有协议，不能互相假定。**规范/上游**：[VT510 DA1](https://vt100.net/docs/vt510-rm/DA1.html)、[Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)、[iTerm2 images](https://iterm2.com/documentation-images.html)；**本地**：`vendor/charm.land/bubbletea/v2/raw.go:9-36`、`vendor/github.com/charmbracelet/x/ansi/ctrl.go:31-80`、`vendor/github.com/charmbracelet/x/ansi/graphics.go:9-61`、`vendor/github.com/charmbracelet/x/ansi/iterm2.go:5-17`。 |

### 2.1 字符帧/ASCII：推荐的主路径

可将现有 `GetAlphaAscii` 的结果预切为行，再按 `phase` 输出：

1. **0–15%**：静态、短的应用名（避免刚打开就是空页）；
2. **15–55%**：按列或按字形 reveal；每次显示完整 cell，不能切半个宽字符；
3. **55–90%**：完整 logo，低频（例如 8–12 fps）单条扫光或 2 帧边缘闪烁；
4. **90–100%**：稳定最终 logo 与“已就绪”，然后切 Main。

逐列 reveal 的视觉收益高于逐字符替换，但成本仍是字符串切片；不要在 2 秒内播放 125 张大 ASCII 图。现有 renderer 会每次为改动行写满到窗口宽，故帧越少、改变的行越少越稳。此结论来自 renderer 的逐行截断/补空格算法，而非对终端性能作未经测量的承诺。**规范/上游**：[fork renderer](https://github.com/go-musicfox/bubbletea/blob/v2.0.8-foxful-1.2/foxful_renderer.go)；**本地**：`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-147`、`util/alpha_ascii.go:545-570`。

### 2.2 色彩/gradient：推荐，但不能把“漂亮”绑定到真彩

当前实现已经以随机 RGB 起止色、每列一个 `color.Color` 构造 ramp，说明真彩渐变已经在使用。建议将随机改为 `Theme.Primary → Theme.Accent`（或显式 animation palette），以保证亮/暗背景有设计过的对比；`ColorProfileMsg` 到达前先按 ANSI 安全 palette 渲染，之后允许升级。Bubble Tea 官方还警告某些 terminal（特别举了 Terminal.app）对 `RGB`/`Tc` termcap 请求会给出错误回应并破坏输出，所以**不要为了启动动画强行发 capability probe**；优先使用初始检测结果，或只让用户显式启用升级探测。**规范/上游**：[Bubble Tea termcap 警告](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/termcap.go#L7-L33)、[Lip Gloss 色彩 profiles](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/README.md#colors)；**本地**：`model/startup.go:17-29,134-143`、`vendor/charm.land/bubbletea/v2/termcap.go:7-33`。

### 2.3 运动/reveal：用“进度驱动状态”，不用时间盲跑

当前 `loadedDuration += TickDuration`，不读取 tick 实际发生时刻。因此 event loop 被阻塞时，墙钟上的启动时间会变长，动画也不会追赶；而 `tea.Tick` 的回调确实提供 `time.Time`。建议新状态保留 `startedAt time.Time`，用 `elapsed := now.Sub(startedAt)` clamp 到 `[0, duration]`，将同一个 `progress` 同时驱动 reveal、bar 和文案。这样 resize、慢终端或 GC 延迟不会把“2 秒”人为拉长。**规范/上游**：[Bubble Tea `Tick` 回调签名与一次性语义](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/commands.go#L116-L164)；**本地**：`model/startup.go:65-77`、`vendor/charm.land/bubbletea/v2/commands.go:154-163`。

推荐 animation tick 为 **10–15 fps**（66–100 ms），而不是默认 16 ms。此数值是设计建议，不是协议限制：目的是让 2–4 行的字符动画仍连贯，同时直接降低该 renderer 的全行写入次数。进度计算可保持更精细但只在可见 phase 改变时渲染。

### 2.4 进度变体：应从“装饰”升级为“含义”

建议把进度抽象成两种 source：

- **Determinate**：已知启动任务的 `0..1`；显示 `Loading 62%` 或阶段名；
- **Indeterminate**：不知道耗时；保留稳定“正在初始化”文本，bar 内有一个小彗星头或 spinner，不伪造百分比。

现状是纯时间模拟的 determinate bar，倒计时文字为 `LoadingDuration-loadedDuration`。如果启动页只是品牌过场，可称“即将进入”；如果未来绑定真实初始化，应由任务报告进度，否则不要向用户暗示真实加载完成度。**规范/上游**：[Bubble Tea `Batch`（并发命令无顺序保证）](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/commands.go#L8-L28)；**本地**：`model/startup.go:65-77,120-143`、`model/progress.go:21-55`。

## 3. 终端图形：技术上可探测，当前仍不适合默认启用

### 3.1 已有的库能力

- Bubble Tea 提供 `tea.Raw` 直接输出原始控制序列；官方示例正是请求 DA1，并以属性 `4` 识别 Sixel。输入转换会把 `uv.PrimaryDeviceAttributesEvent` 以未包装事件继续交给 model，因此可以在应用层处理它。**规范/上游**：[Bubble Tea `raw.go`](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/raw.go)、[`input.go`](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/input.go)、[VT510 DA1](https://vt100.net/docs/vt510-rm/DA1.html)；**本地**：`vendor/charm.land/bubbletea/v2/raw.go:9-36`、`vendor/charm.land/bubbletea/v2/input.go:7-53`、`vendor/github.com/charmbracelet/ultraviolet/event.go:405-455`。
- vendored `x/ansi` 可封装 Sixel、Kitty APC graphics 和 iTerm2 OSC 1337；`x/ansi/kitty.EncodeGraphics` 能编码并可分块写出 Kitty 图片。它们是**协议编码工具**，不是统一的图片布局组件或跨 terminal capability matrix。**规范/上游**：[Charm `x/ansi` graphics 源码](https://github.com/charmbracelet/x/blob/ansi/v0.11.7/ansi/graphics.go)、[Kitty protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)、[iTerm2 image docs](https://iterm2.com/documentation-images.html)；**本地**：`vendor/github.com/charmbracelet/x/ansi/graphics.go:9-61`、`vendor/github.com/charmbracelet/x/ansi/iterm2.go:5-17`、`vendor/github.com/charmbracelet/x/ansi/kitty/writer.go:27-160`。

### 3.2 为什么“检测 + graceful fallback”是准入门槛

Sixel 的 DA1 属性 4 有官方 VT510 定义；但 Kitty 与 iTerm2 采用不同协议，不能由 Sixel DA1 推断。对于任一专有协议，必须按照该协议的官方 query/response 定义单独探测，设超时，在**没有肯定回应**时选择文本 fallback，且不能仅以 `$TERM`、`TERM_PROGRAM` 或终端名称猜测。DA1 检测本身应先完成并落入稳定文本帧后再展示图片，防止无回应时卡住启动页。**规范/上游**：[VT510 DA1](https://vt100.net/docs/vt510-rm/DA1.html)、[Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)、[iTerm2 escape codes](https://iterm2.com/documentation-escape-codes.html)；**本地**：`vendor/github.com/charmbracelet/x/ansi/ctrl.go:31-80`、`vendor/charm.land/bubbletea/v2/raw.go:16-31`。

即使探测成功，仍有当前实现特有的 blocker：foxful renderer 不知道图片占多少行/列，会对**文本**按 newline 计行、截断与补空格，并在下一帧重画；`tea.Raw` 是独立直接写入，可能与 renderer 的写入次序/清屏发生竞争。图像会残留、错位或被空间填充覆盖的风险不能由 capability detection 消除。**规范/上游**：[Bubble Tea Raw](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/raw.go)、[fork renderer](https://github.com/go-musicfox/bubbletea/blob/v2.0.8-foxful-1.2/foxful_renderer.go)；**本地**：`vendor/charm.land/bubbletea/v2/tea.go:843-875`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-166`。

**结论**：不在默认启动页使用 Sixel/Kitty/iTerm2 raster 或动画。只有下列全部完成后，才提供 `GraphicsAuto`（默认为关闭）的实验选项：

1. 按协议的正向响应完成 capability handshake 和短超时；
2. 有图片定位、删除/清屏、resize 和退出时清理的专用 renderer seam，而不是 `tea.Raw` 插入；
3. 文本 fallback 在视觉和功能上完整；
4. 对 multiplexers/远程会话/录屏/CI 的非 TTY 行为有测试；
5. 图片只用作静态 logo（先不要逐帧 raster 动画）。

## 4. 建议的实现 seam / API 选择

以下是提议，不是对现有 API 的描述；均可先在测试中实现而不改变 Main/Page 合约。

### 4.1 推荐 API：策略对象（优先）

将 `StartupOptions` 从散落的布尔值扩为可组合策略，并保持现有字段作兼容映射：

```go
// model/startup_animation.go（建议的新文件）
type AnimationMode uint8
const (
    AnimationAuto AnimationMode = iota // profile/reduced-motion 选择
    AnimationFull
    AnimationReduced
    AnimationOff
)

type StartupAnimation struct {
    Mode       AnimationMode
    FrameRate  int           // 0 => 12; clamp 1..15
    Logo       LogoAnimator  // 纯函数，便于 snapshot test
    Progress   ProgressAnimator
    Palette    AnimationPalette
    Graphics   GraphicsMode  // Off 默认；Auto 仍须正向 capability
}

type StartupFrame struct {
    Logo, Status, Progress string
    Done bool
}

type LogoAnimator interface {
    Frame(StartupState, AnimationCapabilities) string
}
```

`StartupPage` 保有 `startedAt`、`lastFrame`、`capabilities` 和 `animation`；`Update` 只更新状态/调度 tick，`View` 只调用 `Frame` 并负责现有居中 layout。这把时间、能力、视觉策略从 `logoView/tipsView/progressView` 中拆开，保留 `StartupPage` 的 `Page` 接口和 `App` 的切页机制。现有 seam 的依据是：`StartupOptions` 已嵌入 `Options`，`StartupPage` 持有其指针，且三个局部 view 已分开。**规范/上游**：[Bubble Tea Model/Update 命令循环](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/commands.go#L116-L164)；**本地**：`model/options.go:11-55`、`model/startup.go:33-46,84-143`、`model/page.go:1-31`。

`AnimationCapabilities` 应只含本次实际获得的能力，不读全局：

```go
type AnimationCapabilities struct {
    Color colorprofile.Profile
    DarkBackground bool
    ReducedMotion bool
    Graphics GraphicsCapability // None/Sixel/Kitty/ITerm2；默认 None
}
```

在 `StartupPage.Update` 处理 `tea.ColorProfileMsg`，用其 profile 选择 palette；现有 App 已处理背景消息，所以可由 App 暴露只读的 theme/background 信息，或将它作为 frame 输入。不要把 profile 探测塞进 `style` 全局。**规范/上游**：[Bubble Tea `ColorProfileMsg`](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/profile.go)；**本地**：`vendor/charm.land/bubbletea/v2/profile.go:5-14`、`model/app.go:128-144`、`style/theme.go:875-894`。

### 4.2 较小 API：渲染回调（可接受，但约束更弱）

若维护者希望最小 public surface，可提供：

```go
type StartupRenderer func(StartupState, AnimationCapabilities) StartupFrame
// StartupOptions.Renderer nil 时使用 DefaultStartupRenderer.
```

优点是下游可完全自定义品牌动画；缺点是调用方要负责宽度、ANSI、fallback 与可访问性，难以保证一致性。因此默认 renderer 仍应作为库维护的、测试覆盖的实现；callback 只适合有终端经验的应用。该方案可利用既有 `InitPage` 和 `TeaOptions` 注入点，但不应以 `tea.Raw` callback 作为普通扩展机制。**规范/上游**：[Bubble Tea `Raw` 的“advanced use cases”警告](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/raw.go#L9-L14)；**本地**：`model/options.go:28-30`、`vendor/charm.land/bubbletea/v2/raw.go:9-14`。

### 4.3 进度 API：把“值”与“皮肤”分开

保留 `ProgressOptions` 的 rune 边界字符兼容性，但新增：

```go
type ProgressState struct { Value float64; Indeterminate bool; Phase int; Label string }
type ProgressAnimator interface { Render(ProgressState, width int, cap AnimationCapabilities) string }
```

这避免让 `Progress` 的 `fullSize` 偷渡动画语义，也使 spinner/comet 不必假装有完成比例。测试可对 `width=0/1/窄屏`、`Value<0/>1`、每种 profile 和 reduced-motion 做快照。当前 `Progress` 对宽度和已满数量的字符串拼装是自然的兼容层。**规范/上游**：[Lip Gloss 宽度/布局 API](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/README.md#width-and-height)；**本地**：`model/progress.go:10-55`、`model/startup.go:134-143`。

## 5. 优先级与交付切片

### P0 — 安全的视觉升级（推荐先做）

1. **增加 `AnimationOff/Reduced/Full/Auto`，默认 Auto；实现 CLI/config 环境的显式关闭。**Reduced/Off 在首帧直接显示完整 logo、稳定状态文本和静态 bar；不要依据颜色 profile 猜测用户是否怕动效。现有 `EnableStartup` 可以跳过整页，但不能表达“保留欢迎页、取消运动”。**规范/上游**：[仓库 options](https://github.com/anhoder/foxful-cli/blob/master/model/options.go)；**本地**：`model/options.go:49-64`。
2. **改为墙钟 elapsed + 10–15 fps，预计算 logo 行/帧。**保留 2 秒默认但让停顿不会延长逻辑时间；仅在 frame 内容变化时请求下一帧。**规范/上游**：[Bubble Tea Tick](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/commands.go#L116-L164)；**本地**：`model/startup.go:65-77`。
3. **实现 4 phase 字符 reveal + 静态最终帧。**只改变 logo 和 progress 两个区域，不做全屏噪点；使用现有 `layout.Place` 和动态 `WindowSizeMsg`。**规范/上游**：[Lip Gloss layout](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/README.md)；**本地**：`model/startup.go:84-117`。
4. **profile-aware palette。**先将 `ColorProfileMsg` 保存，ANSI/ASCII 退化为高对比单色/无色；不要在启动时自动 `RequestCapability("RGB")`。**规范/上游**：[Bubble Tea termcap](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/termcap.go#L7-L33)；**本地**：`vendor/charm.land/bubbletea/v2/termcap.go:7-33`。

### P1 — 信息与质感（P0 验收后）

5. **语义进度 skin：**determinate 时有 label/百分比；indeterminate 时用低频 comet 或 ASCII spinner；全都在 reduced-motion 下静止。复用 `ProgressOptions`，不必引 Bubbles。**本地**：`model/progress.go:10-55`、`vendor/modules.txt:1-11`。
6. **真彩 sweep：**只有 `TrueColor` 下使用 per-cell gradient；ANSI256 用短 palette，ANSI 用主/辅色。色彩随当前亮/暗主题重新计算，避免浅底低对比。**规范/上游**：[Colorprofile profile/Convert](https://github.com/charmbracelet/colorprofile/blob/v0.4.3/profile.go#L10-L25)；**本地**：`vendor/github.com/charmbracelet/colorprofile/profile.go:10-25,57-100`、`model/app.go:286-304`。
7. **测试和基准：**对固定 width、profile、mode、elapsed 进行 golden/snapshot；用 renderer 输出字节数或 `View` 计数做回归阈值。特别覆盖 `< logo width`、高度不足、resize 中途、`LoadingDuration<=0` 和 `Welcome` 含未支持字符的情况。字库静默跳过未知 rune 是现有行为。**本地**：`util/alpha_ascii.go:545-570`、`model/startup.go:84-143`。

### P2 — 图形实验（不进入默认）

8. 在独立 `graphics` adapter 内实现 Sixel DA1 probe、超时、静态图片清除/resize；先只支持 **Sixel + static logo**，因为已有 Bubble Tea 官方示例路径。此项仍需验证 foxful renderer 是否替换或扩展，成功 probe 不是充分条件。**规范/上游**：[Bubble Tea Raw 的 Sixel 示例](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/raw.go#L16-L31)；**本地**：`vendor/charm.land/bubbletea/v2/raw.go:16-31`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-166`。
9. 后续才为 Kitty/iTerm2 分别加入协议 adapter；不得把三者合并为一个“terminal graphics supported”布尔值。每个 adapter 都必须有 `None` fallback、清理和端到端测试。**规范/上游**：[Kitty](https://sw.kovidgoyal.net/kitty/graphics-protocol/)、[iTerm2](https://iterm2.com/documentation-images.html)；**本地**：`vendor/github.com/charmbracelet/x/ansi/graphics.go:46-61`、`vendor/github.com/charmbracelet/x/ansi/iterm2.go:5-17`。

## 6. 风险登记与防护

| 风险 | 证据 | 防护 |
|---|---|---|
| **可访问性：运动不适、认知干扰** | 当前只有 `EnableStartup`，没有“保留内容但停动画”的状态；现有 16 ms cadence 会产生高频变化。**本地**：`model/options.go:49-64`、`model/startup.go:72-77`。 | 显式 `AnimationOff/Reduced`，接受 CLI/config/env；reduced 首帧即最终内容；`q`/Ctrl-C 始终可退出（当前全局 quit 已处理）。**本地**：`model/app.go:119-127`。 |
| **色彩/对比与色盲可用性** | 现有 startup 用随机 RGB；低色和 ASCII profile 客观存在。**规范/上游**：[Colorprofile](https://github.com/charmbracelet/colorprofile/blob/v0.4.3/profile.go#L10-L25)；**本地**：`model/startup.go:24-29`、`vendor/github.com/charmbracelet/colorprofile/profile.go:10-25`。 | 不以颜色单独传达完成/错误；稳定文字 label；使用主题语义色与 profile mapping，ASCII 仍可读。 |
| **性能/闪烁/带宽** | fork renderer 按文本行写满 width；16 ms 等于高频全行输出。**规范/上游**：[fork renderer](https://github.com/go-musicfox/bubbletea/blob/v2.0.8-foxful-1.2/foxful_renderer.go)；**本地**：`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-166`。 | 10–15 fps、frame 去重、预计算、减少改变行；不要全屏粒子或逐帧大图。 |
| **时间准确性** | 当前累加请求的 tick duration，而不是回调给出的实际时间。**规范/上游**：[Bubble Tea Tick](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/commands.go#L116-L164)；**本地**：`model/startup.go:72-77`。 | 用 `startedAt`/实际 `time.Time`，clamp、零/负 duration 直接完成。 |
| **窄屏、Unicode 宽度、字库覆盖** | logo 固定 7 行且未知 rune 被跳过；renderer 会按 width truncate。**本地**：`util/alpha_ascii.go:542-570`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:138-143`。 | 宽度不足时回退到普通应用名/小 logo；只对完整 grapheme/cell 做 reveal；不要把 emoji 当布局单位。 |
| **颜色探测不可靠** | 官方 termcap 文档明确警告某些 terminal 的 RGB/Tc 响应会破坏输出；背景查询也可能失败并默认 dark。**规范/上游**：[Bubble Tea termcap](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/termcap.go#L15-L28)、[Lip Gloss query](https://github.com/charmbracelet/lipgloss/blob/v2.0.4/query.go#L69-L91)；**本地**：`vendor/charm.land/bubbletea/v2/termcap.go:15-28`、`vendor/charm.land/lipgloss/v2/query.go:69-91`。 | 保守 palette、用户 override；不自动 probe 来“美化”启动页。 |
| **图片协议残留或错位** | `Raw` 直接写，foxful renderer 文本行重绘；图形协议彼此不同。**规范/上游**：[Bubble Tea Raw](https://github.com/charmbracelet/bubbletea/blob/v2.0.8/raw.go)、[Kitty protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)；**本地**：`vendor/charm.land/bubbletea/v2/tea.go:843-875`、`vendor/charm.land/bubbletea/v2/foxful_renderer.go:100-166`。 | 默认禁用；专用 adapter/renderer、正向检测、超时、清理与完整文字 fallback。 |

## 最终建议

**现在就做 P0：字符 reveal + 低频、profile-aware 色带 + reduced-motion/off。**它充分利用现有 `StartupPage`、`tea.Tick`、`ProgressOptions`、Lip Gloss 布局和备用屏幕，不增加依赖，也不会把体验押在某个 terminal 的私有图片协议上。随后用 P1 把进度语义和主题 palette 做完整。只有在替换/扩展 foxful renderer 以管理图片单元之后，才考虑 P2 的 opt-in 静态图形 logo；不要把终端图像动画当作“更炫”的默认启动方案。
