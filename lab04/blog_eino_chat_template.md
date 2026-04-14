# Eino - ChatTemplate 的应用

## 前言

在 AI 应用开发中，Prompt（提示词）是与大模型交互的核心。一个好的提示词工程能够让 AI 理解任务需求并生成高质量的回复。Eino 框架提供了强大的 `ChatTemplate` 功能，支持模板化管理提示词、变量替换、多角色对话等高级特性。

本文将基于 lab04 的四个示例代码，详细介绍 ChatTemplate 的各种应用场景，帮助你掌握提示词模板化的核心技能。

## 一、ChatTemplate 概述

### 1.1 什么是 ChatTemplate

ChatTemplate 是 Eino 框架中用于管理对话消息的模板结构，它允许开发者：
- **模板化**：将提示词以模板形式定义，运行时替换变量
- **多角色支持**：支持 System、User、Assistant 等多种消息类型
- **灵活格式化**：支持 FString 等多种格式化方式

### 1.2 核心概念

```
ChatTemplate
├── prompt.FromMessages()     # 创建模板
├── schema.FString            # 格式化器（类似 Python f-string）
├── template.Format()          # 格式化模板
└── Message Types
    ├── SystemMessage          # 系统消息
    ├── UserMessage            # 用户消息
    └── AssistantMessage       # 助手消息
```

## 二、基础用法：变量替换

### 2.1 简单变量替换

`var_replace.go` 展示了最基础的变量替换功能：

```go
template := prompt.FromMessages(
    schema.FString,
    schema.SystemMessage("你是一个{role}"),
    schema.UserMessage("{question}"),
)

variables := map[string]any{
    "role":     "热爱运动的程序员",
    "question": "运动和工作哪个更重要？",
}

messages, _ := template.Format(ctx, variables)
```

**运行结果：**

```
1. [system] 你是一个热爱运动的程序员。
2. [user] 运动和工作哪个更重要？
```

### 2.2 FString 格式化器

`schema.FString` 是一种类似 Python f-string 的格式化器，使用 `{变量名}` 语法：

```go
schema.SystemMessage("你是一个{role}")  // {role} 会被替换
schema.UserMessage("{question}")        // {question} 会被替换
```

**FString 特点：**
- 语法简洁，类似 Python f-string
- 支持任意数量的变量
- 变量不存在时返回错误

### 2.3 完整代码解析

```go
func main() {
    // 1. 创建模板
    template := prompt.FromMessages(
        schema.FString,
        schema.SystemMessage("你是一个{role}"),
        schema.UserMessage("{question}"),
    )

    // 2. 准备变量
    variables := map[string]any{
        "role":     "热爱运动的程序员",
        "question": "运动和工作哪个更重要？",
    }

    // 3. 格式化消息
    messages, err := template.Format(ctx, variables)
    if err != nil {
        log.Fatalf("格式化失败: %v", err)
    }

    // 4. 查看生成的消息
    for i, msg := range messages {
        fmt.Printf("%d. [%s] %s\n", i+1, msg.Role, msg.Content)
    }
}
```

## 三、模板复用：结构化设计

### 3.1 PromptTemplates 管理器

`var_replace.go` 中的 `PromptTemplates` 结构体展示了如何组织和管理多个模板：

```go
type PromptTemplates struct{}

// 翻译助手模板
func (p *PromptTemplates) Translator(sourceLang, targetLang string) prompt.ChatTemplate {
    return prompt.FromMessages(
        schema.FString,
        schema.SystemMessage(fmt.Sprintf(
            "你是一个专业的翻译助手。请将%s翻译成%s。\n"+
                "要求：\n"+
                "1. 保持原文的语气和风格\n"+
                "2. 确保翻译准确、流畅\n"+
                "3. 只返回翻译结果，不要添加解释",
            sourceLang, targetLang,
        )),
        schema.UserMessage("{text}"),
    )
}
```

### 3.2 多种模板类型

#### 翻译助手模板

```go
func (p *PromptTemplates) Translator(sourceLang, targetLang string) prompt.ChatTemplate {
    return prompt.FromMessages(
        schema.FString,
        schema.SystemMessage(fmt.Sprintf(
            "你是一个专业的翻译助手。请将%s翻译成%s。\n"+
                "要求：\n"+
                "1. 保持原文的语气和风格\n"+
                "2. 确保翻译准确、流畅\n"+
                "3. 只返回翻译结果，不要添加解释",
            sourceLang, targetLang,
        )),
        schema.UserMessage("{text}"),
    )
}

// 使用示例
template := templates.Translator("中文", "英文")
messages := template.Format(ctx, map[string]any{
    "text": "Eino 是一个强大的 AI 开发框架",
})
```

#### 代码审查模板

```go
func (p *PromptTemplates) CodeReviewer(language string) prompt.ChatTemplate {
    return prompt.FromMessages(
        schema.FString,
        schema.SystemMessage(fmt.Sprintf(
            "你是一个资深的%s开发专家。请审查以下代码，并提供：\n"+
                "1. 潜在的bug或问题\n"+
                "2. 性能优化建议\n"+
                "3. 代码风格改进建议\n"+
                "4. 安全性评估",
            language,
        )),
        schema.UserMessage("请审查以下代码：\n\n```{language}\n{code}\n```"),
    )
}

// 使用示例
template := templates.CodeReviewer("Go")
messages := template.Format(ctx, map[string]any{
    "language": "go",
    "code": `func add(a, b int) int {
    return a + b
}`,
})
```

#### 技术面试官模板

```go
func (p *PromptTemplates) TechInterviewer(position, level string) prompt.ChatTemplate {
    return prompt.FromMessages(
        schema.FString,
        schema.SystemMessage(fmt.Sprintf(
            "你是一位%s职位的面试官，针对%s级别的候选人。\n"+
                "请根据候选人的回答：\n"+
                "1. 评估答案的准确性和深度\n"+
                "2. 提出有针对性的追问\n"+
                "3. 给出建设性的反馈",
            position, level,
        )),
        schema.UserMessage("候选人回答：{answer}\n\n请评估并追问。"),
    )
}

// 使用示例
template := templates.TechInterviewer("Go后端开发", "中级")
messages := template.Format(ctx, map[string]any{
    "answer": "goroutine 是 Go 语言的轻量级线程",
})
```

### 3.3 模板使用流程

```
创建模板管理器
      │
      ▼
 ┌─────────────────┐
 │ PromptTemplates │
 └────────┬────────┘
          │
          ├── Translator() ──────▶ 翻译助手模板
          ├── CodeReviewer() ────▶ 代码审查模板
          └── TechInterviewer() ▶ 技术面试模板
          │
          ▼
    ┌────────────┐
    │ Format(ctx,│
    │ variables) │
    └─────┬──────┘
          │
          ▼
    ┌────────────┐
    │ []*Message  │
    └────────────┘
```

## 四、多角色消息

### 4.1 支持的消息类型

`multi_type.go` 展示了 Eino 支持的多种消息类型：

```go
template := prompt.FromMessages(
    schema.FString,
    // System: 系统提示，定义 AI 的角色和行为
    schema.SystemMessage("你是{role}，你的专长是{expertise}"),

    // User: 用户消息
    schema.UserMessage("我的问题是：{question}"),

    // Assistant: AI 的历史回复（用于多轮对话）
    schema.AssistantMessage("我理解了，让我思考一下...", []schema.ToolCall{}),

    // User: 继续对话
    schema.UserMessage("请详细说明"),
)
```

### 4.2 消息类型说明

| 类型 | 用途 | 特点 |
|------|------|------|
| SystemMessage | 系统提示词 | 定义 AI 角色、行为规则 |
| UserMessage | 用户消息 | 用户输入 |
| AssistantMessage | 助手消息 | AI 历史回复，支持多轮对话 |

### 4.3 完整示例

```go
func main() {
    ctx := context.Background()

    template := prompt.FromMessages(
        schema.FString,
        schema.SystemMessage("你是{role}，你的专长是{expertise}"),
        schema.UserMessage("我的问题是：{question}"),
        schema.AssistantMessage("我理解了，让我思考一下...", []schema.ToolCall{}),
        schema.UserMessage("请详细说明"),
    )

    variables := map[string]any{
        "role":      "一位了解哲学的程序员",
        "expertise": "分析和取舍确定性问题",
        "question":  "在不确定的环境中，如何做出最佳决策？",
    }

    messages, _ := template.Format(ctx, variables)

    for i, msg := range messages {
        fmt.Printf("%d. [%s]\n   %s\n\n", i+1, msg.Role, msg.Content)
    }
}
```

**运行结果：**

```
1. [system]
   你是一位了解哲学的程序员，你的专长是分析和取舍确定性问题。

2. [user]
   我的问题是：在不确定的环境中，如何做出最佳决策？

3. [assistant]
   我理解了，让我思考一下...

4. [user]
   请详细说明
```

## 五、复杂数据结构

### 5.1 结构体映射

`complex_logic.go` 展示了如何将复杂数据结构用于模板：

```go
type UserProfile struct {
    Name       string
    Age        int
    Intersts  []string
    VIPLevel   int
}

// 准备用户数据
user := UserProfile{
    Name:      "张三",
    Age:       28,
    Interests: []string{"编程", "阅读", "旅行"},
    VIPLevel:  3,
}

variables := map[string]any{
    "name":      user.Name,
    "age":       user.Age,
    "interests": fmt.Sprintf("%v", user.Interests),
    "vip_level": user.VIPLevel,
}
```

### 5.2 模板中的多行文本

```go
template := prompt.FromMessages(
    schema.FString,
    schema.SystemMessage("你是一个智能推荐助手"),
    schema.UserMessage(`用户信息：
姓名：{name}
年龄：{age}
兴趣：{interests}
VIP等级：{vip_level}

请根据以上信息推荐合适的内容。`),
)
```

### 5.3 格式化集合类型

对于切片等集合类型，需要使用 `fmt.Sprintf` 或 `strings.Join` 转换：

```go
// 方式1: 使用 fmt.Sprintf
"interests": fmt.Sprintf("%v", user.Interests)
// 输出: [编程 阅读 旅行]

// 方式2: 使用 strings.Join
"interests": strings.Join(user.Interests, "、")
// 输出: 编程、阅读、旅行
```

### 5.4 完整示例

```go
func main() {
    template := prompt.FromMessages(
        schema.FString,
        schema.SystemMessage("你是一个智能推荐助手"),
        schema.UserMessage(`用户信息：
姓名：{name}
年龄：{age}
兴趣：{interests}
VIP等级：{vip_level}

请根据以上信息推荐合适的内容。`),
    )

    user := UserProfile{
        Name:      "张三",
        Age:       28,
        Intersts: []string{"编程", "阅读", "旅行"},
        VIPLevel:  3,
    }

    variables := map[string]any{
        "name":      user.Name,
        "age":       user.Age,
        "interests": fmt.Sprintf("%v", user.Interests),
        "vip_level": user.VIPLevel,
    }

    messages, _ := template.Format(ctx, variables)

    for _, msg := range messages {
        fmt.Printf("[%s]\n%s\n\n", msg.Role, msg.Content)
    }
}
```

**运行结果：**

```
[system]
你是一个智能推荐助手

[user]
用户信息：
姓名：张三
年龄：28
兴趣：[编程 阅读 旅行]
VIP等级：3

请根据以上信息推荐合适的内容。
```

## 六、与大模型结合

### 6.1 完整调用流程

`model_multiplex.go` 展示了将 ChatTemplate 与大模型结合的完整流程：

```go
func main() {
    // 1. 加载配置
    cfg, _ := loadConfig(*configPath)

    // 2. 创建 ChatTemplate
    template := prompt.FromMessages(
        schema.FString,
        schema.SystemMessage("你是一个{role}"),
        schema.UserMessage("{question}"),
    )

    // 3. 准备变量
    variables := map[string]any{
        "role":     "热爱运动的程序员",
        "question": "运动和工作哪个更重要？",
    }

    // 4. 格式化消息
    messages, _ := template.Format(ctx, variables)

    // 5. 查看生成的消息
    for i, msg := range messages {
        fmt.Printf("%d. [%s] %s\n", i+1, msg.Role, msg.Content)
    }

    // 6. 创建 ChatModel
    chatModel, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey:  cfg.Model.APIKey,
        Model:   cfg.Model.ModelName,
        BaseURL: cfg.Model.BaseURL,
    })

    // 7. 生成响应
    response, _ := chatModel.Generate(ctx, messages)
    fmt.Printf("\nAI 回答:\n%s\n", response.Content)
}
```

### 6.2 流程图

```
┌─────────────────────────────────────────────────────────────┐
│                        完整调用流程                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐                                          │
│  │ 1. 配置加载   │  loadConfig(config.yml)                  │
│  └──────┬───────┘                                          │
│         │                                                   │
│         ▼                                                   │
│  ┌──────────────┐                                          │
│  │ 2. 创建模板   │  prompt.FromMessages(...)               │
│  └──────┬───────┘                                          │
│         │                                                   │
│         ▼                                                   │
│  ┌──────────────┐    ┌─────────────┐                       │
│  │ 3. 准备变量   │───▶│ variables  │                       │
│  └──────┬───────┘    └─────────────┘                       │
│         │                                                   │
│         ▼                                                   │
│  ┌──────────────┐                                          │
│  │ 4. 格式化消息 │  template.Format(ctx, variables)        │
│  └──────┬───────┘                                          │
│         │                                                   │
│         ▼                                                   │
│  ┌──────────────┐    ┌─────────────┐                       │
│  │ 5. 创建模型   │───▶│ ChatModel  │                       │
│  └──────┬───────┘    └─────────────┘                       │
│         │                                                   │
│         ▼                                                   │
│  ┌──────────────┐                                          │
│  │ 6. 生成响应   │  chatModel.Generate(ctx, messages)      │
│  └──────────────┘                                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 七、最佳实践

### 7.1 提示词设计原则

| 原则 | 说明 | 示例 |
|------|------|------|
| 角色明确 | 指定 AI 扮演的角色 | "你是一个专业的翻译助手" |
| 任务清晰 | 明确说明要做什么 | "请将以下中文翻译成英文" |
| 格式要求 | 说明输出格式要求 | "只输出译文，不要添加解释" |
| 约束条件 | 说明限制或禁止项 | "不要添加引号" |

### 7.2 模板组织建议

```go
// 推荐：使用结构体管理模板
type PromptTemplates struct{}

// 推荐：函数返回模板，而非直接使用
func (p *PromptTemplates) Translator(sourceLang, targetLang string) prompt.ChatTemplate {
    return prompt.FromMessages(...)
}

// 不推荐：每次创建新模板
template := prompt.FromMessages(...)  // 重复代码
```

### 7.3 变量命名规范

```go
// 推荐：清晰的变量名
variables := map[string]any{
    "source_lang": "中文",
    "target_lang": "英文",
    "text": "Hello",
}

// 不推荐：模糊的变量名
variables := map[string]any{
    "s": "中文",
    "t": "英文",
    "x": "Hello",
}
```

### 7.4 错误处理

```go
messages, err := template.Format(ctx, variables)
if err != nil {
    // 检查变量缺失等错误
    log.Fatalf("格式化失败: %v", err)
}

response, err := chatModel.Generate(ctx, messages)
if err != nil {
    // 处理 API 调用错误
    log.Printf("生成失败: %v", err)
}
```

## 八、配置管理

### 8.1 配置文件结构

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

### 8.2 配置加载

```go
type Config struct {
    Model ModelConfig `yaml:"model"`
    App   AppConfig   `yaml:"app"`
}

type ModelConfig struct {
    BaseURL    string  `yaml:"base_url"`
    APIKey     string  `yaml:"api_key"`
    ModelName  string  `yaml:"model_name"`
    Timeout    int     `yaml:"timeout"`
    Temperature float64 `yaml:"temperature"`
    TopP       float64 `yaml:"top_p"`
    MaxTokens  int     `yaml:"max_tokens"`
}

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

### 8.3 命令行参数

```go
func main() {
    configPath := flag.String("config", "config.yml", "配置文件路径")
    flag.Parse()

    cfg, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }
}
```

## 九、运行示例

### 9.1 var_replace.go

```bash
go run var_replace.go -config ../config.yml
```

**输出示例：**

```
===== 翻译示例 =====
翻译结果: Eino is a powerful AI development framework

===== 代码审查示例 =====
审查结果:
1. **潜在bug**: 代码没有错误处理，建议添加...

===== 面试官示例 =====
面试官反馈:
很好，你对 goroutine 的理解基本正确...
```

### 9.2 model_multiplex.go

```bash
go run model_multiplex.go -config ../config.yml
```

**输出示例：**

```
生成的消息:
1. [system] 你是一个热爱运动的程序员。
2. [user] 运动和工作哪个更重要？

AI 回答:
运动和工作都很重要...
```

### 9.3 multi_type.go

```bash
go run multi_type.go
```

**输出示例：**

```
1. [system]
   你是一位了解哲学的程序员，你的专长是分析和取舍确定性问题。

2. [user]
   我的问题是：在不确定的环境中，如何做出最佳决策？

3. [assistant]
   我理解了，让我思考一下...

4. [user]
   请详细说明
```

### 9.4 complex_logic.go

```bash
go run complex_logic.go
```

**输出示例：**

```
[system]
你是一个智能推荐助手

[user]
用户信息：
姓名：张三
年龄：28
兴趣：[编程 阅读 旅行]
VIP等级：3

请根据以上信息推荐合适的内容。
```

## 十、总结

### 10.1 核心功能回顾

| 功能 | 说明 | 示例文件 |
|------|------|---------|
| 变量替换 | 模板中动态替换变量 | var_replace.go |
| 模板复用 | 通过函数封装模板创建 | var_replace.go |
| 多角色 | System/User/Assistant | multi_type.go |
| 复杂数据 | 结构体、切片等 | complex_logic.go |
| 模型结合 | 模板 + 大模型调用 | model_multiplex.go |

### 10.2 ChatTemplate 优势

1. **可维护性**：模板与代码分离，便于修改
2. **可复用性**：一次定义，多次使用
3. **可测试性**：模板可以独立测试
4. **可读性**：清晰的提示词结构

### 10.3 进阶主题

- **模板组合**：多个模板组合使用
- **条件渲染**：根据变量决定渲染内容
- **模板校验**：验证变量完整性
- **模板缓存**：避免重复创建模板

通过本文的学习，你应该掌握了：
1. **ChatTemplate 基础**：创建、格式化、变量替换
2. **多角色消息**：System、User、Assistant 消息
3. **模板管理**：结构化组织多个模板
4. **工程实践**：配置管理、错误处理、最佳实践

ChatTemplate 是 Eino 框架中处理提示词的核心组件，掌握它能够让你的 AI 应用开发更加高效和规范。
