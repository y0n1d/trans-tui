[English](README.md) | [简体中文](README.zh-CN.md)

# trans-tui

轻量级终端翻译工具，支持交互式 TUI、stdin 管道和可选的 OCR 集成。

## 功能特性

- **终端优先** — 在任何终端模拟器中运行（foot、kitty、alacritty 等）
- **Bubble Tea TUI** — 交互式界面，支持鼠标选择和回滚历史
- **单实例** — 多次调用通过 Unix socket IPC 共享同一个 TUI 窗口
- **stdin 管道** — `echo "text" | trans-tui`，方便脚本和管道使用
- **交互式输入** — `trans-tui -i` 在 TUI 中打开输入面板
- **鼠标选择** — 在 TUI 中选择翻译文本，通过 OSC52 自动复制到剪贴板
- **可插拔 Provider** — 支持 OpenAI 兼容、Google Translate、DeepL、LibreTranslate
- **自动语言检测** — 中文输入翻译为英文，其他语言翻译为中文
- **OCR 伴侣 CLI** — `trans-ocr` 从图片中提取文本，通过管道传入 `trans-tui`

## 安装

```bash
git clone https://github.com/y0n1d/trans-tui.git
cd trans-tui
go build -o trans-tui ./cmd/trans-tui
go build -o trans-ocr ./cmd/trans-ocr
```

或通过 `go install` 安装：

```bash
go install github.com/y0n1d/trans-tui/cmd/trans-tui@latest
go install github.com/y0n1d/trans-tui/cmd/trans-ocr@latest
```

## 使用方法

直接翻译文本：

```bash
trans-tui "Hello world"
```

通过 stdin 管道输入：

```bash
echo "Hello world" | trans-tui
cat file.txt | trans-tui
```

打开交互式输入模式：

```bash
trans-tui -i
```

OCR 管道 — 从图片中提取文本并翻译：

```bash
trans-ocr screenshot.png | trans-tui
cat image.png | trans-ocr - | trans-tui
```

使用自定义配置文件：

```bash
trans-tui -c /path/to/config.toml "Hello world"
```

## OCR

`trans-ocr` 是一个独立的 OCR 命令行工具，**不是** TUI 的内置 OCR 模式。

当前支持的 OCR Provider：

- **百度 OCR** — 需要设置 `BAIDU_OCR_API_KEY` 和 `BAIDU_OCR_SECRET_KEY` 环境变量

## 配置

默认配置文件位置：`$XDG_CONFIG_HOME/trans-tui/config.toml`
（通常为 `~/.config/trans-tui/config.toml`）

首次运行时会自动创建默认配置文件。可通过 `-c PATH` 指定自定义路径。

最小配置示例：

```toml
[provider]
type = "openai-compatible"
api_key_env = "OPENAI_API_KEY"
timeout = 30
proxy = true

[provider.openai]
base_url = "https://api.openai.com/v1"
model = "gpt-4o-mini"

[translation]
source_lang = "auto"
target_lang = "auto"
```

API key 从环境变量读取 — 切勿在配置文件中存储真实的 key。

#### `proxy`

`[provider].proxy` 决定 API 请求如何访问网络：

- `proxy = true`（默认）—— 允许使用环境代理：`HTTP_PROXY`、`HTTPS_PROXY`
  和 `NO_PROXY` 由 Go 标准库解释。如果环境里没有设置任何代理变量，请求
  仍然是直连。因此 `true` **不等于**"强制经过代理"，它的含义是"存在环境
  代理时就使用它"。
- `proxy = false` —— 始终直连，忽略 `HTTP_PROXY` / `HTTPS_PROXY`。

该选项出现之前写的旧配置文件行为完全不变，因为默认值就是 `true`。由于它
会改变请求的网络出口，因此参与配置 fingerprint：`proxy` 取值不同的客户端
不会附着到已运行的 server 上。

### 按键绑定

默认 TUI 按键绑定：

| 功能 | 默认按键 |
|------|---------|
| 退出 | `q`、`ctrl+c`、`esc` |
| 手动输入 | `,`（英文逗号）、`，`（中文全角逗号） |
| 上/下滚动 | `up` / `down`、`k` / `j` |
| 上/下翻页 | `pgup` / `pgdown`、`b` / `f` |
| 跳到顶部/底部 | `home` / `end`、`g` / `G` |
| 上一条/下一条记录 | `h` / `l` |
| 重试 | `r` |
| 关闭错误提示 | `esc` |
| 取消输入 | `esc` |
| 提交输入 | `enter` |
| 复制选区 | `ctrl+shift+c` |

自定义按键绑定（未写的动作保持默认值）：

```toml
[keybindings]
quit = ["q", "ctrl+c", "esc"]
manual_input = [",", "，"]
```

每个动作至少需要一个按键，为空会报配置错误。不同动作的按键允许重复；TUI 按固定顺序处理（关闭错误 > 退出 > 手动输入 > 滚动 > 导航 > 重试；输入模式下复制 > 取消 > 提交 > 打字）。

### 界面外观

可选的 `[appearance]` 段控制颜色以及是否显示各个区域；省略时保持默认外观：

```toml
[appearance]
background = "235"                 # 整个窗口的底色：ANSI 调色板编号或十六进制（"#1e1e1e"）
transparent_background = false     # true = 不绘制 TUI 自身背景色

[appearance.header]
enabled = true                   # false = 隐藏标题行

[appearance.status_bar]
enabled = true                   # false = 隐藏信息栏
```

背景行为说明：

- `transparent_background = false`（默认）：TUI 会用 `background` 填满窗口内的每一个单元格——包括边框、标题行和空白区域——整个界面是不透明的，终端背后的内容不会透出来（在 foot 开启 `alpha-mode = all` 时尤其重要，未绘制的单元格会透出桌面）。各组件自身的背景色（状态栏、卡片、面板）和选中高亮优先于底色填充。与旧版本相比这是一个有意的默认视觉变化：以前没有自身背景色的单元格会显示终端背景，现在统一填为 `background`。
- `background` 接受 ANSI 调色板编号（`"235"`）或十六进制颜色（`"#1e1e1e"`），解析方式与其他外观颜色一致。它不会自动匹配你的终端背景色——请按自己的终端配色设置（运行时不会变化；该值参与 IPC 配置指纹，配置不一致的新调用会被拒绝，而不是把文本发给正在运行的服务端）。为空或无法解析时禁用底色填充（组件与选中高亮的颜色仍然生效）。
- `transparent_background = true`：TUI 不绘制任何表面或组件背景（仅保留选中高亮），终端/foot 的背景直接透出。实际透明程度受终端本身限制（foot 的 `alpha`/`alpha-mode` 设置）；TUI 从不读写终端背景（不发 OSC 11）。

各子段（`header`、`status_bar`、`record`、`source`、`translation`、`error`、`error_panel`、`loading`、`input`、`selection`）还接受 `configs/example.toml` 中列出的颜色与显示字段。外观与按键绑定参与 IPC 配置指纹；翻译设置不影响指纹。

## Provider

| 类型 | 环境变量 | 说明 |
|------|---------|------|
| `openai-compatible` | `OPENAI_API_KEY` | 默认。支持 OpenAI、DeepSeek、SiliconFlow 等 |
| `google` | `GOOGLE_TRANSLATE_API_KEY` | Google Cloud Translation API |
| `deepl` | `DEEPL_API_KEY` | 免费版：`api-free.deepl.com`，专业版：`api.deepl.com` |
| `libretranslate` | （可选） | 自托管。在 `[provider.libretranslate]` 中设置 `base_url` |

## 架构

```
trans-tui
├── CLI (cmd/trans-tui)         — 参数解析、stdin 检测
├── runtime                     — 单实例编排、IPC 服务器生命周期
├── IPC (internal/ipc)          — Unix socket 协议（长度前缀 JSON）
├── core (internal/core)        — 服务层、语言检测、状态管理
├── translator providers        — 可插拔翻译后端
└── TUI (ui/tui)                — Bubble Tea 界面

trans-ocr (cmd/trans-ocr)       — 独立 OCR CLI
└── OCR providers (internal/ocr)
```

核心设计原则：

- **按需启动** — 无后台守护进程。TUI 进程在首次调用时启动，关闭时退出。
- **管道友好** — stdin/stdout 天然兼容 shell 管道。
- **解耦** — OCR 和翻译相互独立。`trans-ocr` 输出文本，`trans-tui` 负责翻译。

## 许可证

尚未指定。
