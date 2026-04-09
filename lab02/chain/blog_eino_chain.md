# Eino Chain 编排实战详解

> 摘要：本文深入剖析 Eino 框架中 Chain（链式编排）的实现原理，通过一个篮球教练助手的完整案例，详细讲解 Tool 创建、ToolsNode 输出提取、Prompt 转换、Chain 编排等核心技术的实现细节。

<!-- more -->

## 一、Chain 编排概述

### 1.1 什么是 Chain

Chain（链式编排）是 Eino 框架中用于组合多个 AI 组件的核心概念。它允许我们将多个节点（Node）串联起来，形成一个完整的工作流程：

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ ChatTemplate │ -> │   ChatModel  │ -> │  ToolsNode   │ -> │    Lambda    │
│   (模板)     │    │   (模型)     │    │   (工具)     │    │   (转换)     │
└──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
                                                                  │
                                                                  v
                                    ┌──────────────┐    ┌──────────────┐
                                    │    Lambda    │ -> │   ChatModel  │
                                    │   (转换)     │    │   (模型)     │
                                    └──────────────┘    └──────────────┘
```

### 1.2 本文案例：篮球教练助手

我们将实现一个**篮球教练助手**，用户输入姓名和邮箱，系统自动查询用户信息并生成：
- 位置建议（后卫/锋线/内线）
- 训练计划（一周训练安排）
- 战术建议（简单业余队战术）

### 1.3 Chain 执行流程

```
1. 用户提问 -> ChatTemplate 构建提示词
2. ChatModel 生成包含工具调用的回复
3. ToolsNode 执行工具，返回结果
4. Lambda 转换工具输出为文本
5. Lambda 构造第二次请求
6. ChatModel 生成最终建议
```

---

## 二、核心组件详解

### 2.1 ChatTemplate（提示词模板）

ChatTemplate 负责将用户输入和系统提示组装成完整的对话上下文：

```go
// 系统提示词模板
systemTpl := `你是一名篮球教练与比赛分析师。你需要结合用户的基本信息与训练习惯，
使用 player_info 工具补全信息，然后给出适合他的训练计划/位置建议/一套简单战术建议。
注意：邮箱必须出现，用于查询信息。`

// 构建模板
chatTpl := prompt.FromMessages(schema.FString,
    schema.SystemMessage(systemTpl),           // 系统消息
    schema.MessagesPlaceholder("histories", true), // 历史消息占位符
    schema.UserMessage("{user_query}"),        // 用户输入占位符
)
```

**关键点说明：**

| 组件 | 说明 |
|------|------|
| `schema.FString` | 表示使用 FString 格式化风格 |
| `schema.SystemMessage` | 系统消息，定义 AI 角色 |
| `schema.MessagesPlaceholder` | 历史消息占位符，支持传入消息列表 |
| `schema.UserMessage("{user_query}")` | 用户消息占位符，运行时被实际输入替换 |

### 2.2 ChatModel（聊天模型）

ChatModel 是与大模型交互的组件，本例使用 DeepSeek：

```go
chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
    APIKey:  cfg.Model.APIKey,
    Model:   cfg.Model.ModelName,
    BaseURL: cfg.Model.BaseURL,
})
```

---

## 三、Tool 创建详解

### 3.1 Tool 的作用

Tool（工具）是让大模型能够调用外部函数的能力。本例中 `player_info` 工具用于查询用户篮球相关信息。

### 3.2 工具入参和出参定义

```go
// 工具入参：用户必须提供姓名和邮箱
type playerInfoRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 工具出参：返回用户的篮球相关信息
type playerInfoResponse struct {
    Name        string `json:"name"`         // 位置：后卫/锋线/中锋/教练/爱好者
    Email       string `json:"email"`
    Role        string `json:"role"`
    HeightCM    int    `json:"height_cm"`    // 身高(cm)
    WeightKG    int    `json:"weight_kg"`    // 体重(kg)
    PlayStyle   string `json:"play_style"`   // 打球风格
    WeeklyHours int    `json:"weekly_hours"` // 每周训练时长
}
```

### 3.3 创建 Tool 实例

```go
playerInfoTool := utils.NewTool(
    // 工具信息定义
    &schema.ToolInfo{
        Name: "player_info",  // 工具名称，模型通过这个名称调用
        Desc: "根据用户的姓名和邮箱，查询用户的篮球相关信息（位置倾向、身体数据、打球习惯等）",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "name": {
                Type: "string",
                Desc: "用户姓名",
            },
            "email": {
                Type: "string",
                Desc: "用户邮箱",
            },
        }),
    },
    // 工具的实际执行逻辑（mock 实现）
    func(ctx context.Context, input *playerInfoRequest) (output *playerInfoResponse, err error) {
        return &playerInfoResponse{
            Name:        input.Name,
            Email:       input.Email,
            Role:        "锋线",
            HeightCM:    182,
            WeightKG:    78,
            PlayStyle:   "偏投射+无球空切，偶尔持球突破",
            WeeklyHours: 4,
        }, nil
    },
)
```

**工具定义的关键字段：**

| 字段 | 说明 | 示例 |
|------|------|------|
| `Name` | 工具唯一标识 | `"player_info"` |
| `Desc` | 工具描述，供模型理解何时调用 | `"查询用户篮球信息"` |
| `ParamsOneOf` | 参数定义 | 参数名、类型、描述 |

### 3.4 将 Tool 绑定到 ChatModel

```go
// 获取工具信息
info, err := playerInfoTool.Info(ctx)

// 绑定到模型，模型才能知道有哪些工具可用
if err := chatModel.BindTools([]*schema.ToolInfo{info}); err != nil {
    return fmt.Errorf("绑定工具失败: %w", err)
}
```

**执行流程：**
1. 模型判断需要调用工具 -> 生成 `tool_calls`
2. ToolsNode 接收调用请求 -> 执行工具函数
3. 工具返回结果 -> ToolsNode 输出 `role=tool` 的消息

---

## 四、ToolsNode 创建与使用

### 4.1 创建 ToolsNode

ToolsNode 是 Chain 中的节点，负责执行绑定到模型的工具：

```go
toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
    Tools: []tool.BaseTool{playerInfoTool},
})
```

### 4.2 ToolsNode 的输入输出

```
输入：ChatModel 生成的包含 tool_calls 的消息列表
      [
        {role: "assistant", content: "我来查询您的信息...", tool_calls: [...]},
        {role: "user", content: "..."} // 追加的用户消息
      ]

输出：工具执行结果
      [
        {role: "tool", content: "{\"name\":\"morning\", \"role\":\"锋线\", ...}"},
      ]
```

---

## 五、从 ToolsNode 提取工具返回内容

### 5.1 为什么需要提取

ToolsNode 输出的是 `role=tool` 的特殊消息，但这些消息不能直接作为下一次模型调用的输入，因为：

1. **格式问题**：模型的 `tool` role 消息有特定用途，不适合作为普通对话
2. **上下文问题**：需要将工具结果转换为可读的文本，附加到对话中

### 5.2 Lambda 转换函数

我们使用 `compose.TransformableLambda` 来实现数据转换：

```go
toolToTextOps := func(
    ctx context.Context,
    input *schema.StreamReader[[]*schema.Message],  // 输入：ToolsNode 输出的消息流
) (output *schema.StreamReader[*schema.Message], error) {  // 输出：单个消息
    return schema.StreamReaderWithConvert(input, func(msgs []*schema.Message) (*schema.Message, error) {
        // 从消息列表中提取所有 role=tool 的内容
        var toolContents []string
        for _, m := range msgs {
            if m == nil {
                continue
            }
            if m.Role == "tool" {  // 筛选工具返回的消息
                toolContents = append(toolContents, m.Content)
            }
        }

        // 构建可读的文本
        text := "工具未返回有效信息。"
        if len(toolContents) > 0 {
            text = "工具返回的用户信息如下：\n- " + toolContents[0]
            for i := 1; i < len(toolContents); i++ {
                text += "\n- " + toolContents[i]
            }
        }

        // 转换为普通 user 消息，供下一轮使用
        return schema.UserMessage(text), nil
    }), nil
}

// 创建 Lambda 节点
lambdaToolToText := compose.TransformableLambda[[]*schema.Message, *schema.Message](toolToTextOps)
```

### 5.3 数据转换图示

```
ToolsNode 输出:
[
    {role: "tool", content: "{\"Name\":\"morning\",\"Role\":\"锋线\",...}"},
    {role: "tool", content: "..."},
]

        ↓ Lambda 转换

单个 User 消息:
{
    role: "user",
    content: "工具返回的用户信息如下：\n- {\"Name\":\"morning\",\"Role\":\"锋线\",...}\n- ..."
}
```

---

## 六、构造第二次模型输入

### 6.1 为什么需要第二次调用

第一次模型调用是为了让模型决定调用哪个工具，第二次调用是为了基于工具返回结果生成最终建议。

### 6.2 推荐模板

```go
recommendTpl := `
你是一名篮球教练与比赛分析师。请结合工具返回的用户信息，为用户输出建议，要求具体、可执行。

--- 训练资源（可选方案库）---

### A. 训练方向库（按位置/风格）
**1. 后卫（控运与节奏）**
- 核心：运球对抗、挡拆阅读、急停跳投、突破分球

**2. 锋线（持球终结与防守）**
- 核心：三威胁、低位脚步、协防轮转、错位单打

### B. 输出规则
1) 先总结用户画像（身高体重、风格、每周训练时长）
2) 给出建议位置与核心技能树（3-5个技能）
3) 输出一周训练计划（按天、每次45-90分钟）
4) 给一套战术建议 + 业余局实战注意事项（3条）
`
```

### 6.3 Prompt 转换 Lambda

```go
promptTransformOps := func(
    ctx context.Context,
    input *schema.StreamReader[*schema.Message],  // 输入：lambdaToolToText 输出的单个消息
) (output *schema.StreamReader[[]*schema.Message], error) {  // 输出：消息数组
    return schema.StreamReaderWithConvert(input, func(m *schema.Message) ([]*schema.Message, error) {
        out := make([]*schema.Message, 0, 2)
        // 添加系统消息（包含推荐模板和输出规则）
        out = append(out, schema.SystemMessage(recommendTpl))
        // 添加工具转换后的用户消息
        out = append(out, m)
        return out, nil
    }), nil
}

lambdaPrompt := compose.TransformableLambda[*schema.Message, []*schema.Message](promptTransformOps)
```

### 6.4 数据转换图示

```
输入（单个消息）:
{role: "user", content: "工具返回的用户信息如下：..."}

        ↓ Lambda 转换

输出（消息数组）:
[
    {role: "system", content: "你是一名篮球教练...\n---训练资源---\n..."},
    {role: "user", content: "工具返回的用户信息如下：..."}
]

        ↓ 作为第二次 ChatModel 输入
```

---

## 七、Chain 编排详解

### 7.1 编排代码

```go
// 创建 Chain，指定输入输出类型
chain := compose.NewChain[map[string]any, *schema.Message]()

// 链式调用，按顺序添加节点
chain.
    AppendChatTemplate(chatTpl).      // 1. 模板节点
    AppendChatModel(chatModel).       // 2. 模型节点（第一次）
    AppendToolsNode(toolsNode).       // 3. 工具节点
    AppendLambda(lambdaToolToText).   // 4. Lambda（提取工具结果）
    AppendLambda(lambdaPrompt).       // 5. Lambda（构造第二次输入）
    AppendChatModel(chatModel)        // 6. 模型节点（第二次）
```

### 7.2 Chain 编排流程图

```
用户输入: {user_query: "我叫morning，邮箱是...", histories: []}

    │
    ▼
┌─────────────────────────────────────┐
│ 1. ChatTemplate                      │
│ 输入: {histories, user_query}        │
│ 输出: [system(msg), user(query)]    │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│ 2. ChatModel (第一次)                │
│ 输入: [system, user]                │
│ 输出: assistant(tool_calls)          │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│ 3. ToolsNode                        │
│ 输入: assistant(tool_calls)          │
│ 输出: [tool(result)]                │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│ 4. Lambda (toolToText)              │
│ 输入: [tool(result)]                 │
│ 输出: user(格式化文本)               │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│ 5. Lambda (promptTransform)         │
│ 输入: user(格式化文本)               │
│ 输出: [system(recommendTpl), user]  │
└─────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────┐
│ 6. ChatModel (第二次)                │
│ 输入: [system, user]                │
│ 输出: assistant(最终建议)           │
└─────────────────────────────────────┘
    │
    ▼

最终输出: {content: "根据您的情况...", reasoning_content: "..."}
```

### 7.3 编译与执行

```go
// 编译 Chain，得到可执行的 runnable
runnable, err := chain.Compile(ctx)
if err != nil {
    return fmt.Errorf("编译 Chain 失败: %w", err)
}

// 执行 Chain
output, err := runnable.Invoke(ctx, map[string]any{
    "histories":  []*schema.Message{},
    "user_query": "我叫 morning, 邮箱是 lumworn@gmail.com...",
})
```

### 7.4 Chain 类型参数解析

```go
compose.NewChain[map[string]any, *schema.Message]()
```

| 参数 | 说明 | 示例输入 |
|------|------|----------|
| `map[string]any` | 输入类型 | `{"histories": [], "user_query": "..."}` |
| `*schema.Message` | 输出类型 | `&Message{Content: "...", ...}` |

---

## 八、StreamReader 与数据流

### 8.1 什么是 StreamReader

StreamReader 是 Eino 框架中用于处理流式数据的核心类型，它包装了一个数据流并提供转换能力：

```go
type StreamReader[T any] struct {
    // 内部数据结构
}
```

### 8.2 StreamReaderWithConvert

`StreamReaderWithConvert` 用于将一种类型的 StreamReader 转换为另一种类型：

```go
schema.StreamReaderWithConvert(input, transformFunc)
```

**泛型签名：**
```go
func StreamReaderWithConvert[IN any, OUT any](
    input *StreamReader[IN],
    fn func(IN) (OUT, error),
) *StreamReader[OUT]
```

### 8.3 转换示例

```go
// 将 []*schema.Message 转换为 *schema.Message
schema.StreamReaderWithConvert(input, func(msgs []*schema.Message) (*schema.Message, error) {
    // 合并多个消息为单个消息
    return mergeMessages(msgs), nil
})

// 将 *schema.Message 转换为 []*schema.Message
schema.StreamReaderWithConvert(input, func(m *schema.Message) ([]*schema.Message, error) {
    // 将单个消息包装成数组
    return []*schema.Message{m}, nil
})
```

---

## 九、完整代码实现

### 9.1 核心代码

```go
// initChain 初始化Chain
func initChain(cfg *Config) error {
    ctx := context.Background()

    // 1) ChatTemplate
    chatTpl := prompt.FromMessages(schema.FString,
        schema.SystemMessage(systemTpl),
        schema.MessagesPlaceholder("histories", true),
        schema.UserMessage("{user_query}"),
    )

    // 2) ChatModel
    chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
        APIKey:  cfg.Model.APIKey,
        Model:   cfg.Model.ModelName,
        BaseURL: cfg.Model.BaseURL,
    })

    // 3) Tool
    playerInfoTool := utils.NewTool(&schema.ToolInfo{...}, func(...) {...})

    // 4) BindTools
    info, _ := playerInfoTool.Info(ctx)
    chatModel.BindTools([]*schema.ToolInfo{info})

    // 5) ToolsNode
    toolsNode, _ := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
        Tools: []tool.BaseTool{playerInfoTool},
    })

    // 6) Lambda: 提取工具返回内容
    toolToTextOps := func(ctx context.Context, input *schema.StreamReader[[]*schema.Message]) (...) {
        return schema.StreamReaderWithConvert(input, func(msgs []*schema.Message) (*schema.Message, error) {
            // 提取 role=tool 的内容
            // 转换为 user 消息
        }), nil
    }
    lambdaToolToText := compose.TransformableLambda[[]*schema.Message, *schema.Message](toolToTextOps)

    // 7) Lambda: 构造第二次输入
    promptTransformOps := func(ctx context.Context, input *schema.StreamReader[*schema.Message]) (...) {
        return schema.StreamReaderWithConvert(input, func(m *schema.Message) ([]*schema.Message, error) {
            // 添加 system(recommendTpl) + user(工具结果)
        }), nil
    }
    lambdaPrompt := compose.TransformableLambda[*schema.Message, []*schema.Message](promptTransformOps)

    // 8) Chain 编排
    chain = compose.NewChain[map[string]any, *schema.Message]()
    chain.
        AppendChatTemplate(chatTpl).
        AppendChatModel(chatModel).
        AppendToolsNode(toolsNode).
        AppendLambda(lambdaToolToText).
        AppendLambda(lambdaPrompt).
        AppendChatModel(chatModel)

    return nil
}
```

### 9.2 关键点总结

| 步骤 | 组件 | 作用 |
|------|------|------|
| 1 | ChatTemplate | 构建用户提示词 |
| 2 | ChatModel | 生成工具调用请求 |
| 3 | ToolsNode | 执行工具，返回结果 |
| 4 | Lambda | 提取工具结果，转换为文本 |
| 5 | Lambda | 构造包含推荐模板的第二次输入 |
| 6 | ChatModel | 生成最终建议 |

---

## 十、常见问题与解决方案

### 10.1 模型不调用工具

**可能原因：**
- Tool 未正确绑定到模型
- 工具描述不够清晰
- 系统提示词未引导模型使用工具

**解决方案：**
```go
// 确保正确绑定
info, _ := playerInfoTool.Info(ctx)
chatModel.BindTools([]*schema.ToolInfo{info})

// 在系统提示词中明确要求使用工具
systemTpl := "... 使用 player_info 工具补全信息 ..."
```

### 10.2 工具参数提取失败

**可能原因：**
- 模型生成的参数格式与定义的 ToolInfo 不匹配
- 参数类型定义错误

**解决方案：**
```go
// 确保参数定义正确
ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
    "name": {
        Type: "string",
        Desc: "用户姓名",
    },
    "email": {
        Type: "string",
        Desc: "用户邮箱",
    },
}),
```

### 10.3 Lambda 类型转换错误

**可能原因：**
- 输入输出类型与实际不匹配
- TransformableLambda 泛型参数顺序错误

**解决方案：**
```go
// 明确指定输入输出类型
// 输入: []*schema.Message, 输出: *schema.Message
lambdaToolToText := compose.TransformableLambda[[]*schema.Message, *schema.Message](toolToTextOps)

// 输入: *schema.Message, 输出: []*schema.Message
lambdaPrompt := compose.TransformableLambda[*schema.Message, []*schema.Message](promptTransformOps)
```

---

## 十一、总结

本文通过篮球教练助手案例，详细讲解了 Eino Chain 编排的核心技术：

1. **Chain 原理**：通过链式调用组合多个 AI 组件
2. **Tool 创建**：定义工具信息、参数、执行逻辑
3. **ToolsNode**：执行模型调用的工具
4. **Lambda 转换**：使用 `StreamReaderWithConvert` 实现数据流转换
5. **Prompt 转换**：通过 Lambda 构造第二次模型输入
6. **Chain 编排**：使用 `Append*` 方法组合节点

Chain 编排是构建复杂 AI 应用的核心能力，掌握这些技术可以实现：
- 多轮对话
- 工具调用
- RAG（检索增强生成）
- 智能代理（Agent）

---

**参考资料：**
- [Eino 框架官方文档](https://github.com/cloudwego/eino)
- [Eino Chain 组件](https://github.com/cloudwego/eino/blob/main/compose/chain.go)
- [Eino ToolsNode 组件](https://github.com/cloudwego/eino/blob/main/compose/tools_node.go)

> 首发于 CSDN，转载请注明出处
