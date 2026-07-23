# 启动动画

`model.StartupOptions.Animation` 可以选择纯文本/ANSI 启动动画。所有模式都不依赖 Kitty、Sixel 或 iTerm2 图片协议；无法使用颜色、启用 `ReducedMotion` 时会自动显示静态最终 Logo 和可读进度。

默认动画是 `model.StartupAnimationSequence`，一个多阶段游戏/IDE 风格启动序列：打字机 → 淡入 → 彩虹扫光 → 短暂故障切换，并显示 `Loading runtime`、`Building interface`、`Syncing colors`、`Ready` 等阶段。

```go
opts := model.DefaultOptions()
opts.Welcome = "FOXFUL"
opts.Animation = model.StartupAnimationParticleBurst
opts.LoadingDuration = 3 * time.Second

if userPrefersReducedMotion {
    opts.ReducedMotion = true
}

app := model.NewApp(opts)
```

## 示例程序

仓库提供一个可以从命令行切换模式的示例：

```bash
go run ./example/startup_animation -list
go run ./example/startup_animation -animation sequence
go run ./example/startup_animation -animation matrix-rain -duration 4s
go run ./example/startup_animation -animation particle-burst -reduced-motion
```

启动页结束后示例会进入一个普通菜单，便于确认动画正确地交还应用控制权。

## 可用模式

| 常量 | 效果 | 成本 |
| --- | --- | --- |
| `StartupAnimationFadeIn` | 使用抖动和由暗到亮的颜色模拟淡入 | P0 |
| `StartupAnimationRainbowWave` | 随帧移动的彩虹 Hue 波 | P0 |
| `StartupAnimationTypewriter` | 欢迎词逐字推进；每个 ASCII 大字母在自身内部由左至右逐列展开，且 Logo 保持居中 | P0 |
| `StartupAnimationSpinner` | 自定义 `◐ ◓ ◑ ◒` spinner；ASCII 环境使用 `| / - \` | P0 |
| `StartupAnimationSlideIn` | 带弹性缓动的右侧滑入 | P1 |
| `StartupAnimationGlitch` | 伪随机字符替换和青/洋红/黄颜色分离 | P1 |
| `StartupAnimationSequence` | 默认多阶段启动序列 | P1 |
| `StartupAnimationMatrixRain` | Logo 后方的 Matrix 风格字符雨 | P2 |
| `StartupAnimationParticleBurst` | 彩色粒子向 Logo 聚合 | P2 |

## 运行特性

- 默认 tick 为 **50ms（20 FPS）**，避免当前逐行 renderer 在 SSH 等慢链路中高频重写整屏。
- Logo 超过可用窗口宽高时自动回退为一行欢迎文字，避免 renderer 截断字模。
- `MatrixRain` 和 `ParticleBurst` 每帧重绘整个 viewport，属于显式 opt-in 的 P2 效果；不建议用于长时间加载页。
- 所有时间动画都有固定结束点。`ReducedMotion` 会关闭彩虹、故障、粒子和 spinner，并使用线性进度，适合无障碍和自动化环境。

## 兼容性

颜色效果会根据 `colorprofile` 自动降级；`ASCII`/非 TTY 终端只获得无 ANSI 色的静态最终帧。进度值、阶段文字和填充字符不依赖颜色，因此不会仅通过色相传递状态。
