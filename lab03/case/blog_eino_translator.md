# Eino - 翻译助手实现

## 前言

翻译助手是 AI 应用中的典型场景之一。本文将基于 lab03/case/tran_assistant.go 的实现，详细解析如何利用 Eino 框架构建一个功能完善的翻译助手应用。

## 一、项目概述

### 1.1 功能特性

翻译助手具备以下核心功能：
- **多语言翻译**：支持任意目标语言的翻译
- **格式保留**：保持原文的换行和列表格式
- **结果纯净**：只输出译文，不添加引号或解释
- **错误重试**：网络波动时自动重试
- **超时控制**：防止无限等待

### 1.2 代码结构

```
tran_assistant.go
├── Config / ModelConfig / AppConfig   # 配置结构体
├── loadConfig()                      # 配置加载函数
├── Translator                        # 翻译器结构体
│   ├── chatModel                    # 内部聊天模型
│   ├── NewTranslator()              # 构造函数
│   └── Translate()                  # 翻译方法
└── main()                           # 程序入口
```

## 二、设计原则

### 2.1 面向失败设计

好的翻译器必须考虑各种失败情况：

```go
func (t *Translator) Translate(ctx context.Context, text, targetLang string) (string, error) {
    // 1. 输入验证
    text = strings.TrimSpace(text)
    if text == "" {
        return "", errors.New("empty text")
    }
    if targetLang == "" {
        return "", errors.New("empty target language")
    }

    // 2. 超时控制
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // 3. 重试机制
    var lastErr error
    for attempt := 0; attempt <= 2; attempt++ {
        resp, err := t.chatModel.Generate(ctx, messages)
        if err == nil {
            return strings.TrimSpace(resp.Content), nil
        }
        lastErr = err

        // Context 取消则停止重试
        if errors.Is(ctx.Err(), context.DeadlineExceeded) {
            break
        }

        // 退避重试
        time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
    }
    return "", lastErr
}
```

### 2.2 配置外部化

将配置与代码分离，便于不同环境使用：

```go
// main 函数中
configPath := flag.String("config", "config.yml", "配置文件路径")
flag.Parse()

cfg, err := loadConfig(*configPath)

// 创建翻译器时使用配置
translator, err := NewTranslator(TranslatorConfig{
    APIKey:  cfg.Model.APIKey,
    Model:   cfg.Model.ModelName,
    BaseURL: cfg.Model.BaseURL,
    Timeout: time.Duration(cfg.Model.Timeout) * time.Second,
    Retries: 2,
})
```

### 2.3 构造函数模式

通过构造函数封装复杂的创建逻辑：

```go
func NewTranslator(cfg TranslatorConfig) (*Translator, error) {
    // 参数校验
    if strings.TrimSpace(cfg.APIKey) == "" {
        return nil, errors.New("missing api key")
    }

    // 默认值设置
    if cfg.Model == "" {
        cfg.Model = "gpt-4"
    }
    if cfg.BaseURL == "" {
        cfg.BaseURL = "https://api.openai.com/v1"
    }
    if cfg.Timeout <= 0 {
        cfg.Timeout = 30 * time.Second
    }
    if cfg.Retries < 0 {
        cfg.Retries = 0
    }

    // 创建内部模型
    chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
        APIKey:  cfg.APIKey,
        Model:   cfg.Model,
        BaseURL: cfg.BaseURL,
        Timeout: cfg.Timeout,
    })
    if err != nil {
        return nil, err
    }

    return &Translator{chatModel: chatModel}, nil
}
```

**构造函数的优势：**
- 确保对象在使用前处于有效状态
- 封装复杂的创建逻辑
- 返回错误而非 panic
- 可复用，支持多种配置方式

### 2.4 提示词工程

通过精心设计的提示词控制输出格式：

```go
system := fmt.Sprintf(
    "你是一个专业翻译引擎。将用户输入翻译成%s。"+
        "要求：只输出译文，不要解释；保留原有换行与列表格式；不要添加引号；不要输出多余内容。",
    targetLang,
)

messages := []*schema.Message{
    schema.SystemMessage(system),
    schema.UserMessage(text),
}
```

**提示词设计要点：**
| 要点 | 说明 |
|------|------|
| 角色设定 | 明确 AI 扮演的角色 |
| 任务描述 | 具体说明要做什么 |
| 格式要求 | 明确输出格式 |
| 禁止项 | 说明不要做什么 |

## 三、Go 高阶语法应用

### 3.1 结构体标签与 YAML 解析

```go
type ModelConfig struct {
    BaseURL    string  `yaml:"base_url"`
    APIKey     string  `yaml:"api_key"`
    ModelName  string  `yaml:"model_name"`
    Timeout    int     `yaml:"timeout"`
    Temperature float64 `yaml:"temperature"`
    TopP       float64 `yaml:"top_p"`
    MaxTokens  int     `yaml:"max_tokens"`
}
```

**结构体标签的作用：**
- `yaml:"base_url"` 指定 YAML 中的字段名映射
- 编译时验证字段对应关系
- `yaml.Unmarshal` 自动将 YAML 数据绑定到结构体

```go
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

### 3.2 匿名函数与闭包

虽然本代码中没有直接使用匿名函数，但 Eino 框架的 Option 模式大量使用了闭包：

```go
// 闭包示例（来自 eino 框架）
func WithTemperature(temp float32) model.Option {
    return model.WrapImplSpecificOptFn(func(o *Options) {
        o.Temperature = &temp
    })
}
```

**闭包特性：**
- 可以访问定义时作用域内的变量
- 变量是被捕获的引用，而非副本
- 常用于回调和延迟执行场景

### 3.3 错误包装

使用 `%w` 进行错误包装，保留错误链：

```go
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

**错误链的优势：**
- 保留完整的错误上下文
- 可以使用 `errors.Is` / `errors.As` 判断错误类型
- 便于调试和日志记录

### 3.4 defer 与资源清理

```go
func (t *Translator) Translate(ctx context.Context, text, targetLang string) (string, error) {
    // 超时控制
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()  // 确保函数返回前取消 context

    // ... 业务逻辑 ...
}
```

**defer 的执行时机：**
- 无论函数正常返回还是发生错误，都会执行
- 按照 LIFO（后进先出）顺序执行
- 用于释放资源、关闭连接、取消操作等

### 3.5 切片与匿名结构体

使用匿名结构体和切片定义测试用例：

```go
tests := []struct {
    content string
    target  string
}{
    {"Hello, how are you?", "中文"},
    {"Eino is a powerful AI development framework", "中文"},
    {"Les roses sont rouges", "中文"},
    {"- item1\n- item2\n", "中文"},
}

for _, item := range tests {
    result, err := translator.Translate(context.Background(), item.content, item.target)
    // ...
}
```

**匿名结构体优势：**
- 无需预先定义结构体类型
- 适合临时使用的数据集合
- 提高代码可读性

### 3.6 strings 包常用方法

```go
// TrimSpace - 去除首尾空白
text = strings.TrimSpace(text)

// TrimSpace - 也用于输入验证
if strings.TrimSpace(cfg.APIKey) == "" {
    return nil, errors.New("missing api key")
}
```

### 3.7 time.Duration 类型

```go
// time.Duration 是 time 包定义的类型，本质是 int64
Timeout time.Duration

// 乘法运算需要使用 time 包常量
time.Duration(cfg.Model.Timeout) * time.Second

// 延迟时间
time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
```

**time.Duration 常用单位：**
| 常量 | 含义 |
|------|------|
| time.Second | 秒 |
| time.Millisecond | 毫秒 |
| time.Minute | 分钟 |
| time.Hour | 小时 |

### 3.8 context 与超时控制

```go
// 创建带超时的 context
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

// 判断是否超时
if errors.Is(ctx.Err(), context.DeadlineExceeded) {
    break
}
```

**context 的三大作用：**
| 作用 | 说明 |
|------|------|
| 截止时间 | 通过 `WithTimeout` 设置 |
| 取消信号 | 通过 `WithCancel` 设置 |
| 传递数据 | 通过 `WithValue` 传递 |

### 3.9 errors.Is 错误判断

```go
if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
    break
}
```

**errors.Is vs 直接比较：**
```go
// 不推荐：直接比较可能失败（错误可能被包装）
if err == context.DeadlineExceeded { ... }

// 推荐：errors.Is 可以穿透包装
if errors.Is(err, context.DeadlineExceeded) { ... }
```

### 3.10 fmt.Sprintf 格式化字符串

```go
system := fmt.Sprintf(
    "你是一个专业翻译引擎。将用户输入翻译成%s。"+
        "要求：只输出译文，不要解释；保留原有换行与列表格式；不要添加引号；不要输出多余内容。",
    targetLang,
)
```

**fmt.Sprintf 常用格式化：**
| 占位符 | 说明 |
|--------|------|
| %s | 字符串 |
| %d | 整数 |
| %f | 浮点数 |
| %v | 任意值 |
| %q | 带引号字符串 |

## 四、完整代码解析

### 4.1 配置结构体

```go
// Config 顶层配置
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
```

### 4.2 翻译器结构体

```go
type Translator struct {
    chatModel *openai.ChatModel  // 组合大模型
}
```

### 4.3 翻译流程

```
Translate(text, targetLang)
    │
    ├── 1. 输入验证
    │       ├── TrimSpace
    │       └── 检查空值
    │
    ├── 2. 创建超时 Context
    │       ├── WithTimeout
    │       └── defer cancel
    │
    ├── 3. 构建提示词
    │       ├── SystemMessage (翻译规则)
    │       └── UserMessage (待翻译文本)
    │
    ├── 4. 调用大模型（带重试）
    │       ├── Generate
    │       ├── 失败 → 退避重试
    │       └── 成功 → 返回结果
    │
    └── 5. 返回结果
            └── TrimSpace 清理
```

### 4.4 main 函数流程

```go
func main() {
    // 1. 解析命令行参数
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()

    // 2. 加载配置
    cfg, err := loadConfig(*configPath)

    // 3. 创建翻译器
    translator, err := NewTranslator(TranslatorConfig{
        APIKey:  cfg.Model.APIKey,
        Model:   cfg.Model.ModelName,
        BaseURL: cfg.Model.BaseURL,
        Timeout: time.Duration(cfg.Model.Timeout) * time.Second,
        Retries: 2,
    })

    // 4. 执行翻译测试
    tests := []struct {
        content string
        target  string
    }{
        {"Hello, how are you?", "中文"},
        {"Eino is a powerful AI development framework", "中文"},
        // ...
    }

    for _, item := range tests {
        result, err := translator.Translate(ctx, item.content, item.target)
        // 处理结果
    }
}
```

## 五、运行与测试

### 5.1 配置文件

```yaml
model:
  base_url: "https://api.minimaxi.com/v1"
  api_key: "your-api-key"
  model_name: "MiniMax-M2.7"
  timeout: 30
  temperature: 0.7
  top_p: 0.9
  max_tokens: 500

app:
  host: "0.0.0.0"
  port: 8080
```

### 5.2 运行命令

```bash
go run tran_assistant.go -config ../config.yml
```

### 5.3 输出示例

```
配置加载成功: base_url=https://api.minimaxi.com/v1, model=MiniMax-M2.7
原文: Hello, how are you?
翻译: 你好，你好吗？

原文: Eino is a powerful AI development framework
翻译: Eino 是一个强大的 AI 开发框架

原文: Les roses sont rouges
翻译: 玫瑰是红色的

原文: - item1
- item2
翻译: - 项目1
- 项目2
```

## 六、最佳实践总结

### 6.1 输入验证

| 检查项 | 处理方式 |
|--------|----------|
| 空文本 | 返回明确错误 |
| 空目标语言 | 返回明确错误 |
| 首位空白 | TrimSpace 处理 |
| API Key 缺失 | 构造函数返回错误 |

### 6.2 错误处理

- 构造函数返回错误，而非 panic
- 使用 errors.Is 判断错误类型
- 保留错误链，便于排查

### 6.3 重试策略

```go
for attempt := 0; attempt <= 2; attempt++ {
    resp, err := t.chatModel.Generate(ctx, messages)
    if err == nil {
        return resp.Content, nil
    }

    // Context 取消则停止
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        break
    }

    // 指数退避
    time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
}
```

### 6.4 提示词设计原则

1. **明确角色**：指定 AI 扮演的角色
2. **具体任务**：清晰描述翻译任务
3. **格式要求**：明确输出格式要求
4. **禁止项**：说明不要做什么

## 七、扩展方向

### 7.1 支持更多翻译模式

```go
// 流式翻译
func (t *Translator) TranslateStream(ctx context.Context, text, targetLang string) (*Stream, error)

// 批量翻译
func (t *Translator) TranslateBatch(ctx context.Context, texts []string, targetLang string) ([]string, error)
```

### 7.2 添加 Callback 支持

参考 lab03/callback/option_callback.go，添加日志、埋点等功能。

### 7.3 支持更多模型

通过接口抽象，支持 Deepseek、Claude 等多种模型：

```go
type TranslatorModel interface {
    Generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
}
```

通过本文的学习，你应该掌握了：
1. **设计原则**：面向失败设计、配置外部化、构造函数模式、提示词工程
2. **Go 高阶语法**：结构体标签、错误包装、defer、context、超时控制
3. **工程实践**：输入验证、错误处理、重试策略
