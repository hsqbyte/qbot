# qbot

基于 Go + gorilla/websocket 裸写的 QQ 群机器人，通过 OneBot v11 协议对接 NapCat。

## 项目结构

```
main.go                    # 入口（go run main.go dev）
config/                    # 配置文件（YAML，按 APP_ENV 选择）
deploy/docker/             # Dockerfile + docker-compose
src/
  server.go                # 启动引导（配置→日志→Bot→Graceful Shutdown）
  core/
    config.go              # 配置加载
    logger.go              # 日志器
    bot/
      bot.go               # 核心：WebSocket 连接、自动重连、事件分发
  models/
    event.go               # OneBot v11 事件模型
    action.go              # OneBot v11 动作模型
    action_group.go        # 群管理动作扩展
  services/
    bot_service.go         # 业务逻辑层
  handler/
    commands.go            # 群消息命令 + 关键词回复 + 入群欢迎
    admin.go               # 群管理（踢人、禁言、全员禁言）
  registry/
    registry.go            # Handler 注册中心
  utils/
    strings.go             # 工具函数
```

## 特性

- **分层架构**: Handler → Service → Model（类 fastgox-api-starter）
- **自动注册**: Handler 通过 `init()` 自动注册
- **自动重连**: WebSocket 断线后自动重连
- **Graceful Shutdown**: 信号捕获优雅退出
- **群消息功能**:
  - `/ping` `/help` `/status` `/echo` `/at` 命令
  - 关键词自动回复（可配置规则）
  - 新人入群欢迎
- **群管理功能**:
  - `/kick @xxx` 踢人
  - `/ban @xxx 60` 禁言（秒）
  - `/unban @xxx` 解禁
  - `/mute` `/unmute` 全员禁言
- Docker 一键部署（含 NapCat）

## 快速开始

### 1. 本地开发

```bash
# 先启动 NapCat（Docker）
docker run -d --name napcat -p 6099:6099 -p 3001:3001 mlikiowa/napcat-docker:latest

# 浏览器打开 http://localhost:6099/webui/ 扫码登录小号

# 启动 Bot
go run main.go dev
```

### 2. Docker 部署

```bash
cd deploy/docker
docker-compose up -d

# 打开 NapCat WebUI 扫码登录
# http://服务器IP:6099/webui/
```

### 3. Task 命令

```bash
task dev       # 开发模式
task build     # 编译
task release   # 发布构建
task fmt       # 格式化
```

## 配置

通过 `APP_ENV` 选择配置文件：

```bash
APP_ENV=dev   # → config/dev.yaml
APP_ENV=prod  # → config/prod.yaml
```

### 添加管理员

在 `config/dev.yaml` 中设置管理员 QQ 号：

```yaml
bot:
  admins:
    - 123456789  # 你的 QQ 号
```

### 添加关键词回复

编辑 `src/services/bot_service.go` 中的 `keywordRules`：

```go
var keywordRules = []KeywordRule{
    {
        Keywords: []string{"你好", "hello"},
        Reply:    func(nick string) string { return "你好 " + nick },
    },
    // 添加更多规则...
}
```

## 新增 Handler

在 `src/handler/` 下新建文件，通过 `init()` 自动注册：

```go
package handler

func init() {
    registry.RegisterHandler(&MyHandler{})
}

type MyHandler struct{}

func (h *MyHandler) Name() string { return "MyHandler" }
func (h *MyHandler) Match(event *model.Event, msg *model.MessageEvent) bool { ... }
func (h *MyHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action { ... }
```

## 架构

```
QQ 服务器  ←→  NapCat (Docker)  ←→  qbot (WebSocket/OneBot v11)
                                        ├── Handler (事件匹配分发)
                                        ├── Service (业务逻辑)
                                        └── Model   (数据模型)
```
