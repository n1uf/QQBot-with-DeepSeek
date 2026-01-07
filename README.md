# QQBot with DeepSeek

一个基于 Go 语言开发的 QQ 机器人，集成 DeepSeek AI API，提供智能对话功能。

## 功能特性

- 🤖 **智能对话**：基于 DeepSeek AI 的智能回复
- 💬 **多场景支持**：支持私聊和群聊
- 🎯 **多种触发方式**：
  - 私聊消息自动回复
  - 群聊中艾特机器人
  - 消息中包含"小牛"关键词
- ⚡ **本地命令**：支持以 `niuf` 开头的本地命令
- 🔒 **主人识别**：可识别主人身份，提供个性化回复

## 技术栈

- **语言**：Go 1.21+
- **WebSocket**：gorilla/websocket
- **AI 服务**：DeepSeek API

## 环境要求

- Go 1.21 或更高版本
- NapCat（QQ 机器人框架）
- DeepSeek API Key

## 安装步骤

### 1. 克隆项目

```bash
git clone <repository-url>
cd QQBot-with-DeepSeek
```

### 2. 安装依赖

```bash
go mod download
```

### 3. 配置环境变量

设置以下环境变量：

- `DEEPSEEK_API_KEY`：DeepSeek API 密钥（必需）
- `BOT_QQ`：机器人 QQ 号（可选，用于识别艾特）
- `MASTER_QQ`：主人 QQ 号（可选，用于识别主人身份）

**Windows PowerShell:**
```powershell
$env:DEEPSEEK_API_KEY="your-api-key"
$env:BOT_QQ="123456789"
$env:MASTER_QQ="987654321"
```

**Linux/Mac:**
```bash
export DEEPSEEK_API_KEY="your-api-key"
export BOT_QQ="123456789"
export MASTER_QQ="987654321"
```

### 4. 编译运行

```bash
go build -o QQBot.exe
./QQBot.exe
```

或者直接运行：

```bash
go run main.go
```

## 配置说明

### NapCat 配置

确保 NapCat 配置了 WebSocket 反向连接，连接到 `ws://localhost:8080/ws`

### 环境变量说明

| 变量名 | 说明 | 是否必需 |
|--------|------|---------|
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 | ✅ 必需 |
| `BOT_QQ` | 机器人 QQ 号 | ⚠️ 可选（建议设置） |
| `MASTER_QQ` | 主人 QQ 号 | ⚠️ 可选（建议设置） |

## 使用说明

### 触发方式

1. **私聊**：直接发送消息给机器人
2. **群聊艾特**：在群聊中艾特机器人
3. **关键词触发**：消息中包含"小牛"关键词

### 本地命令

以 `niuf` 开头的消息会被识别为本地命令，由机器人本地处理。

## 项目结构

```
QQBot-with-DeepSeek/
├── main.go          # 主程序文件
├── go.mod           # Go 模块依赖
├── go.sum           # 依赖校验文件
└── README.md        # 项目说明文档
```

## 开发说明

### 核心模块

- **事件分发器**：`dispatch()` 函数处理消息分发逻辑
- **AI 对话处理**：`handleAIChat()` 函数处理 AI 对话请求
- **本地命令处理**：`handleLocalCommand()` 函数处理本地命令
- **WebSocket 通信**：`wsHandler()` 函数处理 WebSocket 连接

### 自定义修改

- **监听端口**：修改 `ListenPort` 常量（默认：`:8080`）
- **AI 模型**：修改 `callDeepSeek()` 函数中的 `model` 参数（默认：`deepseek-chat`）
- **系统提示词**：修改 `callDeepSeek()` 函数中的 `systemMessage`

## 注意事项

- 确保 DeepSeek API Key 有效且有足够的额度
- 建议设置 `BOT_QQ` 和 `MASTER_QQ` 环境变量以获得更好的体验
- WebSocket 连接断开后会自动重连（需要 NapCat 支持）

## 许可证

本项目采用 MIT 许可证。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

如有问题或建议，请通过 Issue 反馈。
