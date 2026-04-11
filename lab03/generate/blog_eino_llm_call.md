# Eino - 从0到1跑通大模型调用

## 前言

Eino 是字节跳动开源的 AI 应用开发框架，提供了丰富的大模型组件支持。本文将基于 lab03 的代码示例，详细介绍如何使用 Eino 框架实现单轮对话、多轮对话、流式输出以及模型参数配置，帮助你从零开始掌握大模型调用。

## 一、环境准备

### 1.1 安装依赖

```bash
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext/components/model/openai@latest
go get gopkg.in/yaml.v2@latest
```

### 1.2 配置文件

创建 `config.yml` 配置文件：

```yaml
model:
  base_url: "https://api.minimaxi.com/v1"
  api_key: "your-api-key"
  model_name: "MiniMax-M2.7"
  timeout: 30          # 超时时间(秒)
  temperature: 0.7     # 控制输出随机性，范围 [0.0, 2.0]
  top_p: 0.9          # 核采样参数，范围 [0.0, 1.0]
  max_tokens: 500     # 最大生成token数量

app:
  host: "0.0.0.0"
  port: 8080
```

## 二、统一配置管理

为了代码复用，我们定义统一的配置结构体：

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
```

通过命令行参数指定配置文件路径：

```go
configPath := flag.String("config", "config.yml", "配置文件路径")
flag.Parse()
```

## 三、单轮对话实现

单轮对话是最基础的场景，用户发送一条消息，AI 返回一条响应。

### 3.1 完整代码

```go
func main() {
    // 0. 解析命令行参数
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()

    // 1. 加载配置
    cfg, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    ctx := context.Background()

    // 2. 创建 ChatModel
    chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey:  cfg.Model.APIKey,
        Model:   cfg.Model.ModelName,
        BaseURL: cfg.Model.BaseURL,
    })
    if err != nil {
        log.Fatalf("创建失败: %v", err)
    }

    // 3. 构建消息
    messages := []*schema.Message{
        schema.SystemMessage("你是一个懂得哲学的程序员。"),
        schema.UserMessage("什么是存在主义？"),
    }

    // 4. 生成响应
    response, err := chatModel.Generate(ctx, messages)
    if err != nil {
        log.Fatalf("生成失败: %v", err)
    }

    // 5. 输出结果
    fmt.Printf("回答:\n%s\n", response.Content)

    // 6. 输出 Token 使用统计
    if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
        fmt.Printf("\nToken 使用统计:\n")
        fmt.Printf("  输入 Token: %d\n", response.ResponseMeta.Usage.PromptTokens)
        fmt.Printf("  输出 Token: %d\n", response.ResponseMeta.Usage.CompletionTokens)
        fmt.Printf("  总计 Token: %d\n", response.ResponseMeta.Usage.TotalTokens)
    }
}
```

### 3.2 核心步骤解析

| 步骤 | 说明 |
|------|------|
| 1. 加载配置 | 从 YAML 文件读取模型配置 |
| 2. 创建 ChatModel | 初始化大模型客户端 |
| 3. 构建消息 | 使用 `schema.SystemMessage` 和 `schema.UserMessage` 构建对话 |
| 4. 生成响应 | 调用 `chatModel.Generate()` 获取非流式响应 |
| 5. 输出结果 | 打印 AI 回复内容和 Token 使用统计 |

## 四、模型参数配置

不同的应用场景需要不同的参数配置，Eino 提供了丰富的参数支持。

### 4.1 完整代码

```go
func main() {
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()
    cfg, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    ctx := context.Background()

    // 示例1: 基础配置
    basicExample(ctx, cfg)

    // 示例2: 高级配置
    advancedExample(ctx, cfg)

    // 示例3: 创意写作配置
    creativeExample(ctx, cfg)
}

// 基础配置示例
func basicExample(ctx context.Context, cfg *Config) {
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
    // ...
}

// 高级配置示例 - 精确控制输出
func advancedExample(ctx context.Context, cfg *Config) {
    temperature := float32(0.7)
    topP := float32(0.9)
    maxTokens := 500
    presencePenalty := float32(0.6)
    frequencyPenalty := float32(0.5)

    chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey:  cfg.Model.APIKey,
        Model:   cfg.Model.ModelName,
        BaseURL: cfg.Model.BaseURL,
        Timeout: 30 * time.Second,

        // 生成参数
        Temperature: &temperature,
        TopP:        &topP,
        MaxTokens:   &maxTokens,

        // 停止序列 - 遇到这些文本时停止生成
        Stop: []string{"\n\n", "总结:"},

        // 惩罚参数 - 控制重复度
        PresencePenalty:  &presencePenalty,
        FrequencyPenalty: &frequencyPenalty,
    })
    // ...
}

// 创意写作配置 - 高随机性
func creativeExample(ctx context.Context, cfg *Config) {
    temperature := float32(1.2)
    topP := float32(0.95)
    maxTokens := 800
    presencePenalty := float32(0.3)
    frequencyPenalty := float32(0.3)

    chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        // 高温度设置，适合创意写作
        Temperature: &temperature,
        TopP:        &topP,
        MaxTokens:   &maxTokens,

        // 减少重复惩罚，允许一定的重复
        PresencePenalty:  &presencePenalty,
        FrequencyPenalty: &frequencyPenalty,
    })
    // ...
}
```

### 4.2 参数详解

| 参数 | 类型 | 说明 | 推荐值 |
|------|------|------|--------|
| Temperature | *float32 | 控制输出随机性，值越高越随机 | 0.7 (基础)、1.2+ (创意) |
| TopP | *float32 | 核采样参数，值越低越聚焦 | 0.9 (基础)、0.95 (创意) |
| MaxTokens | *int | 最大生成 token 数量 | 500 (基础)、800+ (长文本) |
| PresencePenalty | *float32 | 存在惩罚，鼓励新话题 | 0.6 (精确)、0.3 (创意) |
| FrequencyPenalty | *float32 | 频率惩罚，减少重复 | 0.5 (精确)、0.3 (创意) |
| Stop | []string | 停止序列，遇到这些文本停止生成 | - |

### 4.3 场景化配置建议

| 场景 | Temperature | TopP | MaxTokens | PresencePenalty | FrequencyPenalty |
|------|-------------|------|-----------|-----------------|------------------|
| 基础问答 | 0.7 | 0.9 | 500 | 0.6 | 0.5 |
| 技术文档 | 0.7 | 0.9 | 500 | 0.6 | 0.5 |
| 创意写作 | 1.2+ | 0.95 | 800+ | 0.3 | 0.3 |
| 代码生成 | 0.3-0.5 | 0.9 | 1000+ | 0.6 | 0.5 |

## 五、多轮对话实现

多轮对话需要维护对话历史，将 AI 的回复也加入消息列表，实现上下文理解。

### 5.1 完整代码

```go
func main() {
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()
    cfg, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    ctx := context.Background()

    // 创建 ChatModel
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

    // 对话历史
    messages := []*schema.Message{
        schema.SystemMessage("你是一个懂得哲学的程序员。"),
    }

    scanner := bufio.NewScanner(os.Stdin)
    fmt.Println("开始对话（输入 'exit' 退出）：")

    for {
        fmt.Print("\n你: ")
        if !scanner.Scan() {
            break
        }

        userInput := strings.TrimSpace(scanner.Text())
        if userInput == "exit" {
            fmt.Println("再见！")
            break
        }

        if userInput == "" {
            continue
        }

        // 添加用户消息
        messages = append(messages, schema.UserMessage(userInput))

        // 生成 AI 响应
        response, err := chatModel.Generate(ctx, messages)
        if err != nil {
            log.Printf("生成失败: %v", err)
            continue
        }

        // 添加 AI 响应到历史，实现多轮对话
        messages = append(messages, response)

        fmt.Printf("\nAI: %s\n", response.Content)
    }
}
```

### 5.2 核心逻辑

```
┌─────────────────────────────────────────────────────┐
│                    对话循环                          │
├─────────────────────────────────────────────────────┤
│                                                     │
│   用户输入 ──→ 添加到 messages ──→ chatModel.Generate │
│                                                     │
│         ▲                                          │
│         │                                          │
│         │ append(response)                         │
│         │                                          │
│   AI 响应 ◀── messages ◀────────────────────────────┤
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 5.3 关键点

1. **对话历史累积**：每次对话后，将用户消息和 AI 响应都加入 `messages` 列表
2. **SystemMessage 保持不变**：系统提示只在列表开头，不重复添加
3. **上下文理解**：通过传递完整对话历史，AI 能够理解上下文

## 六、流式输出实现

流式输出让 AI 的回复可以逐字显示，带来更好的用户体验。

### 6.1 完整代码

```go
func main() {
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()
    cfg, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    ctx := context.Background()

    // 创建 ChatModel
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

    // 构建消息
    messages := []*schema.Message{
        schema.SystemMessage("你是一个懂得哲学的程序员。"),
        schema.UserMessage("什么是存在主义？"),
    }

    // 流式生成
    stream, err := chatModel.Stream(ctx, messages)
    if err != nil {
        log.Fatalf("流式生成失败: %v", err)
    }
    defer stream.Close() // 记得关闭流

    fmt.Print("AI 回复: ")

    // 逐块接收并打印
    for {
        chunk, err := stream.Recv()
        if err != nil {
            if errors.Is(err, io.EOF) {
                // 流结束
                break
            }
            log.Fatalf("接收失败: %v", err)
        }

        // 打印内容（打字机效果）
        fmt.Print(chunk.Content)
    }

    fmt.Println("\n\n完成！")
}
```

### 6.2 流式 vs 非流式

| 特性 | 非流式 (Generate) | 流式 (Stream) |
|------|-------------------|---------------|
| 返回时机 | 完整响应生成后 | 逐 token 返回 |
| 用户体验 | 等待时间长 | 即时响应，打字机效果 |
| 代码复杂度 | 简单 | 需要处理循环接收 |
| 适用场景 | 短回复、需要完整结果 | 长回复、实时交互 |

### 6.3 流式处理流程

```
chatModel.Stream()
       │
       ▼
   stream.Recv() ──→ chunk.Content ──→ fmt.Print()
       │                            (逐字打印)
       │ chunk
       ▼
   stream.Recv()
       │
       │ io.EOF
       ▼
    结束
```

## 七、Token 使用统计

Eino 提供了完整的 Token 使用统计，通过 `response.ResponseMeta.Usage` 获取：

```go
if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
    fmt.Printf("输入 Token: %d\n", response.ResponseMeta.Usage.PromptTokens)
    fmt.Printf("输出 Token: %d\n", response.ResponseMeta.Usage.CompletionTokens)
    fmt.Printf("总计 Token: %d\n", response.ResponseMeta.Usage.TotalTokens)

    // 缓存 Token（如果有）
    if response.ResponseMeta.Usage.PromptTokenDetails.CachedTokens > 0 {
        fmt.Printf("缓存 Token: %d\n", response.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
    }
}
```

## 八、总结

本文详细介绍了使用 Eino 框架调用大模型的四种核心场景：

| 场景 | 方法 | 特点 |
|------|------|------|
| 单轮对话 | `chatModel.Generate()` | 简单直接，一次请求一次响应 |
| 参数配置 | `ChatModelConfig` | 灵活控制输出质量 |
| 多轮对话 | 维护 `messages` 历史 | 上下文理解，连续交互 |
| 流式输出 | `chatModel.Stream()` | 即时响应，打字机效果 |

### 关键配置参数

- **Temperature**：控制随机性，创意场景调高，精确场景调低
- **TopP**：核采样，与 Temperature 二选一使用
- **MaxTokens**：限制输出长度，避免过长回复
- **PresencePenalty / FrequencyPenalty**：控制重复内容

### 运行方式

```bash
# 单轮对话
go run single_chat.go -config ../../config.yml

# 参数配置示例
go run param_conf.go -config ../../config.yml

# 多轮对话
go run multi_chat.go -config ../../config.yml

# 流式输出
go run stream_chat.go -config ../../config.yml
```

通过本文的学习，你应该已经掌握了 Eino 框架调用大模型的核心技能，能够根据不同场景选择合适的调用方式并配置合适的参数。
