# Eino - 让系统具备工程化扩展能力

## 前言

在大型 AI 应用开发中，工程化能力是保障系统稳定、可维护、可扩展的关键。Eino 框架通过一系列设计模式，让开发者能够轻松扩展系统能力。本篇文章将基于 lab03/callback/option_callback.go 的实现，详细解析如何利用 Option 模式、Callback 机制以及 Go 高阶语法，构建具备工程化扩展能力的 AI 应用。

## 一、设计原则

### 1.1 组合优于继承

在传统面向对象设计中，我们常用继承来扩展功能。但在大模型应用中，模型类型繁多，继承层次会变得复杂且脆弱。

Eino 框架采用了**组合优于继承**的设计理念：

```go
type WrappedChatModel struct {
    inner     *openai.ChatModel      // 组合：封装内部模型
    modelName string
    callbacks []ChatCallback          // 组合：注入回调机制
}
```

**优势：**
- 可以包装任意实现了 `ChatModel` 接口的类型
- 回调机制可以动态添加/移除
- 功能可以自由组合

### 1.2 依赖注入

通过构造函数将依赖注入，而不是在内部创建：

```go
func NewWrappedChatModel(cfg *Config, callbacks ...ChatCallback) (*WrappedChatModel, error) {
    // 依赖通过参数传入，而非硬编码
    inner, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
        APIKey:  cfg.Model.APIKey,
        Model:   cfg.Model.ModelName,
        BaseURL: cfg.Model.BaseURL,
    })
    // ...
}
```

**优势：**
- 便于单元测试（可以注入 mock 对象）
- 配置与代码分离
- 灵活性高

### 1.3 开放封闭原则

系统对扩展开放，对修改封闭。通过 Option 和 Callback 机制，无需修改核心代码即可扩展功能。

```go
// 添加新功能只需定义新的 Option 或 Callback
_, err = w.Generate(ctx, msgs,
    WithRetryCount(2),      // 新增：重试次数
    WithTimeout(60*time.Second), // 新增：超时控制
)
```

## 二、Option 模式实现

### 2.1 为什么需要 Option 模式

Go 语言的函数不支持默认参数和命名参数，当函数参数过多时，调用变得不友好：

```go
// 不好的设计：参数过多，调用困难
func NewChatModel(apiKey, model, baseURL string, timeout int, temperature float32, maxTokens int, topP float32, ...)

// 好的设计：使用 Option 模式
func NewChatModel(apiKey, model, baseURL string, opts ...Option)
chatModel, _ := NewChatModel("key", "gpt-4",
    WithTimeout(30*time.Second),
    WithTemperature(0.7),
)
```

### 2.2 Eino 的 Option 结构

```go
type Option struct {
    // Has unexported fields.
}
```

Option 是 Eino 封装的不可变对象，通过 `WrapImplSpecificOptFn` 函数创建：

```go
func WithRetryCount(count int) model.Option {
    return model.WrapImplSpecificOptFn(func(o *MyChatModelOptions) {
        o.RetryCount = count
    })
}
```

### 2.3 自定义选项实现

```go
// 1. 定义自定义选项结构
type MyChatModelOptions struct {
    RetryCount int
    Timeout    time.Duration
}

// 2. 创建 Option 构造函数
func WithRetryCount(count int) model.Option {
    return model.WrapImplSpecificOptFn(func(o *MyChatModelOptions) {
        o.RetryCount = count
    })
}

func WithTimeout(timeout time.Duration) model.Option {
    return model.WrapImplSpecificOptFn(func(o *MyChatModelOptions) {
        o.Timeout = timeout
    })
}
```

### 2.4 解析 Option

Eino 提供了公开 API 来解析选项：

```go
// 解析通用选项（Temperature, MaxTokens 等标准选项）
commonOpts := model.GetCommonOptions(nil, opts...)

// 解析实现特定选项（自定义选项）
myOpts := model.GetImplSpecificOptions(&MyChatModelOptions{}, opts...)
```

完整使用示例：

```go
func (m *WrappedChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
    // 解析选项
    myOpts := model.GetImplSpecificOptions(&MyChatModelOptions{}, opts...)

    // 使用选项
    if myOpts.Timeout > 0 {
        ctx, cancel = context.WithTimeout(ctx, myOpts.Timeout)
        defer cancel()
    }
    // ...
}
```

## 三、Callback 机制实现

### 3.1 Callback 核心思想

Callback（回调）是一种通知机制，允许在特定事件发生时执行自定义逻辑。在大模型调用中，我们需要在：
- **开始时**：记录日志、收集请求信息
- **结束时**：记录响应、处理异常、统计性能

### 3.2 定义 Callback 接口

```go
// 输入信息
type CallbackInput struct {
    Model    string
    Messages []*schema.Message
    Extra    map[string]any  // 扩展信息
}

// 输出信息
type CallbackOutput struct {
    Message *schema.Message
    Usage   *schema.TokenUsage
    Extra   map[string]any
}

// Callback 接口定义
type ChatCallback interface {
    OnStart(ctx context.Context, in CallbackInput)
    OnEnd(ctx context.Context, out CallbackOutput, err error)
}
```

### 3.3 实现 LoggingCallback

```go
type LoggingCallback struct{}

func (LoggingCallback) OnStart(ctx context.Context, in CallbackInput) {
    fmt.Println("== [callback] start ==")
    fmt.Println("model:", in.Model)
    for i, m := range in.Messages {
        fmt.Printf("  [%d] role=%s content=%q\n", i, m.Role, m.Content)
    }
    if len(in.Extra) > 0 {
        fmt.Println("extra:", in.Extra)
    }
}

func (LoggingCallback) OnEnd(ctx context.Context, out CallbackOutput, err error) {
    fmt.Println("== [callback] end ==")
    if err != nil {
        fmt.Println("error:", err)
        return
    }
    if out.Message != nil {
        fmt.Println("assistant:", out.Message.Content)
    }
    if out.Usage != nil {
        fmt.Printf("usage: prompt=%d completion=%d total=%d\n",
            out.Usage.PromptTokens, out.Usage.CompletionTokens, out.Usage.TotalTokens)
    }
}
```

### 3.4 在 Generate 中集成 Callback

```go
func (m *WrappedChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
    // 1. 解析选项
    myOpts := model.GetImplSpecificOptions(&MyChatModelOptions{}, opts...)

    // 2. 构建 callback 输入
    in := CallbackInput{
        Model:    m.modelName,
        Messages: messages,
        Extra: map[string]any{
            "retry_count": myOpts.RetryCount,
            "timeout":     myOpts.Timeout.String(),
        },
    }

    // 3. 执行 OnStart 回调
    for _, cb := range m.callbacks {
        cb.OnStart(ctx, in)
    }

    // 4. 执行业务逻辑（重试生成）
    // ... 生成逻辑 ...

    // 5. 构建 callback 输出
    out := CallbackOutput{
        Message: resp,
        Usage:   usage,
        Extra: map[string]any{
            "latency_ms": time.Since(start).Milliseconds(),
        },
    }

    // 6. 执行 OnEnd 回调
    for _, cb := range m.callbacks {
        cb.OnEnd(ctx, out, lastErr)
    }

    return resp, nil
}
```

### 3.5 Callback 执行流程

```
┌─────────────────────────────────────────────────────────┐
│                      Generate                            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   ┌─────────────┐                                       │
│   │  OnStart   │ ◀── 遍历所有 callbacks                │
│   │  callbacks │     (LoggingCallback 等)              │
│   └──────┬──────┘                                       │
│          │                                              │
│          ▼                                              │
│   ┌─────────────┐     ┌─────────────┐                  │
│   │   执行     │ ──▶ │   失败?    │──▶ 退避重试        │
│   │  Generate  │     └─────────────┘     │             │
│   └──────┬─────┘                         │             │
│          │成功                           │             │
│          ▼                              │             │
│   ┌─────────────┐                        │             │
│   │   OnEnd    │ ◀── 遍历所有 callbacks  │             │
│   │  callbacks │     (带错误信息)         └─────────────┘
│   └─────────────┘
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## 四、Go 高阶语法应用

### 4.1 泛型 `WrapImplSpecificOptFn`

```go
func WrapImplSpecificOptFn[T any](optFn func(*T)) Option
```

这是 Go 1.18+ 的泛型应用。`[T any]` 表示 T 可以是任意类型：

```go
// 使用示例
func WithRetryCount(count int) model.Option {
    return model.WrapImplSpecificOptFn(func(o *MyChatModelOptions) {
        o.RetryCount = count
    })
}
```

**泛型优势：**
- 编译时类型检查，安全可靠
- 无需反射，性能更好
- 代码复用性高

### 4.2 闭包

Option 函数返回的 `func(o *MyChatModelOptions)` 是一个闭包：

```go
func WithRetryCount(count int) model.Option {
    // count 是自由变量，被闭包捕获
    return model.WrapImplSpecificOptFn(func(o *MyChatModelOptions) {
        o.RetryCount = count  // 闭包使用外部变量
    })
}
```

**闭包特点：**
- 可以访问外部函数作用域的变量
- 变量是被捕获的引用，而非副本
- 常用于回调和异步场景

### 4.3 可变参数 `...`

```go
func NewWrappedChatModel(cfg *Config, callbacks ...ChatCallback) (*WrappedChatModel, error)
```

`callbacks ...ChatCallback` 表示可以接收零个或多个 `ChatCallback` 参数：

```go
// 调用方式灵活
w1 := NewWrappedChatModel(cfg)                        // 无 callback
w2 := NewWrappedChatModel(cfg, LoggingCallback{})      // 一个 callback
w3 := NewWrappedChatModel(cfg, cb1, cb2, cb3)         // 多个 callbacks
```

### 4.4 defer 与资源管理

```go
if myOpts.Timeout > 0 {
    callCtx, cancel = context.WithTimeout(ctx, myOpts.Timeout)
    defer cancel()  // 确保超时后取消 context
}
```

**defer 特点：**
- 在函数返回前执行
- 即使发生 panic 也会执行
- 常用于资源清理（关闭文件、释放锁、取消 context）

### 4.5 错误处理与 sentinel error

```go
if errors.Is(callCtx.Err(), context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.Canceled) {
    break
}
```

**错误处理模式：**
- 使用 `errors.Is` 判断错误类型
- 保留错误链：`fmt.Errorf("重试失败: %w", err)`
- 区分不同错误类型采取不同策略

### 4.6 切片与动态功能扩展

```go
type WrappedChatModel struct {
    callbacks []ChatCallback  // 切片支持动态添加
}

// 支持多个 callback
for _, cb := range m.callbacks {
    cb.OnStart(ctx, in)
}
```

**切片优势：**
- 长度可变，零到多个
- 可以组合多个 callback
- 顺序执行，互不影响

### 4.7 结构体标签与 YAML 解析

```go
type ModelConfig struct {
    BaseURL    string  `yaml:"base_url"`
    APIKey     string  `yaml:"api_key"`
    ModelName  string  `yaml:"model_name"`
    Timeout    int     `yaml:"timeout"`
    Temperature float64 `yaml:"temperature"`
}
```

**结构体标签用途：**
- `yaml:"base_url"` 指定 YAML 字段映射
- 编译时检查字段对应关系
- 解析结果自动填充到结构体

## 五、完整代码解析

### 5.1 代码结构总览

```
lab03/callback/
├── Config / ModelConfig / AppConfig  # 配置结构体
├── loadConfig()                       # 配置加载
├── MyChatModelOptions                # 自定义选项结构
├── WithRetryCount / WithTimeout       # Option 构造函数
├── CallbackInput / CallbackOutput     # Callback 数据结构
├── ChatCallback interface            # Callback 接口
├── LoggingCallback                   # Callback 实现
├── WrappedChatModel                  # 封装模型
│   ├── NewWrappedChatModel()        # 构造函数
│   └── Generate()                    # 生成方法（含重试逻辑）
└── main()                            # 入口
```

### 5.2 核心流程

```go
func main() {
    // 1. 加载配置
    cfg, _ := loadConfig("config.yml")

    // 2. 创建封装模型（注入 callback）
    w, _ := NewWrappedChatModel(cfg, LoggingCallback{})

    // 3. 调用生成（传入 options）
    w.Generate(ctx, msgs,
        WithRetryCount(2),
        WithTimeout(60*time.Second),
    )
}
```

### 5.3 重试机制

```go
for attempt := 0; attempt <= retries; attempt++ {
    r, err := m.inner.Generate(callCtx, messages)
    if err == nil {
        resp = r
        break
    }
    lastErr = err

    // Context 取消/超时则停止重试
    if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
        break
    }

    // 指数退避
    time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
}
```

## 六、最佳实践总结

### 6.1 Option 模式最佳实践

| 实践 | 说明 |
|------|------|
| 不可变性 | Option 创建后不可修改 |
| 命名规范 | `With` 前缀，如 `WithTimeout` |
| 合理默认值 | 在结构体中设置默认值 |
| 文档注释 | 说明参数含义和取值范围 |

### 6.2 Callback 机制最佳实践

| 实践 | 说明 |
|------|------|
| 单一职责 | 每个 Callback 只做一件事 |
| 错误处理 | OnEnd 中正确处理 err |
| 性能意识 | 避免在 callback 中做耗时操作 |
| 扩展性 | 使用接口而非具体类型 |

### 6.3 工程化建议

- **配置外置**：敏感信息和环境差异通过配置文件管理
- **错误分类**：区分可重试错误和不可重试错误
- **日志规范**：记录足够上下文便于排查问题
- **超时控制**：防止无限等待，合理设置超时时间
- **资源清理**：使用 defer 确保资源释放

## 七、扩展阅读

### 7.1 更多 Callback 场景

| 场景 | 实现方式 |
|------|---------|
| 埋点统计 | 在 OnEnd 中上报数据 |
| 敏感词过滤 | 在 OnStart 中检查输入 |
| 速率限制 | 在 OnStart 中检查配额 |
| 链路追踪 | 在 OnStart/OnEnd 中记录 trace |

### 7.2 相关 Go 语法

- [Go 泛型入门](https://go.dev/blog/intro-generics)
- [Go 错误处理](https://go.dev/blog/error-handling)
- [Go Context 模式](https://go.dev/blog/context)

## 八、运行示例

```bash
# 运行代码
go run option_callback.go -config ../config.yml
```

**输出示例：**

```
配置加载成功: base_url=https://api.minimaxi.com/v1, model=MiniMax-M2.7
== [callback] start ==
model: MiniMax-M2.7
  [0] role=system content="你是一个简洁、专业的篮球教练。"
  [1] role=user content="我每周打球2次，想提升运球和终结，请给我一周训练计划。"
extra: map[retry_count:2 timeout:60s]
== [callback] end ==
assistant: (AI 回复内容)
usage: prompt=39 completion=500 total=539
extra: map[latency_ms:12067]
```

通过本文的学习，你应该掌握了：
1. **设计原则**：组合优于继承、依赖注入、开放封闭
2. **Option 模式**：自定义选项、灵活配置
3. **Callback 机制**：生命周期钩子、扩展功能
4. **Go 高阶语法**：泛型、闭包、可变参数、defer、错误处理

这些技术组合在一起，让 Eino 框架具备了强大的工程化扩展能力。
