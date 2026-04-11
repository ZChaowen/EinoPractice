# Eino-Graph 实战详解

## 概述

本文基于 `lab02/graph/graph_chat.go` 实战项目，详细讲解 Eino 框架中 Graph 的概念、初始化、编排和编译过程。Graph 是 Eino 框架中用于构建复杂 AI 流程编排的核心组件，支持有向无环图（DAG）结构的节点编排。

## 一、Eino 框架中 Graph 的概念

### 1.1 什么是 Graph

Graph（有向无环图）是 Eino 框架中用于编排 AI 流程的核心数据结构。与 Chain 的线性结构不同，Graph 支持更复杂的分支、循环和条件跳转逻辑，可以构建多阶段、多分支的复杂 AI 应用。

### 1.2 Graph vs Chain vs Workflow

| 特性 | Chain | Workflow | Graph |
|------|-------|----------|-------|
| 结构 | 线性 | 分支/条件 | DAG |
| 节点连接 | 顺序执行 | 可条件分支 | 任意连接 |
| 适用场景 | 简单流水线 | 中等复杂流程 | 复杂AI应用 |
| 灵活性 | 低 | 中 | 高 |

### 1.3 Graph 的核心组成

```go
// Graph 的类型定义
graph := compose.NewGraph[map[string]any, *schema.Message]()
```

- **泛型参数**：`[map[string]any, *schema.Message]` 表示输入是 map，输出是 schema.Message
- **节点类型**：ChatTemplateNode、ChatModelNode、ToolsNode、LambdaNode 等
- **边（Edge）**：连接节点的边，定义数据流向

## 二、openai.NewChatModel 方法使用

### 2.1 引入依赖

```go
import (
    "github.com/cloudwego/eino-ext/components/model/openai"
)
```

### 2.2 配置参数

```go
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey:  cfg.Model.APIKey,    // API密钥
    Model:   cfg.Model.ModelName,  // 模型名称
    BaseURL: cfg.Model.BaseURL,    // API基础地址
})
```

### 2.3 工具绑定

大模型可以通过 `BindTools` 方法绑定工具，使其具备调用外部函数的能力：

```go
info, err := playerInfoTool.Info(ctx)
if err != nil {
    return fmt.Errorf("获取工具信息失败: %w", err)
}
if err := chatModel.BindTools([]*schema.ToolInfo{info}); err != nil {
    return fmt.Errorf("绑定工具失败: %w", err)
}
```

## 三、如何初始化 Graph

### 3.1 创建 Graph 实例

```go
func initGraph(cfg *Config) error {
    ctx := context.Background()
    g := compose.NewGraph[map[string]any, *schema.Message]()
    // ...
}
```

### 3.2 定义提示词模板

```go
systemTpl := `你是一名篮球教练与比赛分析师。你需要结合用户的基本信息与训练习惯，
使用 player_info API，为其补全信息，然后给出适合他的训练计划、位置建议与一套简单战术建议。`

chatTpl := prompt.FromMessages(schema.FString,
    schema.SystemMessage(systemTpl),
    schema.MessagesPlaceholder("histories", true),
    schema.UserMessage("{user_query}"),
)
```

**提示词模板说明**：
- `schema.SystemMessage()` - 系统消息，设置 AI 角色
- `schema.MessagesPlaceholder("histories", true)` - 历史消息占位符
- `schema.UserMessage("{user_query}")` - 用户消息模板，`{user_query}` 会动态替换

### 3.3 创建工具节点

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

### 3.4 创建 Lambda 节点

Lambda 节点用于数据转换，实现自定义的数据处理逻辑：

```go
// 从 toolsNode 输出中提取工具结果，转为普通 user 文本
extractToolOps := func(
    ctx context.Context,
    input *schema.StreamReader[[]*schema.Message],
) (*schema.StreamReader[*schema.Message], error) {
    return schema.StreamReaderWithConvert(input, func(msgs []*schema.Message) (*schema.Message, error) {
        if len(msgs) == 0 {
            return nil, errors.New("no messages from tools node")
        }

        type msgLite struct {
            Role    string `json:"role,omitempty"`
            Content string `json:"content,omitempty"`
        }
        lites := make([]msgLite, 0, len(msgs))
        for _, m := range msgs {
            if m == nil {
                continue
            }
            lites = append(lites, msgLite{
                Role:    string(m.Role),
                Content: m.Content,
            })
        }
        b, _ := json.MarshalIndent(lites, "", "  ")

        text := "工具执行完成，返回信息如下（结构化摘要）：\n" + string(b)
        return schema.UserMessage(text), nil
    }), nil
}
extractToolLambda := compose.TransformableLambda[[]*schema.Message, *schema.Message](extractToolOps)
```

## 四、如何编排 Graph

### 4.1 节点类型

Eino Graph 支持以下几种核心节点类型：

| 节点类型 | 方法 | 说明 |
|---------|------|------|
| ChatTemplateNode | `AddChatTemplateNode` | 提示词模板节点 |
| ChatModelNode | `AddChatModelNode` | 大模型节点 |
| ToolsNode | `AddToolsNode` | 工具节点 |
| LambdaNode | `AddLambdaNode` | 自定义转换节点 |

### 4.2 添加节点

```go
const (
    promptNodeKey        = "prompt"
    chatNodeKey          = "chat"
    toolsNodeKey         = "tools"
    extractNodeKey       = "extract_tool_result"
    lambdaPromptNodeKey  = "build_recommend_prompt"
    recommendChatNodeKey = "chat_recommend"
)

_ = g.AddChatTemplateNode(promptNodeKey, chatTpl)
_ = g.AddChatModelNode(chatNodeKey, chatModel)
_ = g.AddToolsNode(toolsNodeKey, toolsNode)
_ = g.AddLambdaNode(extractNodeKey, extractToolLambda)
_ = g.AddLambdaNode(lambdaPromptNodeKey, buildPromptLambda)
_ = g.AddChatModelNode(recommendChatNodeKey, chatModel)
```

### 4.3 添加边（连接）

```go
_ = g.AddEdge(compose.START, promptNodeKey)          // START -> prompt
_ = g.AddEdge(promptNodeKey, chatNodeKey)           // prompt -> chat
_ = g.AddEdge(chatNodeKey, toolsNodeKey)            // chat -> tools
_ = g.AddEdge(toolsNodeKey, extractNodeKey)        // tools -> extract
_ = g.AddEdge(extractNodeKey, lambdaPromptNodeKey)  // extract -> build_prompt
_ = g.AddEdge(lambdaPromptNodeKey, recommendChatNodeKey)  // build_prompt -> chat_recommend
_ = g.AddEdge(recommendChatNodeKey, compose.END)    // chat_recommend -> END
```

### 4.4 Graph 编排流程图

```
START
  │
  ▼
┌─────────────────┐
│   promptNode    │  (ChatTemplate: 篮球教练系统提示词)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   chatNode      │  (ChatModel: 第一次模型调用，决定是否调用工具)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   toolsNode      │  (ToolsNode: player_info 工具)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  extractNode    │  (Lambda: 提取工具结果，转为用户消息)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ buildPromptNode │  (Lambda: 构造第二次模型输入，添加推荐模板)
└────────┬────────┘
         │
         ▼
┌──────────────────────┐
│  recommendChatNode   │  (ChatModel: 第二次模型调用，生成最终推荐)
└────────┬─────────────┘
         │
         ▼
        END
```

## 五、如何编译 Graph

### 5.1 编译方法

```go
func compileGraph() error {
    ctx := context.Background()
    r, err := graph.Compile(ctx)
    if err != nil {
        return fmt.Errorf("编译 Graph 失败: %w", err)
    }
    runnable = r
    log.Printf("Graph 编译成功")
    return nil
}
```

### 5.2 执行 Graph

编译后的 Graph 是一个 `Runnable` 对象，可以调用 `Invoke` 方法执行：

```go
output, err := runnable.Invoke(ctx, map[string]any{
    "histories":  []*schema.Message{},
    "user_query": req.UserQuery,
})
```

### 5.3 响应结构

```go
resp := GraphResponse{
    Content:          output.Content,           // 模型回复内容
    ReasoningContent: output.ReasoningContent, // 思考内容（部分模型支持）
}

if output.ResponseMeta != nil && output.ResponseMeta.Usage != nil {
    resp.PromptTokens = output.ResponseMeta.Usage.PromptTokens      // 输入token数
    resp.OutputTokens = output.ResponseMeta.Usage.CompletionTokens  // 输出token数
    resp.TotalTokens = output.ResponseMeta.Usage.TotalTokens        // 总token数
}
```

## 六、完整项目结构

```
lab02/
├── config.yml              # 配置文件
├── graph/
│   ├── graph_chat.go       # 主程序
│   ├── graph_chat          # 编译产物
│   └── docs/               # Swagger文档
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
cd lab02/graph
go run graph_chat.go -log graph.log
```

### API 调用

```bash
curl -X POST http://localhost:8080/graph \
  -H "Content-Type: application/json" \
  -d '{"user_query": "我叫morning，邮箱是lumworn@gmail.com，帮我制定训练计划"}'
```

### 访问 Swagger UI

```
http://localhost:8080/swagger/index.html
```

## 八、总结

本文通过 `graph_chat.go` 实战项目，详细讲解了：

1. **Graph 概念**：Eino 框架中用于构建复杂 AI 流程编排的核心组件，基于 DAG 结构
2. **openai.NewChatModel**：通过配置创建 OpenAI 兼容的 ChatModel，支持工具绑定
3. **初始化 Graph**：创建 Graph 实例、定义模板、创建工具和 Lambda 节点
4. **编排 Graph**：使用 `AddNode` 方法添加节点，`AddEdge` 方法连接节点
5. **编译 Graph**：调用 `Compile` 方法将编排好的 Graph 编译为可执行对象

Graph 相比 Chain 和 Workflow 提供了更灵活的编排能力，适用于需要多分支、复杂数据流转的 AI 应用场景。