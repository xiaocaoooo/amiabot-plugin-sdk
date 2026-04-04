# AGENTS.md — amiabot-plugin-sdk

> 面向 AI 代理的项目指南。帮助你在编码、审查、重构此仓库时快速理解架构与约定。

---

## 1. 项目概述

amiabot-plugin-sdk 是 NyaNyaBot / AmiaBot 的 **外置插件开发 SDK**。

- **模块路径**：`github.com/xiaocaoooo/amiabot-plugin-sdk`
- **Go 版本**：`1.25.6`
- **核心依赖**：`github.com/hashicorp/go-plugin v1.7.0`
- **通信机制**：基于 HashiCorp go-plugin 框架，底层使用 `net/rpc` 进行进程间通信（IPC）
- **插件形态**：插件被编译为独立可执行文件，放置在宿主程序的 `plugins/` 目录下，由宿主自动发现并加载

---

## 2. 目录结构

```
amiabot-plugin-sdk/
├── go.mod                              # 模块定义
├── go.sum
├── onebot/
│   └── ob11/
│       └── types.go                    # OneBot 11 协议类型定义（Event, APIRequest, APIResponse）
└── plugin/
    ├── api.go                          # 插件核心接口（Plugin）与导出类型定义
    ├── errors.go                       # 结构化错误类型（StructuredError, ErrorCode）
    └── transport/
        ├── proto.go                    # RPC 传输层的 Args/Reply 结构体 + HostAPI 接口定义
        └── plugin_rpc.go              # RPC 客户端/服务端实现 + go-plugin 胶水代码（Map, Handshake 等）
```

**包依赖方向**：
- `onebot/ob11` — 无外部依赖，纯类型定义
- `plugin` — 依赖 `onebot/ob11`
- `plugin/transport` — 依赖 `plugin`（别名 `papi`）和 `onebot/ob11`，以及 `github.com/hashicorp/go-plugin`

---

## 3. 核心接口完整定义

### 3.1 Plugin 接口 — 插件必须实现

```go
// plugin/api.go
type Plugin interface {
    // 返回插件元数据（名称、ID、版本、命令、事件、导出方法、配置等）
    Descriptor(ctx context.Context) (Descriptor, error)

    // 接收/更新配置。config 为 JSON 原始字节
    Configure(ctx context.Context, config json.RawMessage) error

    // 被其他插件通过宿主跨插件调用时触发
    Invoke(ctx context.Context, method string, paramsJSON json.RawMessage, callerPluginID string) (resultJSON json.RawMessage, err error)

    // 事件/命令处理入口。listenerID 标识匹配到的监听器，eventRaw 为原始事件，match 为命令正则匹配结果（事件监听器为 nil）
    Handle(ctx context.Context, listenerID string, eventRaw ob11.Event, match *CommandMatch) (HandleResult, error)

    // 优雅关闭
    Shutdown(ctx context.Context) error
}
```

### 3.2 HostAPI 接口 — 宿主暴露给插件的能力

```go
// plugin/transport/proto.go
type HostAPI interface {
    // 调用 OneBot 11 API（如 send_msg、get_group_list 等）
    CallOneBot(ctx context.Context, action string, params any) (ob11.APIResponse, error)

    // 跨插件调用：调用目标插件的导出方法
    CallDependency(ctx context.Context, targetPluginID string, method string, params json.RawMessage) (json.RawMessage, *papi.StructuredError)

    // 获取宿主运行统计信息
    GetStats(ctx context.Context) (GetStatsReply, error)
}
```

### 3.3 两个接口的 RPC 桥接

| 方向 | Server 实现 | Client 实现 |
|------|------------|------------|
| 宿主 → 插件 | `PluginRPCServer`（插件进程内） | `PluginRPCClient`（宿主进程内） |
| 插件 → 宿主 | `HostRPCServer`（宿主进程内） | `HostRPCClient`（插件进程内） |

插件通过全局函数 `transport.Host()` 获取 `*HostRPCClient`，进而调用宿主能力。

---

## 4. 所有导出类型和函数速查

### 4.1 plugin 包（`plugin/api.go` + `plugin/errors.go`）

| 类型 | 字段 | 说明 |
|------|------|------|
| `Plugin` (interface) | — | 插件必须实现的 5 个方法 |
| `Descriptor` | `Name`, `PluginID`, `Version`, `Author`, `Description`, `Dependencies []string`, `Exports []ExportSpec`, `Config *ConfigSpec`, `Commands []CommandListener`, `Events []EventListener` | 插件元数据 |
| `ConfigSpec` | `Version`, `Description`, `Schema json.RawMessage`, `Default json.RawMessage` | 配置规范（Schema 为 JSON Schema） |
| `ExportSpec` | `Name`, `Description`, `ParamsSchema json.RawMessage`, `ResultSchema json.RawMessage` | 导出方法规范 |
| `CommandListener` | `Name`, `ID`, `Description`, `Pattern string`, `MatchRaw bool`, `Handler string` | 命令监听器（Pattern 为正则） |
| `EventListener` | `Name`, `ID`, `Description`, `Event string`, `Handler string` | 事件监听器（Event 为事件类型名） |
| `CommandMatch` | `Full string`, `Groups []string` | 正则匹配结果 |
| `HandleResult` | （空结构体，预留扩展） | Handle 返回值 |
| `CallResult` | `Raw ob11.APIResponse` | OneBot API 调用结果包装 |
| `CallOneBotFunc` | — | `func(ctx, action, params) (CallResult, error)` |
| `ErrorCode` (string) | 常量：`FORBIDDEN`, `NOT_FOUND`, `INVALID_PARAMS`, `INTERNAL` | 错误码 |
| `StructuredError` | `Code ErrorCode`, `Message string` | 结构化错误，实现 `error` 接口 |

**函数**：

| 函数 | 签名 | 说明 |
|------|------|------|
| `NewStructuredError` | `(code ErrorCode, message string) *StructuredError` | 创建结构化错误 |
| `AsStructuredError` | `(err error) *StructuredError` | 尝试将 error 转为 StructuredError（不成功返回 nil） |
| `NormalizeStructuredError` | `(err error, fallback ErrorCode) *StructuredError` | 统一转为 StructuredError（已是则保留，否则用 fallback 包装） |

### 4.2 ob11 包（`onebot/ob11/types.go`）

| 类型 | 定义 | 说明 |
|------|------|------|
| `Event` | `= json.RawMessage` | 原始 OneBot 11 事件，保持非类型化以保证前向兼容 |
| `APIRequest` | `Action string`, `Params interface{}`, `Echo string` | OneBot API 请求 |
| `APIResponse` | `Status string`, `RetCode int`, `Msg string`, `Wording string`, `Data json.RawMessage`, `Echo string` | OneBot API 响应 |

### 4.3 transport 包（`plugin/transport/`）

**RPC Args/Reply 结构体**：

| 类型 | 说明 |
|------|------|
| `DescribeReply` | = `papi.Descriptor` |
| `ConfigureArgs` | `Config json.RawMessage` |
| `HandleArgs` | `ListenerID string`, `EventRawJSON json.RawMessage`, `Match *CommandMatch` |
| `HandleReply` | = `papi.HandleResult` |
| `InvokeArgs` | `Method string`, `Params json.RawMessage`, `CallerPluginID string` |
| `InvokeReply` | `Result json.RawMessage`, `Error *StructuredError` |
| `CallOneBotArgs` | `Action string`, `Params json.RawMessage` |
| `CallOneBotReply` | `Resp ob11.APIResponse` |
| `CallDependencyArgs` | `TargetPluginID string`, `Method string`, `Params json.RawMessage` |
| `CallDependencyReply` | `Result json.RawMessage`, `Error *StructuredError` |
| `GetStatsArgs` | 空结构体 |
| `GetStatsReply` | `RecvCount int64`, `SentCount int64`, `StartTime time.Time`, `Uptime string` |
| `AttachHostArgs` | `BrokerID uint32` |

**RPC Server/Client**：

| 类型 | 说明 |
|------|------|
| `PluginRPCServer` | 宿主端：持有 `papi.Plugin` 实现 + `*plugin.MuxBroker`，处理宿主对插件的 RPC 调用 |
| `PluginRPCClient` | 宿主端：通过 `*rpc.Client` 调用插件进程，实现 `papi.Plugin` 接口 |
| `HostRPCServer` | 插件端：持有 `HostAPI` 实现，处理插件对宿主的 RPC 调用 |
| `HostRPCClient` | 插件端：通过 `*rpc.Client` 调用宿主进程，实现 `HostAPI` 接口 |

**go-plugin 胶水**：

| 名称 | 说明 |
|------|------|
| `Map` | 实现 `plugin.Plugin` 接口（`Server` / `Client` 方法），持有 `PluginImpl` 和可选 `Host` |
| `PluginName` | `const = "nyanyabot_plugin"`，go-plugin 插件注册键 |
| `Handshake()` | 返回 `plugin.HandshakeConfig`（ProtocolVersion=1, MagicCookieKey="NYANYABOT_PLUGIN", MagicCookieValue="1"） |
| `ServeHostAPI(broker, host)` | 在 broker 上启动 HostAPI RPC 服务，返回 brokerID |
| `SetHost(c)` / `Host()` | 全局 host client 的设置/获取。插件通过 `transport.Host()` 调用宿主能力 |

---

## 5. 插件生命周期图

```
宿主启动
  │
  ├─ 扫描 plugins/ 目录，发现插件可执行文件
  │
  ├─ 启动插件子进程（go-plugin 标准流程）
  │   └─ 插件 main() 中调用 plugin.Serve()，建立 RPC 连接
  │
  ├─ 调用 Descriptor() ──→ 获取插件元数据（名称、命令、事件、导出方法、配置规范）
  │
  ├─ 调用 Configure(configJSON) ──→ 下发配置给插件
  │
  ├─ 事件循环：
  │   │
  │   ├─ 收到 OneBot 事件
  │   │
  │   ├─ 匹配 Commands[].Pattern（正则）──→ 命中则调用 Handle(listenerID, event, &CommandMatch{Full, Groups})
  │   │
  │   └─ 匹配 Events[].Event（事件类型）──→ 命中则调用 Handle(listenerID, event, nil)
  │
  ├─ 跨插件调用：
  │   └─ Invoke(method, paramsJSON, callerPluginID)
  │
  └─ 宿主关闭
      └─ 调用 Shutdown() ──→ 插件清理资源，进程退出
```

---

## 6. RPC 传输层架构

```
┌─────────────────────────────┐         ┌─────────────────────────────┐
│        宿主进程 (Host)       │         │       插件进程 (Plugin)      │
│                             │  net/rpc │                             │
│  PluginRPCClient ──────────┼─────────→│  PluginRPCServer            │
│  (调用插件的 Plugin 接口)    │         │  (转发给 papi.Plugin 实现)    │
│                             │         │                             │
│  HostRPCServer ←───────────┼←─────────│  HostRPCClient              │
│  (实现 HostAPI)             │         │  (通过 transport.Host() 获取) │
│                             │         │                             │
└─────────────────────────────┘         └─────────────────────────────┘
```

**关键流程**：
1. `Map.Server()` 在插件进程中创建 `PluginRPCServer`
2. `Map.Client()` 在宿主进程中创建 `PluginRPCClient`，并可选调用 `AttachHost()` 将宿主 API 注入插件
3. `AttachHost()` 使用 `MuxBroker` 建立第二个 RPC 通道，插件端通过 `AttachHostArgs.BrokerID` 连接宿主的 `HostRPCServer`
4. 插件通过全局 `transport.Host()` 获取 `*HostRPCClient`，调用 `CallOneBot` / `CallDependency` / `GetStats`

---

## 7. 使用示例 — 如何编写一个新插件

### 7.1 最小插件骨架

```go
package main

import (
    "context"
    "encoding/json"
    "log"

    "github.com/hashicorp/go-plugin"
    "github.com/xiaocaoooo/amiabot-plugin-sdk/onebot/ob11"
    papi "github.com/xiaocaoooo/amiabot-plugin-sdk/plugin"
    "github.com/xiaocaoooo/amiabot-plugin-sdk/plugin/transport"
)

// ---- 实现 papi.Plugin 接口 ----

type MyPlugin struct{}

func (p *MyPlugin) Descriptor(_ context.Context) (papi.Descriptor, error) {
    return papi.Descriptor{
        Name:     "我的插件",
        PluginID: "my_plugin",
        Version:  "0.1.0",
        Author:   "YourName",
        Description: "示例插件",
        Commands: []papi.CommandListener{
            {
                Name:        "ping",
                ID:          "ping",
                Description: "回复 pong",
                Pattern:     `^ping$`,
                Handler:     "handlePing",
            },
        },
        Events: []papi.EventListener{
            {
                Name:        "入群欢迎",
                ID:          "welcome",
                Description: "新成员入群时发送欢迎消息",
                Event:       "notice.group_increase",
                Handler:     "handleWelcome",
            },
        },
        Exports: []papi.ExportSpec{
            {
                Name:         "greet",
                Description:  "问候接口",
                ParamsSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
            },
        },
        Config: &papi.ConfigSpec{
            Version:     "1",
            Description: "示例配置",
            Default:     json.RawMessage(`{"greeting":"Hello"}`),
        },
    }, nil
}

func (p *MyPlugin) Configure(_ context.Context, config json.RawMessage) error {
    // 解析并存储配置
    log.Printf("收到配置: %s", string(config))
    return nil
}

func (p *MyPlugin) Invoke(_ context.Context, method string, paramsJSON json.RawMessage, callerPluginID string) (json.RawMessage, error) {
    switch method {
    case "greet":
        var params struct {
            Name string `json:"name"`
        }
        if err := json.Unmarshal(paramsJSON, &params); err != nil {
            return nil, papi.NewStructuredError(papi.ErrorCodeInvalidParams, "无效参数")
        }
        return json.Marshal(map[string]string{"message": "Hello, " + params.Name})
    default:
        return nil, papi.NewStructuredError(papi.ErrorCodeNotFound, "未知方法: "+method)
    }
}

func (p *MyPlugin) Handle(_ context.Context, listenerID string, eventRaw ob11.Event, match *papi.CommandMatch) (papi.HandleResult, error) {
    switch listenerID {
    case "handlePing":
        // 通过宿主 API 回复消息
        host := transport.Host()
        if host != nil {
            // 从事件中解析 group_id 和 user_id 以构造回复
            // ...
            _, _ = host.CallOneBot(context.Background(), "send_msg", map[string]any{
                "message_type": "group",
                "message":      "pong",
            })
        }
    case "handleWelcome":
        log.Printf("新成员入群事件: %s", string(eventRaw))
    }
    return papi.HandleResult{}, nil
}

func (p *MyPlugin) Shutdown(_ context.Context) error {
    log.Println("插件关闭")
    return nil
}

// ---- 启动 go-plugin 服务 ----

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: transport.Handshake(),
        Plugins: plugin.PluginSet{
            transport.PluginName: &transport.Map{PluginImpl: &MyPlugin{}},
        },
    })
}
```

### 7.2 调用宿主能力

在插件任意位置，通过 `transport.Host()` 获取宿主客户端：

```go
host := transport.Host()
if host != nil {
    // 调用 OneBot API
    resp, err := host.CallOneBot(ctx, "send_group_msg", map[string]any{
        "group_id": 123456,
        "message":  "你好！",
    })

    // 跨插件调用
    result, err := host.CallDependency(ctx, "other_plugin", "greet", json.RawMessage(`{"name":"World"}`))
}
```

### 7.3 编译与部署

```bash
# 编译插件为可执行文件
go build -o my_plugin ./cmd/my_plugin/

# 放置到宿主的 plugins/ 目录
cp my_plugin /path/to/AmiaBot/plugins/
```

---

## 8. 被哪些项目引用

- **AmiaBot**（外置插件集合）：通过 `replace` 指令本地引用此 SDK，包含 7 个外置插件
- **NyaNyaBot**（宿主程序）：可能引用用于宿主端的插件加载与管理

---

## 9. 开发约定

### 代码风格
- 使用标准 Go 代码风格（`gofmt` / `goimports`）
- 包 `transport` 内部使用 `papi` 作为 `plugin` 包的别名，避免与 `hashicorp/go-plugin` 冲突
- 错误处理使用 `StructuredError`，通过 `NormalizeStructuredError` 统一转换

### 接口设计
- `Plugin` 接口的 5 个方法必须全部实现，不可 panic
- `Handle` 方法中 `match` 参数在事件监听器场景下为 `nil`，需要判空
- `Invoke` 方法返回的错误应使用 `StructuredError`，宿主会通过 `NormalizeStructuredError` 兜底转换

### RPC 层
- `PluginRPCServer` / `HostRPCServer` 的方法签名为 `func(args, *reply) error`，符合 `net/rpc` 规范
- `Invoke` 和 `CallDependency` 的错误不通过 RPC 返回值传递（`return nil`），而是放在 `Reply.Error` 字段中
- `PluginRPCClient` 的方法实现了 `papi.Plugin` 接口，但忽略 `ctx` 参数（RPC 本身无上下文传递）

### 命名约定
- 插件 ID 使用 snake_case（如 `my_plugin`）
- 导出方法名使用 camelCase（如 `greet`、`getInfo`）
- 事件类型使用点分格式（如 `notice.group_increase`、`message.group`）
- 命令 Pattern 使用正则表达式，以 `^` 开头 `$` 结尾确保全匹配

### 版本管理
- 插件版本遵循语义化版本（SemVer）
- `ConfigSpec.Version` 是配置格式版本，与插件版本独立

---

## 10. 常见任务指引

### 如果你要创建一个新插件
1. 创建新的 Go 模块（或在 AmiaBot 仓库中创建子目录）
2. 在 `go.mod` 中添加 `require github.com/xiaocaoooo/amiabot-plugin-sdk`（或使用 `replace` 本地引用）
3. 实现 `plugin.Plugin` 接口的全部 5 个方法
4. 在 `main()` 中调用 `plugin.Serve()` 启动 go-plugin 服务
5. 编译后将可执行文件放到宿主的 `plugins/` 目录

### 如果你要修改插件接口（Plugin interface）
1. 编辑 `plugin/api.go`
2. 同步更新 `transport/plugin_rpc.go` 中的 `PluginRPCServer` 和 `PluginRPCClient`
3. 同步更新 `transport/proto.go` 中的 Args/Reply 结构体
4. 注意 RPC 方法签名必须是 `func(args, *reply) error`
5. 这是破坏性变更，会影响所有已编译的插件

### 如果你要添加新的宿主能力
1. 在 `transport/proto.go` 中定义 `HostAPI` 接口的新方法
2. 添加对应的 Args/Reply 结构体
3. 在 `transport/plugin_rpc.go` 中实现 `HostRPCServer`（宿主端）和 `HostRPCClient`（插件端）的对应方法
4. 确保 `HostRPCClient` 的方法签名与 `HostAPI` 接口一致

### 如果你要添加新的错误码
1. 在 `plugin/errors.go` 中添加 `ErrorCode` 常量
2. 保持命名风格（大写 SNAKE_CASE 字符串）

### 如果你要修改 OneBot 协议类型
1. 编辑 `onebot/ob11/types.go`
2. `Event` 保持为 `json.RawMessage`（非类型化），以保证前向兼容
3. 如果需要类型化的事件解析，在插件侧自行 `json.Unmarshal`

### 如果你要调试 RPC 通信
1. go-plugin 底层使用 `net/rpc` + `yamux` 多路复用
2. `Handshake` 配置中 `MagicCookieKey="NYANYABOT_PLUGIN"`，`MagicCookieValue="1"`，版本不匹配会导致插件启动失败
3. `MuxBroker` 用于在主 RPC 连接上建立额外的子连接（用于 HostAPI 通道）
4. 插件进程的标准输出/错误可通过 go-plugin 的日志配置获取
