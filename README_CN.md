# Claude Agent SDK for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/godeps/claude-agent-sdk-go.svg)](https://pkg.go.dev/github.com/godeps/claude-agent-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/godeps/claude-agent-sdk-go)](https://goreportcard.com/report/github.com/godeps/claude-agent-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Python SDK Parity](https://img.shields.io/badge/Python%20SDK%20Parity-100%25-brightgreen)](docs/python-sdk-alignment.md)

[English](README.md) | **中文**

基于 Claude Code CLI 的 Go SDK，为构建 AI 驱动的应用程序提供强大的接口。

## 100% Python SDK 功能对齐

本 Go SDK 实现了与[官方 Python SDK](https://github.com/anthropics/claude-agent-sdk-python) **完全功能对齐**，覆盖全部 257 项特性：

- 全部 12 种 Hook 事件（PreToolUse、PostToolUse、PrePrompt、PostPrompt 等）
- Python `@tool` 装饰器风格 API：`SimpleTool`、`Tool()`、`QuickTool()`
- 完整的 MCP 服务器支持（stdio、SSE、HTTP、SDK）
- 全部权限模式和回调（含完整 ToolPermissionContext 9 字段）
- 工具拦截器（Tool Handler）：回调模式 + 事件流模式
- 文件检查点与回滚
- 结构化输出（JSON Schema）
- 中间件系统、Agent 池、泛型类型查询、重试、认证提供者

详见 [Python SDK 对齐指南](docs/python-sdk-alignment.md)。

## 概览

SDK 提供以下核心能力：

- **简单查询接口**：通过 `Query` 函数执行一次性查询
- **交互式客户端**：通过 `Client` 进行多轮对话
- **工具集成**：支持 Bash、Read、Write、Edit、Grep、Glob 等工具
- **权限管理**：细粒度工具权限控制和自定义回调
- **工具拦截**：通过 Tool Handler 拦截工具调用，构建自定义 UI
- **中间件系统**：可组合的查询管道（日志、限流、成本控制）
- **Agent 池**：FanOut / MapReduce 并发多代理工作流
- **泛型类型查询**：`QueryTyped[T]()` 自动反序列化到 Go 结构体
- **MCP 服务器**：集成 Model Context Protocol 服务器
- **Hook 系统**：响应生命周期事件
- **第三方 API 兼容**：支持 DashScope、OpenRouter 等兼容接口

## 安装

```bash
go get github.com/godeps/claude-agent-sdk-go
```

## 前提条件

- 安装 Claude Code CLI：`npm install -g @anthropic-ai/claude-code`
- 设置 API 密钥环境变量：`ANTHROPIC_API_KEY`

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"

    claude "github.com/godeps/claude-agent-sdk-go"
    "github.com/godeps/claude-agent-sdk-go/types"
)

func main() {
    ctx := context.Background()
    opts := types.NewClaudeAgentOptions().WithModel("claude-sonnet-4-6")

    messages, err := claude.Query(ctx, "1+1等于几？", opts)
    if err != nil {
        if types.IsCLINotFoundError(err) {
            log.Fatal("未安装 Claude CLI")
        }
        log.Fatal(err)
    }

    for msg := range messages {
        switch m := msg.(type) {
        case *types.AssistantMessage:
            for _, block := range m.Content {
                if tb, ok := block.(*types.TextBlock); ok {
                    fmt.Println(tb.Text)
                }
            }
        case *types.ResultMessage:
            if m.TotalCostUSD != nil {
                fmt.Printf("费用: $%.4f\n", *m.TotalCostUSD)
            }
        }
    }
}
```

## 第三方 API / 自定义端点（重要）

使用自定义模型端点（代理、自托管、或第三方兼容 API 如 DashScope、OpenRouter）时，**必须**配置三个关键设置，否则 CLI 子进程无法正常工作：

| 配置 | CLI 参数 | 解决的问题 |
|------|---------|-----------|
| `WithAllowDangerouslySkipPermissions(true)` + `WithDangerouslySkipPermissions(true)` | `--allow-dangerously-skip-permissions --dangerously-skip-permissions` | CLI 子进程在每次工具调用时阻塞等待终端交互授权，导致超时无输出 |
| `WithBareMode()` | `--bare` | CLI 输出包含进度条、spinner、ANSI 转义码，SDK 的 JSON 消息解析器无法正确解析 |
| `WithSettingsOverride({"env": ...})` | `--settings '{...}'` | CLI 读取 `~/.claude/settings.json` 可能覆盖你的 API key/base URL；仅用 `WithEnvVar` 不够，因为 CLI settings 优先级更高 |

**完整可运行示例：**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    claude "github.com/godeps/claude-agent-sdk-go"
    "github.com/godeps/claude-agent-sdk-go/types"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    customBaseURL := "https://your-proxy.example.com/v1"
    customAPIKey  := "sk-your-custom-api-key"
    modelName     := "claude-sonnet-4-6"

    opts := types.NewClaudeAgentOptions().
        // (1) 跳过权限检查 — 防止 CLI 阻塞等待交互式授权
        // 两个 flag 缺一不可："allow" 是安全开关，"skip" 是实际跳过。
        // 安全警告：仅在沙箱/自动化环境中使用。
        WithAllowDangerouslySkipPermissions(true).
        WithDangerouslySkipPermissions(true).

        // (2) Bare mode — 强制 stdout 输出纯 JSON 协议消息
        // 没有这个，stdout 包含 ANSI 码和进度条，SDK 解析器会报错。
        WithBareMode().

        // (3) Settings override — 优先级最高，覆盖 ~/.claude/settings.json
        // 确保自定义端点和 API key 被 CLI 子进程实际使用。
        WithSettingsOverride(map[string]interface{}{
            "env": map[string]interface{}{
                "ANTHROPIC_BASE_URL": customBaseURL,
                "ANTHROPIC_API_KEY":  customAPIKey,
            },
        }).

        // 常规配置
        WithModel(modelName).
        WithMaxTurns(5).
        WithSystemPromptString("你是一个有用的助手。")

    messages, err := claude.Query(ctx, "1+1等于几？一个字回答。", opts)
    if err != nil {
        log.Fatalf("查询失败: %v", err)
    }

    for msg := range messages {
        switch m := msg.(type) {
        case *types.AssistantMessage:
            for _, block := range m.Content {
                if tb, ok := block.(*types.TextBlock); ok {
                    fmt.Println(tb.Text)
                }
            }
        case *types.ResultMessage:
            cost := 0.0
            if m.TotalCostUSD != nil {
                cost = *m.TotalCostUSD
            }
            fmt.Printf("费用: $%.6f, 会话: %s\n", cost, m.SessionID)
        }
    }
}
```

**为什么三者缺一不可：**

- 没有 `DangerouslySkipPermissions` — CLI 子进程在每次工具调用时阻塞等待终端输入，你的程序会卡死无输出
- 没有 `BareMode` — stdout 被 rich UI 内容污染，SDK 的 JSON line 解析器无法解码
- 没有 `SettingsOverride` — CLI 可能从用户本地 `~/.claude/settings.json` 读取不同的 API key 或 base URL，完全忽略你的自定义端点

**生产环境推荐的三层配置**（belt-and-suspenders）：

```go
opts := types.NewClaudeAgentOptions().
    WithAllowDangerouslySkipPermissions(true).
    WithDangerouslySkipPermissions(true).
    WithBareMode().
    // 第 1 层：CLI --settings 覆盖（最高优先级）
    WithSettingsOverride(map[string]interface{}{
        "env": map[string]interface{}{
            "ANTHROPIC_BASE_URL": baseURL,
            "ANTHROPIC_API_KEY":  apiKey,
        },
    }).
    // 第 2 层：子进程环境变量
    WithBaseURL(baseURL).
    WithEnvVar("ANTHROPIC_API_KEY", apiKey).
    // 第 3 层：CLI --model 参数 + ANTHROPIC_MODEL 环境变量
    WithModel("claude-sonnet-4-6").
    WithMaxTurns(10)
```

也可以通过环境变量配置（更简单但可靠性较低）：
```bash
export ANTHROPIC_BASE_URL=https://dashscope.aliyuncs.com/apps/anthropic
export ANTHROPIC_AUTH_TOKEN=your-token
export LLM_MODEL=glm-5.1
```

完整示例见 [examples/configuration/custom_endpoint](examples/configuration/custom_endpoint/main.go)。

**真实运行示例**（使用 DashScope + glm-5.1）：
```
=== Complete Permission Callback Example ===
Prompt: Create a file called /tmp/hello_sdk_test.txt with the content 'Hello from Claude SDK!'

+----- Permission Request -------------------------
| Tool:           Write
| ToolUseID:      toolu_tool-a61d274966af42569ace5a51a054dbed
| DisplayName:    Write
| DecisionReason: Path is outside allowed working directories
| Suggestions:    2
| Input keys:     [file_path content]
+--------------------------------------------------
  -> ALLOW (rewritten path: /tmp/hello_sdk_test.txt -> /tmp/hello_sdk_test.txt.safe)

--- Session: 8097540f... | Duration: 14572ms | Cost: $0.1465 ---
```

## 功能详解

### 配置选项

```go
opts := types.NewClaudeAgentOptions().
    WithModel("claude-sonnet-4-6").      // 主模型
    WithFallbackModel("claude-haiku-4-5"). // 备用模型
    WithAllowedTools("Bash", "Write", "Read").    // 允许的工具
    WithPermissionMode(types.PermissionModeAcceptEdits). // 权限模式
    WithMaxBudgetUSD(1.0).                        // 预算限制
    WithSystemPromptString("你是一个有用的编程助手。"). // 系统提示词
    WithCWD("/path/to/working/directory")         // 工作目录
```

### 结构化输出

请求符合 JSON Schema 的结构化输出：

```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "answer": map[string]interface{}{"type": "string"},
    },
    "required": []interface{}{"answer"},
}

opts := types.NewClaudeAgentOptions().WithJSONSchemaOutput(schema)

for msg := range claude.Query(ctx, "以 JSON 格式返回答案", opts) {
    if res, ok := msg.(*types.ResultMessage); ok {
        fmt.Printf("结构化输出: %#v\n", res.StructuredOutput)
    }
}
```

### 泛型类型查询

使用 Go 泛型自动将结构化输出反序列化为 Go 结构体：

```go
type Analysis struct {
    Summary    string   `json:"summary"`
    Issues     []string `json:"issues"`
    Confidence float64  `json:"confidence"`
}

result, meta, err := claude.QueryTyped[Analysis](ctx, "分析这段代码的bug", opts)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("摘要: %s (置信度: %.0f%%)\n", result.Summary, result.Confidence*100)
fmt.Printf("费用: $%.4f, 轮次: %d\n", meta.CostUSD, meta.NumTurns)
```

### 文件检查点与回滚

```go
opts := types.NewClaudeAgentOptions().WithEnableFileCheckpointing(true)

client, _ := claude.NewClient(ctx, opts)
defer client.Close(ctx)
_ = client.Connect(ctx)
_ = client.Query(ctx, "安全地修改文件")

var checkpoint string
for msg := range client.ReceiveResponse(ctx) {
    if user, ok := msg.(*types.UserMessage); ok && user.UUID != nil {
        checkpoint = *user.UUID
    }
}

// 回滚到检查点
_ = client.RewindFiles(ctx, checkpoint)
```

### 权限回调

通过自定义回调精确控制工具权限，支持完整的上下文信息（Python SDK 完全对齐）：

```go
opts := types.NewClaudeAgentOptions().
    WithPermissionMode(types.PermissionModeDefault).
    WithCanUseTool(func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
        // 完整的权限上下文
        fmt.Printf("工具: %s (ID: %s)\n", toolName, permCtx.ToolUseID)
        fmt.Printf("显示名: %s\n", permCtx.DisplayName)
        fmt.Printf("原因: %s\n", permCtx.DecisionReason)

        // 只读工具直接放行
        if toolName == "Read" || toolName == "Grep" {
            return types.PermissionResultAllow{Behavior: "allow"}, nil
        }

        // 拒绝危险操作（Interrupt=true 立即停止执行）
        if toolName == "Bash" {
            cmd, _ := input["command"].(string)
            if strings.Contains(cmd, "rm -rf") {
                return types.PermissionResultDeny{
                    Behavior:  "deny",
                    Message:   "危险命令已阻止",
                    Interrupt: true,
                }, nil
            }
        }

        // 重写工具输入
        if toolName == "Write" {
            filePath, _ := input["file_path"].(string)
            updated := make(map[string]interface{})
            for k, v := range input { updated[k] = v }
            updated["file_path"] = "/safe/" + filePath
            return types.PermissionResultAllow{
                Behavior:     "allow",
                UpdatedInput: &updated,
            }, nil
        }

        // 自动应用 CLI 建议的权限规则
        if len(permCtx.Suggestions) > 0 {
            return types.PermissionResultAllow{
                Behavior:           "allow",
                UpdatedPermissions: permCtx.Suggestions,
            }, nil
        }

        return types.PermissionResultAllow{Behavior: "allow"}, nil
    })
```

**ToolPermissionContext 字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `ToolUseID` | `string` | 本次工具调用的唯一标识 |
| `AgentID` | `string` | 请求权限的 Agent ID |
| `Title` | `string` | 工具标题 |
| `DisplayName` | `string` | 显示名称 |
| `Description` | `string` | 工具描述 |
| `BlockedPath` | `string` | 触发权限检查的文件路径 |
| `DecisionReason` | `string` | 需要权限的原因（如"路径在允许的工作目录之外"） |
| `Suggestions` | `[]PermissionUpdate` | CLI 建议的权限规则 |
| `Signal` | `interface{}` | 保留字段，用于未来的中止信号支持 |

详见 [examples/permissions/permission_callback_complete](examples/permissions/permission_callback_complete/main.go)。

### 工具拦截器（Tool Handler）

注册处理器拦截工具调用，适用于构建自定义 UI、转发到 Web 前端或程序化工具执行。

**回调模式** — 直接调用处理器：
```go
opts := types.NewClaudeAgentOptions().
    WithToolHandler("AskUserQuestion", func(ctx context.Context, req types.ToolHandlerRequest) (*types.ToolResult, error) {
        fmt.Printf("工具: %s, 输入: %v\n", req.ToolName, req.Input)
        return types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "用户的回答"}), nil
    })
```

**事件流模式** — 通过 `ReceiveResponse()` 接收 `ToolExecutionRequest`，异步提交结果：
```go
opts := types.NewClaudeAgentOptions().
    WithToolHandler("AskUserQuestion", nil) // nil = 事件流模式

client, _ := claude.NewClient(ctx, opts)
client.Connect(ctx)
client.Query(ctx, "问我一个问题")

for msg := range client.ReceiveResponse(ctx) {
    switch m := msg.(type) {
    case *types.ToolExecutionRequest:
        // 转发到 Web 前端，收集用户回答后提交
        result := types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "蓝色"})
        client.SubmitToolResult(ctx, m.ToolUseID, result)
    case *types.AssistantMessage:
        // 处理响应
    }
}
```

详见 [examples/tool_handler](examples/tool_handler/main.go)。

### Hook 系统

响应 Claude 生命周期中的各种事件，支持全部 12 种 Hook 事件：

- `HookEventPreToolUse` — 工具执行前
- `HookEventPostToolUse` — 工具执行后
- `HookEventUserPromptSubmit` — 用户提交提示词
- `HookEventPrePrompt` — 发送消息到模型前
- `HookEventPostPrompt` — 收到模型响应后
- `HookEventPreResponse` — 发送响应给用户前
- `HookEventPostResponse` — 发送响应给用户后
- `HookEventPreCompact` — 上下文压缩前
- `HookEventPostCompact` — 上下文压缩后
- `HookEventOnError` — 发生错误时
- `HookEventStop` — Agent 停止时
- `HookEventSubagentStop` — 子 Agent 停止时

```go
opts := types.NewClaudeAgentOptions().
    WithHook(types.HookEventPreToolUse, types.HookMatcher{
        Hooks: []types.HookCallbackFunc{
            func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
                log.Printf("工具即将执行")
                return &types.SyncHookJSONOutput{}, nil
            },
        },
    })
```

详见 [examples/hooks/comprehensive_hooks](examples/hooks/comprehensive_hooks/main.go)。

### MCP 服务器集成

配置外部 Model Context Protocol 服务器：

```go
mcpServers := map[string]interface{}{
    "my-server": map[string]interface{}{
        "type":    "stdio",
        "command": "/path/to/server",
        "args":    []string{"--arg", "value"},
    },
}

opts := types.NewClaudeAgentOptions().WithMcpServers(mcpServers)
```

### 自定义工具（SDK MCP 服务器）

创建进程内自定义工具，类似 Python 的 `@tool` 装饰器：

**方法 1：SimpleTool（最接近 Python @tool）**
```go
greetTool := types.SimpleTool{
    Name:        "greet",
    Description: "按名字问候用户",
    Parameters: map[string]types.SimpleParam{
        "name": {Type: "string", Description: "用户名", Required: true},
    },
    Handler: func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
        name := args["name"].(string)
        return types.NewMcpToolResult(
            types.TextBlock{Type: "text", Text: fmt.Sprintf("你好, %s!", name)},
        ), nil
    },
}

tool, _ := greetTool.Build()
```

**方法 2：链式 API**
```go
tool, _ := types.Tool("greet", "问候用户").
    Param("name", "string", "用户名", true).
    Handle(func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
        name := args["name"].(string)
        return types.NewMcpToolResult(
            types.TextBlock{Type: "text", Text: fmt.Sprintf("你好, %s!", name)},
        ), nil
    })
```

**方法 3：QuickTool（极简）**
```go
tool, _ := types.QuickTool("greet", "问候用户",
    map[string]string{"name": "string"},
    func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
        name := args["name"].(string)
        return types.NewMcpToolResult(
            types.TextBlock{Type: "text", Text: fmt.Sprintf("你好, %s!", name)},
        ), nil
    },
)
```

**使用自定义工具：**
```go
server := types.CreateToolServer("my-tools", "1.0.0", []types.McpTool{tool})

opts := types.NewClaudeAgentOptions().
    WithMcpServers(map[string]interface{}{"tools": server}).
    WithAllowedTools("mcp__tools__greet")

messages, _ := claude.Query(ctx, "问候 Alice", opts)
```

### 中间件系统

用可组合的中间件包装查询，实现日志、限流、成本控制等横切关注点：

```go
sdk := claude.NewSDK(
    claude.AuditLogMiddleware(slog.Default()),    // 记录每次查询
    claude.TimeoutMiddleware(5 * time.Minute),     // 单次查询超时
    claude.RateLimitMiddleware(3),                 // 最多 3 个并发查询
    claude.CostGuardMiddleware(10.0, func(spent float64) {
        log.Printf("预算超限: 已花费 $%.2f", spent)
    }),
)

messages, _ := sdk.Query(ctx, "你好", opts)
```

### Agent 池（Fan-Out / Map-Reduce）

控制并发度运行多个查询：

```go
pool := claude.NewAgentPool(5, opts) // 5 个并发 Agent

// Fan-out: 发送多个提示词，收集所有结果
results := pool.FanOut(ctx, []string{
    "总结第一章",
    "总结第二章",
    "总结第三章",
})

// Map-Reduce: 拆分任务，合并结果
final, _ := pool.MapReduce(ctx, chapters,
    func(item string) string { return "总结: " + item },
    func(results []claude.AgentResult) string {
        return "将这些摘要合并为一个: ..."
    },
)
```

### 重试机制

自动重试瞬态失败，支持指数退避：

```go
opts := types.NewClaudeAgentOptions().
    WithRetry(&types.RetryConfig{
        MaxRetries:     3,
        InitialBackoff: time.Second,
        MaxBackoff:     30 * time.Second,
        Multiplier:     2.0,
    })

messages, _ := claude.QueryWithRetry(ctx, "你好", opts)
```

### 认证提供者

可插拔的认证机制，支持不同 API 后端：

```go
// API Key（默认 Anthropic）
opts := types.NewClaudeAgentOptions().
    WithAuthProvider(types.NewAPIKeyAuth("sk-..."))

// Bearer Token（如 DashScope、Azure）
opts := types.NewClaudeAgentOptions().
    WithAuthProvider(types.NewBearerTokenAuth("your-token"))

// HMAC 签名
opts := types.NewClaudeAgentOptions().
    WithAuthProvider(types.NewHMACAuth("key-id", "secret-key"))
```

### 事件回调

实时追踪工具使用和进度：

```go
opts := types.NewClaudeAgentOptions().
    WithOnToolEvent(func(event types.ToolEvent) {
        fmt.Printf("[%s] 工具: %s (%.0fms)\n", event.Phase, event.ToolName, event.DurationMs)
    }).
    WithOnProgress(func(p types.Progress) {
        fmt.Printf("轮次 %d | 费用: $%.4f\n", p.NumTurns, p.CostUSD)
    })
```

### 结构化日志

使用 `slog.Logger` 进行结构化、分级日志记录：

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

opts := types.NewClaudeAgentOptions().WithLogger(logger)
```

### 会话管理

列出和恢复历史会话：

```go
sessions, _ := claude.ListSessions(ctx, opts)
for _, s := range sessions {
    fmt.Printf("会话 %s: %s (模型: %s)\n", s.ID, s.Summary, s.Model)
}

// 恢复会话
resumeOpts := claude.ResumeSession("session-id-here")
client, _ := claude.NewClient(ctx, resumeOpts)
```

## 错误处理

SDK 提供类型化错误，用于特定故障场景：

- `CLINotFoundError`：找不到 Claude Code CLI
- `CLIConnectionError`：无法连接到 CLI 进程
- `ProcessError`：CLI 子进程错误（退出码、崩溃）
- `CLIJSONDecodeError`：CLI 返回无效 JSON
- `MessageParseError`：JSON 有效但消息结构无效
- `ControlProtocolError`：控制协议违规
- `PermissionDeniedError`：权限请求被拒绝

```go
messages, err := claude.Query(ctx, "你好", opts)
if err != nil {
    if types.IsCLINotFoundError(err) {
        log.Fatal("请安装 Claude CLI: npm install -g @anthropic-ai/claude-code")
    }
    if types.IsCLIConnectionError(err) {
        log.Printf("连接错误: %v", err)
    }
    log.Fatal(err)
}
```

## 线程安全与并发

`Client` 类型**有意设计为非线程安全** — 每个 Client 代表单个对话会话，会话天然是顺序的。

### 推荐模式

| 模式 | 线程安全 | 适用场景 | 推荐 |
|------|----------|----------|------|
| 每协程一个 Client | 隔离安全 | 独立任务 | **推荐** |
| `Query()` 函数 | 隔离安全 | 一次性无状态查询 | **推荐** |
| `ConcurrentClient` | 同步安全 | 共享会话（极少） | 谨慎使用 |

```go
// 最佳实践：每个协程一个 Client
var wg sync.WaitGroup
for _, task := range tasks {
    wg.Add(1)
    go func(t string) {
        defer wg.Done()
        client, _ := claude.NewClient(ctx, opts)
        defer client.Close(ctx)
        client.Connect(ctx)
        client.Query(ctx, t)
        for msg := range client.ReceiveResponse(ctx) {
            // 处理消息
        }
    }(task)
}
wg.Wait()
```

详细的设计原理、性能分析和高级示例请参阅 [并发指南](docs/concurrency-guide.md)。

## 消息类型

SDK 处理以下消息类型：

- `UserMessage`：用户发送给 Claude 的消息
- `AssistantMessage`：Claude 的响应，包含内容块
- `SystemMessage`：系统通知和元数据
- `ResultMessage`：最终结果，包含费用/用量信息
- `StreamEvent`：流式传输中的部分更新

内容块包括：

- `TextBlock`：纯文本内容
- `ThinkingBlock`：Claude 的内部推理过程
- `ToolUseBlock`：工具调用请求
- `ToolResultBlock`：工具执行结果

## 示例

查看 [examples 目录](examples/) 获取详细使用示例：

### 基础示例
- [快速开始](examples/basic/quick_start/main.go) — 最简示例
- [简单查询](examples/basic/simple_query/main.go) — 一次性查询
- [交互式客户端](examples/basic/interactive_client/main.go) — 多轮对话

### 配置示例
- [自定义端点](examples/configuration/custom_endpoint/main.go) — **重要：自定义/代理端点必需的三项关键配置**
- [系统提示词](examples/configuration/system_prompt/main.go) — 自定义系统提示词
- [预算限制](examples/configuration/max_budget_usd/main.go) — 费用控制
- [配置来源](examples/configuration/setting_sources/main.go) — 配置源设置
- [类型安全访问器](examples/configuration/type_safe_accessors/main.go) — 类型安全消息处理
- [可配置通道](examples/configuration/configurable_channels/main.go) — 通道容量配置

### MCP 示例
- [MCP 计算器](examples/mcp/mcp_calculator/main.go) — SDK MCP 服务器
- [装饰器风格工具](examples/mcp/decorator_style_tools/main.go) — Python @tool 风格 API

### Hook 示例
- [基础 Hook](examples/hooks/with_hooks/main.go) — Hook 基本用法
- [全部 Hook 事件](examples/hooks/comprehensive_hooks/main.go) — 12 种 Hook 事件演示

### 权限示例
- [权限模式](examples/permissions/with_permissions/main.go) — 各种权限模式
- [权限回调](examples/permissions/tool_permission_callback/main.go) — 自定义权限逻辑
- [完整权限回调](examples/permissions/permission_callback_complete/main.go) — 完整上下文、审计日志、输入重写、拒绝/中断

### 工具拦截示例
- [Tool Handler](examples/tool_handler/main.go) — 回调和事件流两种模式

### 流式示例
- [流式模式](examples/streaming/streaming_mode/main.go) — 基础流式
- [流式对话](examples/streaming/streaming_mode_conversation/main.go) — 多轮流式
- [高级流式](examples/streaming/streaming_mode_comprehensive/main.go) — 高级流式

### 工具示例
- [部分消息](examples/utilities/include_partial_messages/main.go) — 部分消息更新
- [Stderr 回调](examples/utilities/stderr_callback/main.go) — CLI stderr 处理

### 高级示例
- [自定义 Agent](examples/advanced/agents/main.go) — Agent 定义
- [插件系统](examples/advanced/plugin_example/main.go) — 插件示例
- [Python 等价](examples/python_equivalence/main.go) — Python SDK API 对比

## 文档

- [README.md](README.md) — 主文档（英文）
- [Python SDK 对齐](docs/python-sdk-alignment.md) — 完整功能对比和迁移指南
- [功能清单](docs/feature-checklist.md) — 全部 257 项功能实现状态
- [快速参考](docs/quick-reference.md) — 常用操作速查
- [并发指南](docs/concurrency-guide.md) — 并发模式和最佳实践
- [设计决策](docs/design-decisions.md) — 架构决策和原理
- [架构](docs/architecture.md) — 系统架构概览
- [测试策略](docs/testing-strategy.md) — 测试方法和覆盖率
- [变更日志](docs/CHANGELOG.md) — 版本历史和重要变更
- [贡献指南](docs/contributing.md) — 如何参与贡献

## 许可证

本项目使用 MIT 许可证 — 详见 [LICENSE](LICENSE) 文件。

## 致谢

> 本项目基于 [claude-agent-sdk-go](https://github.com/schlunsen/claude-agent-sdk-go) 的原始工作。
