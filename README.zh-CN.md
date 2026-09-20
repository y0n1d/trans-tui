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

[provider.openai]
base_url = "https://api.openai.com/v1"
model = "gpt-4o-mini"

[translation]
source_lang = "auto"
target_lang = "auto"
```

API key 从环境变量读取 — 切勿在配置文件中存储真实的 key。

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
