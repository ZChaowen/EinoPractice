# Eino-Workflow 实战详解

## 概述

本文基于 `lab02/workflow/workflow_chat.go` 实战项目，详细讲解 Eino 框架中 Workflow 的概念、初始化、编排和编译过程。Workflow 是 Eino 框架中用于构建分支 AI 流程的核心组件，提供了比 Chain 更灵活的编排能力。

## 一、Eino 框架中 Workflow 的概念

### 1.1 什么是 Workflow

Workflow（工作流）是 Eino 框架中用于编排 AI 流程的重要组件。与 Chain 的线性执行不同，Workflow 支持设置多个输入源和分支结构，可以构建更复杂的 AI 应用流程。

### 1.2 Workflow vs Chain vs Graph

| 特性 | Chain | Workflow | Graph |
|------|-------|----------|-------|
| 结构 | 严格线性 | 分支结构 | DAG |
| 输入源 | 单一起点 | 多个入口 | 多个入口 |
| 节点连接 | 顺序执行 | 链式+分支 | 任意连接 |
| 执行模式 | 顺序 | 顺序/并行 | 拓扑排序 |
| 适用场景 | 简单流水线 | 多阶段流程 | 复杂AI应用 |
| 灵活性 | 低 | 中 | 高 |

### 1.3 Workflow 的核心特点

```go
workflow := compose.NewWorkflow[map[string]any, *schema.Message]()
```

- **泛型参数**：`[map[string]any, *schema.Message]` 定义输入输出类型
- **链式调用**：通过 `AddXXXNode().AddInput()` 实现链式编排
- **多入口设置**：可设置多个 `AddInput` 来源
- **明确的终点**：通过 `.End()` 标记工作流终点

## 二、如何初始化 Workflow

### 2.1 创建 Workflow 实例

```go
func initWorkflow(cfg *Config) error {
    ctx := context.Background()
    wf := compose.NewWorkflow[map[string]any, *schema.Message]()
    // ...
}
```

### 2.2 定义提示词模板

```go
systemTpl := `你是一名篮球教练与比赛分析师。你需要结合用户的基本信息与训练习惯，
使用 player_info API 补全用户画像，然后给出：位置建议、核心技能树、一周训练计划、以及一套简单战术建议。`

chatTpl := prompt.FromMessages(schema.FString,
    schema.SystemMessage(systemTpl),
    schema.MessagesPlaceholder("histories", true),
    schema.UserMessage("{user_query}"),
)
```

### 2.3 创建 ChatModel

```go
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey:  cfg.Model.APIKey,
    Model:   cfg.Model.ModelName,
    BaseURL: cfg.Model.BaseURL,
})
```

### 2.4 创建工具节点

```go
playerInfoTool := utils.NewTool(
    &schema.ToolInfo{
        Name: "player_info",
        Desc: "根据用户的姓名和邮箱，查询用户的篮球相关信息",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "name":  {Type: "string", Desc: "用户的姓名"},
            "email": {Type: "string", Desc: "用户的邮箱"},
        }),
    },
    func(ctx context.Context, input *playerInfoRequest) (*playerInfoResponse, error) {
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

toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
    Tools: []tool.BaseTool{playerInfoTool},
})
```

### 2.5 创建 Lambda 节点

Lambda 节点用于数据转换，处理节点的输入输出：

```go
// 把 toolsNode 输出的 []*schema.Message -> 提炼成一个普通 user message
toolToTextOps := func(
    ctx context.Context,
    input *schema.StreamReader[[]*schema.Message],
) (*schema.StreamReader[*schema.Message], error) {
    return schema.StreamReaderWithConvert(input, func(msgs []*schema.Message) (*schema.Message, error) {
        if len(msgs) == 0 {
            return nil, errors.New("no message")
        }

        type lite struct {
            Content string `json:"content,omitempty"`
        }
        lites := make([]lite, 0, len(msgs))
        for _, m := range msgs {
            if m == nil {
                continue
            }
            lites = append(lites, lite{Content: m.Content})
        }

        b, _ := json.MarshalIndent(lites, "", "  ")
        text := "工具返回的用户信息（汇总）：\n" + string(b)

        return schema.UserMessage(text), nil
    }), nil
}
lambdaToolToText := compose.TransformableLambda[[]*schema.Message, *schema.Message](toolToTextOps)
```

## 三、如何编排 Workflow

### 3.1 节点类型

Workflow 支持的节点类型与 Graph 类似：

| 节点类型 | 方法 | 说明 |
|---------|------|------|
| ChatTemplateNode | `AddChatTemplateNode` | 提示词模板节点 |
| ChatModelNode | `AddChatModelNode` | 大模型节点 |
| ToolsNode | `AddToolsNode` | 工具节点 |
| LambdaNode | `AddLambdaNode` | 自定义转换节点 |

### 3.2 链式编排

Workflow 的核心特点是链式调用，每个节点方法返回 Workflow 自身，支持连续调用：

```go
wf.AddChatTemplateNode("prompt", chatTpl).AddInput(compose.START)
wf.AddChatModelNode("chat", chatModel).AddInput("prompt")
wf.AddToolsNode("tools", toolsNode).AddInput("chat")
wf.AddLambdaNode("tool_to_text", lambdaToolToText).AddInput("tools")
wf.AddLambdaNode("prompt_transform", lambdaPrompt).AddInput("tool_to_text")
wf.AddChatModelNode("chat_recommend", chatModel).AddInput("prompt_transform")
wf.End().AddInput("chat_recommend")
```

### 3.3 关键方法说明

- **AddChatTemplateNode(name, template)**：添加提示词模板节点
- **AddChatModelNode(name, model)**：添加大模型节点
- **AddToolsNode(name, tools)**：添加工具节点
- **AddLambdaNode(name, lambda)**：添加 Lambda 转换节点
- **AddInput(source)**：设置节点的输入源（可以是 `compose.START`、其他节点名称或多个节点）
- **End()**：标记工作流终点

### 3.4 Workflow 编排流程图

```
START ────────────────────────────────────────────────────────
  │                                                            │
  ▼                                                            │
┌─────────────────┐                                            │
│   promptNode    │  (ChatTemplate: 篮球教练系统提示词)              │
└────────┬────────┘                                            │
         │                                                     │
         ▼                                                     │
┌─────────────────┐                                            │
│   chatNode      │  (ChatModel: 第一次模型调用，决定是否调用工具)      │
└────────┬────────┘                                            │
         │                                                     │
         ▼                                                     │
┌─────────────────┐                                            │
│   toolsNode     │  (ToolsNode: player_info 工具)              │
└────────┬────────┘                                            │
         │                                                     │
         ▼                                                     │
┌─────────────────┐                                            │
│ tool_to_text    │  (Lambda: 工具结果转为用户消息)                 │
└────────┬────────┘                                            │
         │                                                     │
         ▼                                                     │
┌─────────────────┐                                            │
│ prompt_transform│  (Lambda: 构造第二次模型输入)                  │
└────────┬────────┘                                            │
         │                                                     │
         ▼                                                     │
┌──────────────────────┐                                        │
│  chat_recommend     │  (ChatModel: 第二次模型调用，生成最终推荐)     │
└────────┬─────────────┘                                        │
         │                                                     │
         ▼                                                     │
        END ◀─────────────────────────────────────────────────┘
```

## 四、如何编译 Workflow

### 4.1 编译方法

```go
func compileWorkflow() error {
    ctx := context.Background()
    r, err := workflow.Compile(ctx)
    if err != nil {
        return fmt.Errorf("编译 Workflow 失败: %w", err)
    }
    runnable = r
    log.Printf("Workflow 编译成功")
    return nil
}
```

### 4.2 执行 Workflow

编译后的 Workflow 是一个 `Runnable` 对象：

```go
output, err := runnable.Invoke(ctx, map[string]any{
    "histories":  []*schema.Message{},
    "user_query": req.UserQuery,
})
```

### 4.3 响应处理

```go
resp := WorkflowResponse{
    Content:          output.Content,
    ReasoningContent: output.ReasoningContent,
}

if output.ResponseMeta != nil && output.ResponseMeta.Usage != nil {
    resp.PromptTokens = output.ResponseMeta.Usage.PromptTokens
    resp.OutputTokens = output.ResponseMeta.Usage.CompletionTokens
    resp.TotalTokens = output.ResponseMeta.Usage.TotalTokens
}
```

## 五、Workflow 与 Graph 的区别

### 5.1 编排方式不同

**Workflow 链式编排**：
```go
wf.AddChatTemplateNode("prompt", chatTpl).AddInput(compose.START)
wf.AddChatModelNode("chat", chatModel).AddInput("prompt")
```

**Graph 声明式编排**：
```go
g.AddChatTemplateNode("prompt", chatTpl)
g.AddChatModelNode("chat", chatModel)
g.AddEdge("prompt", "chat")  // 显式添加边
```

### 5.2 结构特点

- **Workflow**：更适合线性或简单分支结构，代码更简洁
- **Graph**：更适合复杂网状结构，需要显式定义节点间的边

### 5.3 选择建议

| 场景 | 推荐使用 |
|------|---------|
| 简单流水线 | Chain |
| 多阶段分支流程 | Workflow |
| 复杂 DAG 结构 | Graph |

## 六、完整项目结构

```
lab02/
├── config.yml              # 配置文件
├── workflow/
│   ├── workflow_chat.go    # 主程序
│   ├── workflow_chat       # 编译产物
│   └── docs/              # Swagger文档
│       ├── docs.go
│       ├── swagger.json
│       └── swagger.yaml
```

### 配置文件 config.yml

```yaml
model:
  base_url: "https://api.minimaxi.com/v1"
  api_key: "your-api-key"
  model_name: "MiniMax-M2.7"

app:
  host: "0.0.0.0"
  port: 8080
```

## 七、运行项目

### 启动服务

```bash
cd lab02/workflow
go run workflow_chat.go -log workflow.log
```

### API 调用

```bash
curl -X POST http://localhost:8080/workflow \
  -H "Content-Type: application/json" \
  -d '{"user_query": "我叫morning，邮箱是lumworn@gmail.com，帮我制定训练计划"}'
```

### 访问 Swagger UI

```
http://localhost:8080/swagger/index.html
```

## 八、总结

本文通过 `workflow_chat.go` 实战项目，详细讲解了：

1. **Workflow 概念**：Eino 框架中用于构建分支 AI 流程的核心组件，支持链式编排
2. **初始化 Workflow**：创建实例、定义模板、创建工具和 Lambda 节点
3. **编排 Workflow**：通过链式调用 `AddNode().AddInput()` 方法串联各节点
4. **编译 Workflow**：调用 `Compile` 方法将 Workflow 编译为可执行对象

Workflow 相比 Chain 提供了更灵活的分支编排能力，同时比 Graph 的语法更简洁，适用于多阶段、多分支的 AI 应用场景。