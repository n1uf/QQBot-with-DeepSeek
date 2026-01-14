# QQBot with DeepSeek

一个基于 Go 语言开发的 QQ 机器人，集成 DeepSeek AI API，提供智能对话功能。

## 功能特性

- 🤖 **智能对话**：基于 DeepSeek AI 的智能回复
- 💬 **多场景支持**：支持私聊和群聊
- 🎯 **多种触发方式**：
  - 私聊消息自动回复
  - 群聊中艾特机器人
  - 消息中包含"小牛"关键词
  - 群聊中@主人时自动代为回复
- ⚡ **本地命令**：支持本地命令处理（如"小牛"）
- 🔒 **身份识别**：可识别主人、主人女朋友等特殊身份，提供个性化回复
- 🔁 **重复消息检测**：群聊中连续 3 条相同消息时自动回复相同内容
- 💾 **对话历史**：私聊和群聊上下文记忆，支持最多 50 条历史消息
- 👤 **昵称映射**：自动识别并记忆群聊中的用户昵称，持久化存储

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
# 编译到 bin 目录
go build -o bin/QQBot.exe ./internal

# 运行
./bin/QQBot.exe
```

或者直接运行：

```bash
go run ./internal
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

发送"小牛"会被识别为本地命令，由机器人本地处理。

### 对话历史

- **私聊历史**：每个用户的私聊对话历史会保存在 `data/user_{QQ号}.json`
- **群聊上下文**：每个群的对话上下文会保存在 `data/group_{群号}.json`
- **昵称映射**：每个群的用户昵称映射会保存在 `data/group_{群号}_nicknames.json`

## 项目结构

```
QQBot-with-DeepSeek/
├── internal/              # 源代码目录
│   ├── main.go          # 主程序入口（WebSocket、事件分发）
│   ├── common/          # 共享基础包
│   │   ├── types.go     # 共享类型定义（QQEvent）
│   │   ├── config.go    # 配置变量（环境变量、常量）
│   │   └── sender.go    # 消息发送函数
│   ├── deepseek/        # DeepSeek AI 模块
│   │   ├── handler.go   # 事件处理函数（HandleAIChat、HandleAtMasterChat）
│   │   ├── api.go       # API 调用函数
│   │   └── should.go    # 判断函数（ShouldHandleAIChat、ShouldHandleAtMasterChat）
│   ├── local/            # 本地逻辑模块
│   │   ├── command.go   # 本地命令处理
│   │   └── repeat.go    # 重复消息检测
│   └── storage/          # 数据存储模块
│       └── conversation.go # 对话历史、昵称映射管理
├── bin/                  # 编译输出目录
│   └── QQBot.exe         # 编译后的可执行文件
├── data/                 # 数据存储目录（自动创建）
│   ├── user_*.json      # 私聊对话历史
│   ├── group_*.json     # 群聊上下文
│   └── group_*_nicknames.json # 群昵称映射
├── go.mod                # Go 模块依赖
├── go.sum                # 依赖校验文件
└── README.md             # 项目说明文档
```

## 开发说明

### 架构设计

项目采用模块化设计，按功能划分为多个包：

- **`common` 包**：提供共享的基础功能
  - `types.go`：定义 `QQEvent` 等共享类型
  - `config.go`：管理环境变量和配置常量
  - `sender.go`：提供统一的消息发送接口

- **`deepseek` 包**：处理所有 AI 相关逻辑
  - `handler.go`：`HandleAIChat()` 处理普通 AI 对话，`HandleAtMasterChat()` 处理@主人的情况
  - `api.go`：`callDeepSeekAPI()` 实际调用 DeepSeek API
  - `should.go`：判断是否应该处理 AI 相关事件

- **`local` 包**：处理不需要 AI 的本地逻辑
  - `command.go`：处理本地命令（如"小牛"）
  - `repeat.go`：检测并处理重复消息

- **`storage` 包**：管理数据存储
  - `conversation.go`：管理私聊对话历史、群聊上下文、昵称映射

### 核心流程

1. **消息接收**：`main.go` 的 `wsHandler()` 接收 WebSocket 消息
2. **事件解析**：`parseEvent()` 解析消息并提取信息
3. **事件分发**：`dispatch()` 根据消息类型分发到不同模块
4. **模块处理**：各模块根据职责处理相应事件
5. **消息发送**：通过 `common.SendReply()` 统一发送回复

### 自定义修改

- **监听端口**：修改 `internal/common/config.go` 中的 `ListenPort` 常量（默认：`:8080`）
- **AI 模型**：修改 `internal/deepseek/api.go` 中的 `deepSeekModel` 常量（默认：`deepseek-chat`）
- **系统提示词**：修改 `internal/deepseek/api.go` 中的 `systemPromptBase`、`groupChatContext` 等常量
- **重复消息队列大小**：修改 `internal/common/config.go` 中的 `RepeatMessageQueueSize` 常量（默认：`3`）
- **历史消息数量**：修改 `internal/storage/conversation.go` 中的 `MaxHistoryMessages` 和 `MaxGroupContextMessages` 常量（默认：`50`）
- **消息长度限制**：修改 `internal/storage/conversation.go` 中的 `MaxMessageLength` 常量（默认：`500`）

## 注意事项

- 确保 DeepSeek API Key 有效且有足够的额度
- 建议设置 `BOT_QQ`、`MASTER_QQ` 环境变量以获得更好的体验
- WebSocket 连接断开后会自动重连（需要 NapCat 支持）
- 对话历史文件会自动保存在 `data/` 目录下，请确保有写入权限
- 群聊上下文和昵称映射会持久化存储，重启后自动恢复
- 超长消息（超过 500 字符）不会加入群聊上下文，但仍会触发其他功能

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

如有问题或建议，请通过 Issue 反馈。
