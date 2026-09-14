# my-trans

## 轻量级 Wayland / Niri 终端翻译工具

**文档版本：V1.0**

---

# 1. 项目概述

开发一个面向 Linux 桌面环境、尤其是 **Wayland + Niri + Arch Linux** 的轻量级终端翻译工具。

项目暂定名称：

```text
my-trans
```

核心目标不是重新实现 OCR、翻译模型、TTS 或 Wayland 桌面功能，而是将已有的 Linux 原生工具、云端 OCR、翻译 API、TTS 引擎组合成一个统一的翻译工作流。

核心理念：

> **按需启动、用完退出、没有无意义的后台常驻；使用一个固定的 foot 翻译窗口承载 TUI，后续翻译请求通过 Unix Domain Socket 更新同一个窗口，并将翻译结果持续追加到历史记录。**

---

# 2. 核心目标

项目必须满足：

- Arch Linux 优先
- Wayland 原生
- Niri 优先
- foot 作为翻译窗口
- Go 实现
- Bubble Tea 实现 TUI
- Unix Domain Socket 实现本地 IPC
- 默认不常驻后台
- TUI 存在期间才运行 Server
- TUI 关闭后 Server 自动退出
- 支持文本翻译
- 支持 stdin
- 支持剪贴板
- 支持 Wayland 选区
- 支持截图
- 支持 OCR
- 支持 TTS
- 支持多个 Translation Provider
- 支持多个 OCR Provider
- Provider 与核心业务解耦
- UI 与核心业务解耦
- 翻译结果默认**追加**
- 支持浏览历史翻译
- 支持鼠标滚轮
- 支持键盘上下翻页
- 支持复制
- 支持重新翻译
- API Key 使用**环境变量**
- 不把 API Key 写入配置文件
- 不创建 systemd 常驻服务
- 不创建开机启动 daemon
- 不加载常驻本地 OCR/LLM/TTS 模型

---

# 3. 最终使用体验

用户通过 Niri 快捷键调用：

```bash
my-trans --selection
```

第一次执行：

```text
Niri
 ↓
my-trans CLI
 ↓
检测 Unix Socket
 ↓
不存在
 ↓
启动 Server
 ↓
启动 foot
 ↓
显示翻译结果
```

第二次执行：

```bash
my-trans --selection
```

程序发现：

```text
$XDG_RUNTIME_DIR/my-trans.sock
```

已经存在。

于是：

```text
CLI
 ↓
Unix Socket
 ↓
已有 Server
 ↓
新增 TranslationRecord
 ↓
TUI 更新
```

**绝对不能创建第二个 foot 窗口。**

第三次、第四次同样如此。

最终窗口中形成翻译历史：

```text
┌────────────────────────────────────────────┐
│ Translator                           DeepSeek │
├────────────────────────────────────────────┤
│                                            │
│ ① Hello world                              │
│                                            │
│   你好，世界                                │
│                                            │
│ ────────────────────────────────────────── │
│                                            │
│ ② How are you doing?                       │
│                                            │
│   你最近怎么样？                            │
│                                            │
│ ────────────────────────────────────────── │
│                                            │
│ ③ The derivative exists at x = 0.          │
│                                            │
│   函数在 x = 0 处存在导数。                 │
│                                            │
├────────────────────────────────────────────┤
│ 3 records │ ↑↓ Scroll │ Copy │ TTS │ Retry │
└────────────────────────────────────────────┘
```

---

# 4. 翻译记录必须采用追加模式

这是核心需求之一。

每一次成功翻译都产生一个新的：

```go
TranslationRecord
```

例如：

```go
type TranslationRecord struct {
    ID          string
    Source      string
    Translation string

    SourceLang  string
    TargetLang  string

    Provider    string
    Model       string

    CreatedAt   time.Time
}
```

程序状态：

```go
type AppState struct {
    Records      []TranslationRecord

    Cursor       int
    ScrollOffset int

    Loading      bool
    Error        error
}
```

---

# 5. 新翻译行为

新请求完成后：

```text
旧记录
+
新 TranslationRecord
```

而不是覆盖。

例如：

第一次：

```text
① Hello

你好
```

第二次：

```text
① Hello

你好

② World

世界
```

第三次：

```text
① Hello

你好

② World

世界

③ Good morning

早上好
```

---

# 6. 自动滚动

每次产生新翻译后：

> **TUI 自动滚动到最新记录。**

例如：

```text
①
②
③
④ ← 当前底部
```

用户再次翻译：

```text
⑤ ← 自动显示
```

这样用户永远可以看到最新结果。

---

# 7. 历史浏览

必须支持：

### 键盘

```text
↑
↓
```

用于上下滚动。

支持：

```text
PageUp
PageDown
Home
End
```

建议：

```text
Ctrl+U
Ctrl+D
```

也可以作为快速滚动快捷键。

---

## 鼠标

Bubble Tea 必须支持鼠标事件。

至少支持：

```text
Mouse Wheel Up
Mouse Wheel Down
```

用户可以直接使用鼠标滚轮查看之前的翻译。

---

# 8. 历史生命周期

第一阶段：

> **历史仅保存在内存中。**

例如：

```text
启动 TUI
 ↓
翻译 30 次
 ↓
内存保存 30 条
```

关闭：

```text
foot
 ↓
Server 退出
 ↓
内存释放
 ↓
历史消失
```

第一版**不实现永久历史数据库**。

原因：

- 保持轻量
- 减少磁盘 IO
- 避免隐私问题
- 避免数据库复杂度
- 符合按需运行理念

未来版本可以增加 SQLite 历史记录。

---

# 9. 普通文本翻译

支持：

```bash
my-trans "Hello world"
```

输出：

```text
你好，世界
```

普通 CLI 模式完成后直接退出。

不会启动长期运行的 Server。

---

# 10. stdin

支持：

```bash
echo "Hello world" | my-trans
```

以及：

```bash
cat file.txt | my-trans
```

程序：

```text
stdin
 ↓
Translation Provider
 ↓
stdout
```

完成后退出。

---

# 11. 剪贴板

支持：

```bash
my-trans --clipboard
```

使用：

```text
wl-paste
```

获取当前 Wayland 剪贴板内容。

流程：

```text
wl-paste
 ↓
获取文本
 ↓
Translation Provider
 ↓
TranslationRecord
 ↓
TUI
```

如果当前没有 TUI：

```text
启动 Server
 ↓
启动 foot
```

如果已经存在：

```text
发送 Unix Socket
```

---

# 12. Wayland 选区翻译

支持：

```bash
my-trans --selection
```

设计目标：

用户在其他应用中选中文字，然后通过 Niri 快捷键触发翻译。

例如：

```kdl
Mod+T {
    spawn "my-trans" "--selection";
}
```

流程：

```text
用户选中文字
       ↓
Wayland selection
       ↓
获取文本
       ↓
Translation Provider
       ↓
TranslationRecord
       ↓
TUI
```

必须使用 Wayland 原生机制。

不要依赖 X11 clipboard hack。

---

# 13. 截图翻译

支持：

```bash
my-trans --screen
```

或者：

```bash
my-trans --screenshot
```

流程：

```text
my-trans
   ↓
slurp
   ↓
用户选择区域
   ↓
grim
   ↓
得到 PNG
   ↓
百度 OCR
   ↓
得到文本
   ↓
Translation Provider
   ↓
TranslationRecord
   ↓
TUI
```

不要自行实现：

```text
Wayland screenshot protocol
```

直接使用：

```text
grim
slurp
```

---

# 14. OCR

第一阶段 OCR Provider：

> **百度高精度 OCR**

程序应该封装：

```go
type OCRProvider interface {
    Recognize(ctx context.Context, image []byte) (string, error)
}
```

百度 OCR 实现：

```text
BaiduOCR
```

核心业务不允许直接依赖百度 API。

正确：

```text
Core
 ↓
OCRProvider
 ↓
BaiduOCR
```

错误：

```text
Core
 ↓
直接调用百度 API
```

---

# 15. OCR Provider 可扩展

未来可以增加：

```text
BaiduGeneralOCR
BaiduFormulaOCR
TesseractOCR
PaddleOCR
VisionOCR
```

例如：

```go
type OCRProvider interface {
    Recognize(ctx context.Context, image []byte) (string, error)
}
```

这样以后更换 OCR 不需要修改截图逻辑。

---

# 16. 数学公式 OCR

需要注意：

> 百度通用高精度 OCR 主要针对普通文字识别。

项目未来可能用于：

```text
高等数学
考研数学
数学公式
```

因此架构必须允许未来加入：

```text
Formula OCR
Vision OCR
```

例如：

```text
截图
 ↓
OCR Provider
 ├── BaiduGeneralOCR
 ├── BaiduFormulaOCR
 └── VisionOCR
```

第一版不强制实现公式 OCR。

---

# 17. 翻译 Provider

必须定义统一接口：

```go
type Translator interface {
    Translate(
        ctx context.Context,
        req TranslationRequest,
    ) (TranslationResult, error)
}
```

例如：

```go
type TranslationRequest struct {
    Text       string
    SourceLang string
    TargetLang string
}
```

---

# 18. 第一阶段翻译 Provider

第一阶段原则：

> **优先选择目前市场上免费可用、响应快速的 API。**

具体 Provider 不应该写死在核心代码中。

最终应该通过配置选择：

```toml
[general]
provider = "xxx"
```

项目第一阶段重点支持：

```text
OpenAI-compatible API
```

原因是大量服务可以共用：

```text
base_url
api_key
model
```

这一套结构。

---

# 19. OpenAI-compatible Provider

配置示例：

```toml
[providers.xxx]
type = "openai-compatible"
base_url = "https://example.com/v1"
model = "example-model"
api_key_env = "MY_TRANS_API_KEY"
```

程序：

```text
配置
 ↓
读取 api_key_env
 ↓
os.Getenv()
 ↓
API
```

API Key 不进入配置文件。

---

# 20. API Key

最终决定：

> **使用环境变量保存 API Key。**

例如：

```bash
export MY_TRANS_API_KEY="..."
```

配置：

```toml
api_key_env = "MY_TRANS_API_KEY"
```

程序通过：

```go
os.Getenv("MY_TRANS_API_KEY")
```

获取。

---

# 21. API Key 安全要求

禁止：

```text
config.toml 保存 API Key
README 保存 API Key
源代码硬编码 API Key
日志输出 API Key
错误信息输出完整 API Key
Git commit 保存 API Key
```

允许：

```text
环境变量
```

例如：

```bash
export DEEPSEEK_API_KEY="..."
```

或者：

```bash
export MY_TRANS_API_KEY="..."
```

---

# 22. 配置文件

配置文件：

```text
~/.config/my-trans/config.toml
```

配置文件只保存：

```text
Provider
URL
Model
语言
UI
行为
```

不保存秘密。

示例：

```toml
[general]
source_lang = "auto"
target_lang = "zh"
provider = "xxx"

[ui]
mouse = true

[providers.xxx]
type = "openai-compatible"
base_url = "https://..."
model = "..."
api_key_env = "MY_TRANS_API_KEY"
```

---

# 23. 自动语言检测

默认：

```text
source_lang = "auto"
target_lang = "zh"
```

例如：

```text
English → Chinese
Japanese → Chinese
Korean → Chinese
```

如果 Provider 支持自动识别：

```text
source = auto
```

优先使用 Provider 自己的能力。

不需要第一版额外实现复杂语言检测模型。

---

# 24. TUI

使用：

```text
Bubble Tea
Lip Gloss
Bubbles
```

技术栈：

```text
Go
+
Bubble Tea
+
Lip Gloss
+
Bubbles
```

---

# 25. TUI 架构

遵循：

```text
Event
 ↓
Update
 ↓
State
 ↓
View
```

例如：

```text
鼠标滚轮
 ↓
Update
 ↓
ScrollOffset
 ↓
View
```

新翻译：

```text
TranslationResult
 ↓
Update
 ↓
Records append
 ↓
Scroll to bottom
 ↓
View
```

---

# 26. Core 与 UI 解耦

Bubble Tea 不能成为整个程序的核心。

正确结构：

```text
Core
 ├── Translation
 ├── OCR
 ├── Screenshot
 ├── Clipboard
 ├── TTS
 ├── Provider
 ├── Config
 └── IPC

UI
 └── Bubble Tea
```

未来可以：

```text
Core
 ├── Bubble Tea
 ├── GTK
 └── Web UI
```

而无需重写：

```text
OCR
Translation
TTS
Provider
Config
IPC
```

---

# 27. foot

foot 是默认翻译窗口。

程序启动：

```text
my-trans Server
 ↓
foot
```

foot 使用专门的 app-id：

```text
my-trans
```

Niri 根据 app-id 匹配窗口。

---

# 28. 窗口位置

程序**不负责决定窗口位置**。

Niri 负责：

```text
floating
position
width
height
focus
```

例如：

```text
右上角
```

可以通过 Niri：

```kdl
window-rule {
    match app-id="my-trans"
    ...
}
```

实现。

程序只负责：

```text
提供正确 app-id
```

---

# 29. 单窗口原则

这是项目的核心功能。

第一次：

```text
my-trans --selection
```

如果 Server 不存在：

```text
Server
+
foot
```

第二次：

```text
my-trans --selection
```

必须：

```text
检测 socket
 ↓
连接已有 Server
 ↓
发送 request
```

不能：

```text
创建第二个 foot
```

也不能：

```text
关闭旧窗口
+
重新打开窗口
```

窗口：

```text
位置不变
大小不变
实例不变
```

只有内容变化。

---

# 30. Unix Domain Socket

IPC 使用：

```text
Unix Domain Socket
```

路径：

```text
$XDG_RUNTIME_DIR/my-trans.sock
```

例如：

```text
/run/user/1000/my-trans.sock
```

Unix Socket：

- 不使用 TCP
- 不经过网络接口
- 不需要端口
- 仅本机 IPC
- 延迟低
- 权限容易控制

Go：

```go
net.Listen("unix", socketPath)
```

---

# 31. IPC 协议

不要直接发送任意字符串。

使用版本化 JSON。

例如：

```json
{
  "version": 1,
  "type": "translate",
  "request_id": "xxx",
  "source": "selection",
  "text": "Hello world",
  "source_lang": "auto",
  "target_lang": "zh",
  "provider": "xxx"
}
```

未来可以支持：

```text
translate
ocr
tts
retry
copy
clear
show
hide
quit
```

---

# 32. Server 生命周期

必须遵守：

> **没有 TUI 就没有 Server。**

普通命令：

```bash
my-trans "hello"
```

流程：

```text
启动
 ↓
API
 ↓
输出
 ↓
退出
```

没有常驻进程。

---

# 33. TUI Server 生命周期

第一次：

```text
my-trans --selection
 ↓
Server 启动
 ↓
foot 启动
```

Server 在 TUI 存在期间运行。

关闭 foot：

```text
foot exit
 ↓
Server 检测
 ↓
停止任务
 ↓
关闭 socket
 ↓
删除 socket
 ↓
Server exit
```

---

# 34. 不使用 systemd

项目默认：

```text
不创建 systemd service
```

不：

```text
开机启动
```

不：

```text
后台永久运行
```

不：

```text
systemctl --user enable my-trans
```

Server 仅作为 TUI 生命周期的一部分存在。

---

# 35. Stale Socket

必须处理异常退出导致的：

```text
my-trans.sock
```

残留。

启动时：

```text
检查 socket
 ↓
尝试连接
```

如果：

```text
连接成功
```

说明 Server 已存在。

如果：

```text
连接失败
```

需要判断是否为 stale socket。

确认不存在有效 Server 后：

```text
删除 stale socket
 ↓
重新创建
```

避免：

```text
socket 文件存在
+
Server 实际不存在
```

导致程序无法启动。

---

# 36. 并发

程序必须支持用户连续操作。

例如：

```text
截图
 ↓
OCR
 ↓
翻译
```

过程中用户又执行：

```text
选区翻译
```

程序不能卡死。

使用：

```go
context.Context
```

以及：

```go
goroutine
```

---

# 37. 默认请求策略

默认：

> **新请求优先。**

例如：

```text
Request 1
正在翻译
```

用户又发起：

```text
Request 2
```

可以：

```text
Cancel Request 1
 ↓
执行 Request 2
```

原因：

翻译窗口通常最关心用户刚刚发起的请求。

未来可以增加：

```toml
cancel_previous = true
```

---

# 38. 网络请求

使用：

```go
net/http
```

必须支持：

```text
timeout
context cancellation
HTTP status checking
错误处理
```

不能：

```text
无限等待
```

---

# 39. Retry

第一版支持基本的：

```text
Retry
```

如果翻译失败：

```text
Error
```

用户可以：

```text
Retry
```

重新执行上一次请求。

第一版不要求复杂的自动重试机制。

---

# 40. TTS

支持：

```bash
my-trans --tts "Hello world"
```

第一阶段：

```text
espeak-ng
```

通过：

```go
os/exec
```

调用。

Provider：

```go
type TTSProvider interface {
    Speak(ctx context.Context, text string, lang string) error
}
```

---

# 41. TTS 生命周期

不要启动：

```text
常驻 TTS daemon
```

正确：

```text
TTS request
 ↓
启动 espeak-ng
 ↓
播放
 ↓
退出
```

---

# 42. Screenshot

截图使用：

```text
grim
```

区域选择：

```text
slurp
```

不要自己实现截图协议。

---

# 43. Clipboard

使用：

```text
wl-paste
wl-copy
```

不要依赖 X11 clipboard 工具。

---

# 44. CLI

建议支持：

```text
my-trans [TEXT]

my-trans --clipboard
my-trans --selection
my-trans --screen
my-trans --screenshot

my-trans --ocr IMAGE

my-trans --tts TEXT

my-trans tui

my-trans providers

my-trans config

my-trans --version
my-trans --help
```

---

# 45. CLI 行为原则

常用操作必须尽可能简单。

例如：

```bash
my-trans --selection
```

完成：

```text
选区
→ OCR/文本获取
→ 翻译
→ TUI
```

用户不需要手动执行：

```text
grim
slurp
OCR
API
```

---

# 46. Niri 快捷键

推荐：

```kdl
Mod+T {
    spawn "my-trans" "--selection";
}

Mod+Shift+T {
    spawn "my-trans" "--screen";
}

Mod+C {
    spawn "my-trans" "--clipboard";
}
```

这里只提供建议。

程序不得硬编码快捷键。

用户可以自由修改。

---

# 47. UI 设计

目标：

> 简洁、轻量、信息密度适中。

建议：

```text
╭────────────────────────────────────────────╮
│ Translator                           DeepSeek │
├────────────────────────────────────────────┤
│                                            │
│ ① The derivative exists at x = 0.         │
│                                            │
│   函数在 x = 0 处存在导数。                 │
│                                            │
│ ────────────────────────────────────────── │
│                                            │
│ ② How are you doing?                       │
│                                            │
│   你最近怎么样？                            │
│                                            │
├────────────────────────────────────────────┤
│ 2 records │ ↑↓ / Wheel │ Copy │ TTS │ Retry │
╰────────────────────────────────────────────╯
```

---

# 48. TUI 必须支持

至少：

```text
↑
↓
PageUp
PageDown
Home
End
鼠标滚轮
复制
Retry
TTS
退出
```

未来可以增加：

```text
搜索
清空历史
删除单条记录
Provider 切换
语言切换
```

---

# 49. 长文本处理

翻译结果可能很长。

必须支持：

```text
自动换行
纵向滚动
```

不能让长文本：

```text
溢出终端
```

中英文混排必须正常。

---

# 50. 数学符号

因为项目未来可能用于数学学习场景，TUI 必须尽可能正确显示：

```text
∫
∑
√
∞
→
≤
≥
α
β
π
∂
```

以及：

```text
x²
f'(x)
lim
```

等常见数学内容。

---

# 51. 字体

程序不要强制安装字体。

只保证：

```text
UTF-8
```

正确处理。

实际字体由 foot / 系统负责。

---

# 52. Provider 配置

推荐：

```toml
[general]
source_lang = "auto"
target_lang = "zh"
provider = "xxx"

[providers.xxx]
type = "openai-compatible"
base_url = "https://..."
model = "..."
api_key_env = "MY_TRANS_API_KEY"
```

---

# 53. Provider 选择

代码不能假设：

```text
DeepSeek = 唯一 Provider
```

必须：

```text
Provider interface
```

以后可以：

```text
DeepSeek
Qwen
Gemini
SiliconFlow
OpenAI
其他兼容 API
```

甚至：

```text
Ollama
```

但本地 Ollama 不作为默认方案。

---

# 54. 免费 Provider 策略

第一阶段选择：

> **当前市场上免费可用、速度较快的翻译 API 作为默认 Provider。**

但是：

**Provider 本身不能写死。**

即使未来：

```text
Provider A
```

免费额度取消，也只需要：

```toml
provider = "B"
```

而无需修改程序。

具体默认 Provider 在正式开发时根据当时的实际免费额度、速度和可访问性确定。

---

# 55. OCR Provider 配置

例如：

```toml
[ocr]
provider = "baidu"

[ocr.baidu]
api_key_env = "BAIDU_OCR_API_KEY"
secret_key_env = "BAIDU_OCR_SECRET_KEY"
```

百度 OAuth / access token 等认证逻辑由：

```text
BaiduOCR Provider
```

内部处理。

核心业务不应该知道百度认证细节。

---

# 56. 百度 OCR Key

与翻译 API 一样：

```text
环境变量
```

例如：

```bash
export BAIDU_OCR_API_KEY="..."
export BAIDU_OCR_SECRET_KEY="..."
```

禁止：

```text
config.toml
源代码
Git
日志
```

保存真实 Key。

---

# 57. 错误处理

错误必须分层。

例如：

```text
选区失败
截图失败
OCR 失败
翻译失败
TTS 失败
IPC 失败
Provider 配置错误
API Key 不存在
网络超时
```

TUI 中应该显示用户能理解的错误。

例如：

```text
✗ Translation failed

API request timed out.

Press R to retry.
```

而不是直接：

```text
panic
```

---

# 58. 日志

默认不产生大量日志。

CLI：

```text
stdout
```

用于正常结果。

```text
stderr
```

用于错误。

Debug：

```bash
my-trans --debug
```

或者：

```bash
MY_TRANS_LOG=debug
```

禁止日志输出：

```text
API Key
Authorization header
完整敏感请求
```

---

# 59. 缓存

第一版可以暂时不实现缓存。

未来可以加入：

```text
相同文本
+
相同源语言
+
相同目标语言
+
相同 Provider
+
相同 Model
```

复用翻译结果。

如果加入缓存：

- 必须可以关闭
- 必须限制大小
- 不保存 API Key
- 不无限增长

推荐未来使用：

```text
SQLite
```

但不是 MVP 必需功能。

---

# 60. 永久历史

第一版：

```text
不实现
```

未来：

```text
SQLite
```

保存：

```text
TranslationRecord
```

可以增加：

```text
搜索
标签
收藏
删除
历史浏览
```

---

# 61. Provider Fallback

未来支持：

```toml
[general]
provider = "primary"
fallback_provider = "secondary"
```

例如：

```text
Primary
 ↓
失败
 ↓
Fallback
```

MVP 不实现复杂自动 fallback。

---

# 62. 性能目标

项目必须尽可能轻量。

没有使用翻译工具时：

```text
my-trans 进程不存在
```

因此：

```text
CPU ≈ 0
RAM ≈ 0
```

---

# 63. TUI 模式资源

TUI 开启时：

```text
my-trans Server
+
Bubble Tea
+
foot
```

不要加载：

```text
OCR 模型
LLM
TTS 模型
```

除非用户实际调用对应功能。

---

# 64. OCR 按需执行

正确：

```text
打开 TUI
 ↓
不加载 OCR
```

用户截图：

```text
slurp
 ↓
grim
 ↓
Baidu OCR
 ↓
结束
```

不要：

```text
启动 TUI
 ↓
后台启动 OCR 服务
```

---

# 65. TTS 按需执行

正确：

```text
用户按 TTS
 ↓
启动 espeak-ng
 ↓
播放
 ↓
退出
```

不要：

```text
启动 TUI
 ↓
启动 TTS daemon
```

---

# 66. 架构总览

最终架构：

```text
                         Niri
                          │
                  Shortcut / CLI
                          │
                          ▼
                     my-trans CLI
                          │
              ┌───────────┴───────────┐
              │                       │
          普通 CLI                 TUI 请求
              │                       │
              ▼                       ▼
        Translation API          Unix Socket
                                      │
                                      ▼
                               my-trans Server
                                      │
                                      ▼
                                     foot
                                      │
                               Bubble Tea TUI
                                      │
                         ┌────────────┼────────────┐
                         │            │            │
                       Input        History       Actions
                         │            │            │
                         └────────────┼────────────┘
                                      │
                                      ▼
                                    Core
                                      │
               ┌──────────────────────┼─────────────────────┐
               │                      │                     │
           Translation               OCR                   TTS
               │                      │                     │
          Provider API           Baidu OCR             espeak-ng
               │
               │
        Free/Fast Provider
```

---

# 67. 推荐项目目录

```text
my-trans/
│
├── cmd/
│   └── my-trans/
│       └── main.go
│
├── internal/
│   │
│   ├── core/
│   │   ├── state.go
│   │   ├── events.go
│   │   └── service.go
│   │
│   ├── translator/
│   │   ├── provider.go
│   │   ├── openai_compatible.go
│   │   └── ...
│   │
│   ├── ocr/
│   │   ├── provider.go
│   │   └── baidu.go
│   │
│   ├── screenshot/
│   │   └── screenshot.go
│   │
│   ├── clipboard/
│   │   └── clipboard.go
│   │
│   ├── tts/
│   │   ├── provider.go
│   │   └── espeak.go
│   │
│   ├── ipc/
│   │   └── unix_socket.go
│   │
│   ├── config/
│   │   └── config.go
│   │
│   └── runtime/
│       └── lifecycle.go
│
├── ui/
│   └── tui/
│       ├── model.go
│       ├── update.go
│       ├── view.go
│       ├── keys.go
│       └── styles.go
│
├── configs/
│   └── example.toml
│
├── docs/
│
├── go.mod
├── go.sum
└── README.md
```

实际开发过程中允许根据代码规模调整目录。

---

# 68. Core 状态原则

Core 不允许 import：

```text
bubbletea
lipgloss
```

Core 只处理：

```text
TranslationRecord
TranslationRequest
TranslationResult
Provider
OCR
TTS
IPC
```

UI 只负责：

```text
展示
输入
滚动
用户操作
```

---

# 69. 事件模型

建议：

```go
type Event interface{}
```

例如：

```text
TranslationStarted
TranslationCompleted
TranslationFailed
OCRStarted
OCRCompleted
OCRFailed
TTSStarted
TTSCompleted
ScrollUp
ScrollDown
Retry
Copy
```

这样 Bubble Tea 只是其中一个事件消费者。

---

# 70. 未来 UI 替换能力

当前：

```text
Core
 ↓
Bubble Tea
 ↓
foot
```

未来允许：

```text
Core
 ↓
GTK
```

或者：

```text
Core
 ↓
Web UI
```

或者：

```text
Core
 ↓
其他 TUI
```

核心业务代码不应该重写。

---

# 71. MVP 第一阶段

第一阶段只实现：

```text
1. Go CLI
2. 文本翻译
3. OpenAI-compatible Provider
4. 环境变量 API Key
5. TOML 配置
6. Bubble Tea TUI
7. foot
8. Unix Socket
9. 单实例
10. Niri Window Rule
11. Wayland selection
12. Clipboard
13. Translation History
14. Mouse Scroll
15. Keyboard Scroll
16. Copy
17. Retry
```

---

# 72. MVP 验收标准

### 测试 1

执行：

```bash
my-trans --selection
```

结果：

```text
foot 出现
```

---

### 测试 2

再次：

```bash
my-trans --selection
```

结果：

```text
不会出现第二个 foot
```

---

### 测试 3

第三次：

```bash
my-trans --selection
```

结果：

```text
同一个窗口
+
新增一条记录
```

---

### 测试 4

使用：

```text
鼠标滚轮
```

结果：

```text
可以查看历史翻译
```

---

### 测试 5

使用：

```text
↑
↓
PageUp
PageDown
Home
End
```

结果：

```text
可以浏览历史
```

---

### 测试 6

关闭 foot：

```text
Server 自动退出
```

然后：

```bash
test ! -e "$XDG_RUNTIME_DIR/my-trans.sock"
```

应该成立。

---

### 测试 7

再次：

```bash
my-trans --selection
```

应该重新：

```text
启动 Server
+
启动 foot
```

---

### 测试 8

截图：

```bash
my-trans --screen
```

应该：

```text
slurp
 ↓
grim
 ↓
百度 OCR
 ↓
翻译
 ↓
追加历史
```

---

# 73. 第二阶段

实现：

```text
截图
grim
slurp
百度 OCR
```

完整流程：

```text
区域选择
 ↓
截图
 ↓
OCR
 ↓
Translation
 ↓
History
 ↓
TUI
```

---

# 74. 第三阶段

加入：

```text
TTS
Provider fallback
更完善 Retry
复制
历史管理
缓存
```

---

# 75. 第四阶段

加入：

```text
Baidu Formula OCR
Vision OCR
更多 Translation Provider
永久历史
SQLite
搜索
收藏
更完善 TUI
```

---

# 76. Anti-goals

第一阶段明确禁止：

```text
Electron
Qt
GTK
systemd service
常驻 daemon
开机启动
常驻 OCR
常驻 LLM
常驻 TTS
自制 OCR
自制 TTS
自制截图协议
自制 Wayland compositor 功能
复杂数据库
```

除非后续需求明确证明必要。

---

# 77. 外部依赖

Linux 系统工具：

```text
foot
grim
slurp
wl-clipboard
espeak-ng
```

核心程序：

```text
Go
Bubble Tea
Lip Gloss
Bubbles
```

网络：

```text
Translation API
Baidu OCR API
```

---

# 78. 安装后的系统状态

安装完成后：

```text
my-trans
```

是一个普通用户程序。

不应该出现：

```text
/etc/systemd/system/my-trans.service
```

也不应该出现：

```text
后台 my-trans daemon
```

---

# 79. 安全原则

必须做到：

```text
API Key → 环境变量
```

禁止：

```text
API Key → Git
API Key → config.toml
API Key → 日志
API Key → TUI
```

TUI 不应该显示完整 API Key。

---

# 80. Unix Socket 安全

Socket：

```text
$XDG_RUNTIME_DIR/my-trans.sock
```

应该使用合理权限。

因为它承担：

```text
CLI → Server
```

通信，所以应该确保普通其他用户不能随意操作当前用户的 Server。

---

# 81. 可维护性

开发时不要过度工程化。

优先：

```text
简单
清晰
可测试
可替换
```

而不是：

```text
抽象层过多
接口过多
复杂依赖注入
复杂框架
```

---

# 82. 开发原则

AI Coding Agent 开始开发时必须：

1. 先阅读完整需求。
2. 不要一次性实现所有阶段。
3. 先完成 MVP。
4. 每个阶段都必须保持可运行。
5. 不允许擅自引入重量级 GUI 框架。
6. 不创建常驻 systemd 服务。
7. 不创建开机启动 daemon。
8. API Key 不得硬编码。
9. API Key 不得进入 TOML。
10. 不把 Provider 写进 TUI。
11. Core 不依赖 Bubble Tea。
12. 所有外部进程调用必须处理错误。
13. 网络请求必须支持 timeout。
14. 网络请求必须支持 context cancellation。
15. Unix Socket 必须处理 stale socket。
16. 必须支持单实例 Server。
17. 第二次翻译不得创建第二个 foot。
18. 新翻译必须追加到历史。
19. 新翻译完成后自动滚动到底部。
20. 必须支持鼠标滚轮浏览历史。
21. 必须支持键盘浏览历史。
22. TUI 关闭后 Server 必须退出。
23. Server 退出后 Socket 必须清理。
24. 第一阶段不实现永久历史。
25. 不为了未来功能提前引入复杂系统。

---

# 83. 最终产品定义

这个项目不是：

> 一个自己重新实现 OCR、翻译、TTS、截图和桌面 GUI 的大型翻译软件。

而是：

> **一个轻量级 Linux / Wayland 翻译工作流编排器。**

它利用：

```text
Linux
Wayland
Niri
foot
grim
slurp
wl-clipboard
Baidu OCR
Translation API
espeak-ng
```

并使用 Go 将这些能力统一起来。

最终提供：

```text
统一 CLI
+
统一配置
+
统一 Provider
+
统一状态
+
统一历史
+
统一 TUI
+
统一 IPC
```

核心体验：

```text
                  不使用
                    │
                    ▼
                无进程
                    │
                    │
                 用户触发
                    │
                    ▼
                my-trans
                    │
              ┌─────┴─────┐
              │           │
             CLI         TUI
                          │
                          ▼
                       foot
                          │
                     Unix Socket
                          │
                     my-trans Server
                          │
                 ┌────────┼────────┐
                 │        │        │
                OCR    Translation TTS
                 │        │        │
              百度OCR   Provider  espeak
                 │        │
                 └────┬───┘
                      ▼
              TranslationRecord
                      │
                      ▼
                 History[]
                      │
                      ▼
                Bubble Tea
                      │
              ┌───────┴───────┐
              │               │
           自动到底部       用户滚动历史
                              │
                       ↑ ↓ / Wheel
```

最终原则：

> **不使用时没有后台程序。**

> **需要时自动启动。**

> **同一时间只保留一个翻译窗口。**

> **后续翻译永远追加到同一个窗口。**

> **用户可以用鼠标或键盘浏览之前的翻译。**

> **窗口位置由 Niri 管理。**

> **foot 负责窗口和终端显示。**

> **Bubble Tea 负责 TUI。**

> **Go Server 负责状态、业务和 IPC。**

> **Unix Socket 负责本地通信。**

> **百度高精度 OCR 负责截图文字识别。**

> **Translation Provider 负责翻译，并且可以随时替换。**

> **API Key 只通过环境变量提供。**

> **OCR、翻译、TTS 全部按需执行。**

> **没有无意义的常驻服务。**
