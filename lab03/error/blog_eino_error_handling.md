# Eino - 错误处理与稳定性

## 前言

在大模型应用开发中，错误处理是保障系统稳定性的关键环节。网络波动、API 限流、服务端异常等都可能导致请求失败。本篇文章将基于 lab03/error/error_handling.go 的实现，详细介绍如何在 Eino 框架中设计健壮的错误处理机制。

## 一、为什么需要错误处理

### 1.1 大模型调用的不确定性

调用大模型 API 时，可能遇到以下问题：

| 错误类型 | 原因 | 处理策略 |
|---------|------|---------|
| 网络超时 | 网络波动、防火墙 | 重试 |
| API 限流 | 请求频率过高 | 指数退避重试 |
| 服务端异常 | 模型服务不可用 | 重试 |
| 参数错误 | 配置不当 | 检查配置 |
| 超时 | 请求时间过长 | 调整超时设置 |

### 1.2 稳定性的重要性

没有完善的错误处理会导致：
- 用户体验差（突然失败，无反馈）
- 排查困难（不知道哪里出错）
- 系统脆弱（一次失败就崩溃）
- 资源浪费（重试效率低）

## 二、外部配置管理

将配置与代码分离是提高系统可维护性的重要原则。

### 2.1 配置文件结构

```yaml
model:
  base_url: "https://api.minimaxi.com/v1"
  api_key: "your-api-key"
  model_name: "MiniMax-M2.7"
  timeout: 30          # 超时时间(秒)
  temperature: 0.7
  top_p: 0.9
  max_tokens: 500

app:
  host: "0.0.0.0"
  port: 8080
```

### 2.2 配置加载实现

```go
// Config 模型配置
type Config struct {
    Model ModelConfig `yaml:"model"`
    App   AppConfig   `yaml:"app"`
}

// ModelConfig 大模型配置
type ModelConfig struct {
    BaseURL    string  `yaml:"base_url"`
    APIKey     string  `yaml:"api_key"`
    ModelName  string  `yaml:"model_name"`
    Timeout    int     `yaml:"timeout"`
    Temperature float64 `yaml:"temperature"`
    TopP       float64 `yaml:"top_p"`
    MaxTokens  int     `yaml:"max_tokens"`
}

// loadConfig 加载配置文件
func loadConfig(configPath string) (*Config, error) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("读取配置文件失败: %w", err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("解析配置文件失败: %w", err)
    }

    return &config, nil
}
```

### 2.3 命令行参数传入配置路径

```go
func main() {
    // 通过命令行参数指定配置文件路径
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()

    // 加载配置
    cfg, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }
}
```

**优势：**
- 无需重新编译即可修改配置
- 不同环境使用不同配置（开发、测试、生产）
- 敏感信息可通过环境变量或密钥管理服务注入

## 三、重试机制设计

### 3.1 重试函数实现

```go
// generateWithRetry 尝试多次调用 ChatModel 的 Generate 方法，直到成功或达到最大重试次数。
func generateWithRetry(ctx context.Context, chatModel *openai.ChatModel, messages []*schema.Message, maxRetries int) (*schema.Message, error) {
    // 记录最后一次错误
    var lastErr error

    // 重试逻辑
    for i := 0; i < maxRetries; i++ {
        response, err := chatModel.Generate(ctx, messages)
        if err == nil {
            return response, nil
        }

        lastErr = err
        log.Printf("尝试 %d/%d 失败: %v", i+1, maxRetries, err)

        // 指数退避
        if i < maxRetries-1 {
            backoff := time.Duration(1<<uint(i)) * time.Second
            log.Printf("等待 %v 后重试...", backoff)
            time.Sleep(backoff)
        }
    }

    return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
}
```

### 3.2 指数退避策略

重试间隔采用指数退避算法：

```
重试次数 | 退避时间
--------|----------
   1    |   1 秒   (2^0)
   2    |   2 秒   (2^1)
   3    |   4 秒   (2^2)
   ...  |   ...
```

```go
backoff := time.Duration(1<<uint(i)) * time.Second
```

**为什么使用指数退避？**
- 避免频繁重试给服务器增加压力
- 给服务恢复时间
- 网络波动时避免雪崩效应

### 3.3 重试流程图

```
┌─────────────────────────────────────────────────────┐
│                   generateWithRetry                  │
├─────────────────────────────────────────────────────┤
│                                                     │
│   ┌─────────┐    成功    ┌──────────┐               │
│   │  第 i   │ ────────▶ │  返回    │               │
│   │  次调用 │           │  响应    │               │
│   └────┬────┘           └──────────┘               │
│        │失败                                       │
│        ▼                                           │
│   ┌─────────┐                                     │
│   │ 记录错误 │                                     │
│   └────┬────┘                                     │
│        ▼                                           │
│   ┌─────────────┐   是    ┌──────────┐             │
│   │ i < max-1?  │ ──────▶ │  等待    │             │
│   └─────────────┘         │ (指数退避)│             │
│        │否                 └────┬─────┘             │
│        ▼                        │                   │
│   ┌──────────┐                  │                   │
│   │ 返回错误 │◀─────────────────┘                   │
│   │ (包装err)│                                    │
│   └──────────┘                                    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 四、错误类型判断

### 4.1 使用 errors.Is 判断错误类型

```go
response, err := generateWithRetry(ctx, chatModel, messages, 3)
if err != nil {
    // 判断是否为超时错误
    if errors.Is(err, context.DeadlineExceeded) {
        log.Fatalf("请求超时")
    }
    log.Fatalf("生成失败: %v", err)
}
```

### 4.2 常见错误类型

| 错误类型 | 含义 | 处理方式 |
|---------|------|---------|
| `context.DeadlineExceeded` | 请求超时 | 增加超时时间或重试 |
| `context.Canceled` | 请求被取消 | 检查调用方逻辑 |
| `io.EOF` | 流结束 | 正常流程 |
| API 返回错误 | 服务端错误 | 根据错误码处理 |

### 4.3 错误链包装

```go
return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
```

使用 `%w` 包装错误，保留错误链，便于排查：

```
重试 3 次后仍然失败: 调用 ChatModel 失败: API 返回错误: rate limit exceeded
```

## 五、完整代码解析

### 5.1 完整代码

```go
package main

import (
    "context"
    "errors"
    "flag"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/schema"
    "gopkg.in/yaml.v2"
)

// Config 模型配置
type Config struct {
    Model ModelConfig `yaml:"model"`
    App   AppConfig   `yaml:"app"`
}

// ModelConfig 大模型配置
type ModelConfig struct {
    BaseURL    string  `yaml:"base_url"`
    APIKey     string  `yaml:"api_key"`
    ModelName  string  `yaml:"model_name"`
    Timeout    int     `yaml:"timeout"`
    Temperature float64 `yaml:"temperature"`
    TopP       float64 `yaml:"top_p"`
    MaxTokens  int     `yaml:"max_tokens"`
}

// AppConfig 应用配置
type AppConfig struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}

// loadConfig 加载配置文件
func loadConfig(configPath string) (*Config, error) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("读取配置文件失败: %w", err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("解析配置文件失败: %w", err)
    }

    return &config, nil
}

// generateWithRetry 重试机制
func generateWithRetry(ctx context.Context, chatModel *openai.ChatModel, messages []*schema.Message, maxRetries int) (*schema.Message, error) {
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        response, err := chatModel.Generate(ctx, messages)
        if err == nil {
            return response, nil
        }

        lastErr = err
        log.Printf("尝试 %d/%d 失败: %v", i+1, maxRetries, err)

        if i < maxRetries-1 {
            backoff := time.Duration(1<<uint(i)) * time.Second
            log.Printf("等待 %v 后重试...", backoff)
            time.Sleep(backoff)
        }
    }

    return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
}

func main() {
    // 0. 解析命令行参数
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()

    // 1. 加载配置
    cfg, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }
    log.Printf("配置加载成功: base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)

    ctx := context.Background()

    // 2. 创建 ChatModel
    timeout := time.Duration(cfg.Model.Timeout) * time.Second
    if timeout == 0 {
        timeout = 30 * time.Second
    }

    temperature := float32(cfg.Model.Temperature)
    maxTokens := cfg.Model.MaxTokens

    chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey:     cfg.Model.APIKey,
        Model:      cfg.Model.ModelName,
        BaseURL:    cfg.Model.BaseURL,
        Timeout:    timeout,
        Temperature: &temperature,
        MaxTokens:   &maxTokens,
    })
    if err != nil {
        log.Fatalf("创建失败: %v", err)
    }

    messages := []*schema.Message{
        schema.UserMessage("你好"),
    }

    // 3. 带重试的生成
    response, err := generateWithRetry(ctx, chatModel, messages, 3)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            log.Fatalf("请求超时")
        }
        log.Fatalf("生成失败: %v", err)
    }

    fmt.Printf("成功! 回答: %s\n", response.Content)

    // 4. 输出 Token 使用统计
    if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
        fmt.Printf("\nToken 使用统计:\n")
        fmt.Printf("  输入 Token: %d\n", response.ResponseMeta.Usage.PromptTokens)
        fmt.Printf("  输出 Token: %d\n", response.ResponseMeta.Usage.CompletionTokens)
        fmt.Printf("  总计 Token: %d\n", response.ResponseMeta.Usage.TotalTokens)
    }
}
```

### 5.2 代码结构

```
main()
├── 0. 解析命令行参数 (flag)
├── 1. 加载配置 (loadConfig)
├── 2. 创建 ChatModel (openai.NewChatModel)
└── 3. 带重试的生成 (generateWithRetry)
    └── 循环调用 chatModel.Generate
        ├── 成功 → 返回
        └── 失败 → 指数退避 → 重试
```

## 六、超时控制

### 6.1 配置超时

```go
timeout := time.Duration(cfg.Model.Timeout) * time.Second
if timeout == 0 {
    timeout = 30 * time.Second  // 默认 30 秒
}

chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    // ...
    Timeout: timeout,
})
```

### 6.2 Context 超时

更精细的控制可以使用 `context.WithTimeout`：

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

response, err := chatModel.Generate(ctx, messages)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Printf("请求超时")
    }
}
```

## 七、运行与测试

### 7.1 运行命令

```bash
go run error_handling.go -config ../config.yml
```

### 7.2 输出示例

```
2026/04/11 22:16:13 配置加载成功: base_url=https://api.minimaxi.com/v1, model=MiniMax-M2.7
成功! 回答: 你好！有什么我可以帮助你的吗？

Token 使用统计:
  输入 Token: 42
  输出 Token: 30
  总计 Token: 72
```

### 7.3 失败重试日志

当 API 调用失败时，会看到类似日志：

```
2026/04/11 22:16:13 尝试 1/3 失败: context deadline exceeded
2026/04/11 22:16:13 等待 1s 后重试...
2026/04/11 22:16:14 尝试 2/3 失败: context deadline exceeded
2026/04/11 22:16:14 等待 2s 后重试...
2026/04/11 22:16:16 尝试 3/3 失败: context deadline exceeded
2026/04/11 22:16:16 重试 3 次后仍然失败: context deadline exceeded
```

## 八、最佳实践总结

### 8.1 错误处理原则

| 原则 | 说明 |
|------|------|
| 早失败 | 配置错误等尽早检查并退出 |
| 包装错误 | 使用 `fmt.Errorf` 保留错误链 |
| 判断类型 | 使用 `errors.Is` 判断具体错误 |
| 有限重试 | 设置最大重试次数，避免无限循环 |
| 指数退避 | 避免给服务过大压力 |

### 8.2 配置管理建议

- 敏感信息（API Key）通过环境变量或密钥服务注入
- 不同环境使用不同配置文件
- 配置应有默认值，增强容错性

### 8.3 日志规范

- 记录足够的上下文信息（重试次数、错误原因）
- 区分不同级别（log.Printf / log.Fatalf）
- 关键操作前打印日志，便于排查

```go
log.Printf("配置加载成功: base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)
log.Printf("尝试 %d/%d 失败: %v", i+1, maxRetries, err)
```

## 九、扩展阅读

### 9.1 更多重试策略

- **抖动（Jitter）**：在退避时间上添加随机偏移，避免多实例同时重试
- **熔断器（Circuit Breaker）**：连续失败达到阈值后快速失败
- **超时预算**：根据剩余时间动态调整超时

### 9.2 相关资源

- [Eino 官方文档](https://eino.github.io/)
- [Go 错误处理最佳实践](https://go.dev/blog/error-handling)
- [Exponential Backoff And Jitter](https://aws.amazon.com/cn/blogs/architecture/exponential-backoff-and-jitter/)

通过本文的学习，你应该掌握了 Eino 框架中错误处理的核心技巧，能够设计出更加健壮的大模型应用。
