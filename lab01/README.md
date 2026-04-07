# Chat Quickstart

基于 Gin Web 框架和 DeepSeek 大模型的聊天服务，支持 Swagger UI 在线调试。

## 1. 快速开始

### 环境要求

- Go 1.21+
- swag CLI 工具

### 启动步骤

```bash
# 1. 安装 swag 工具
go install github.com/swaggo/swag/cmd/swag@latest

# 2. 进入项目目录
cd lab01

# 3. 生成 Swagger 文档
~/go/bin/swag init -g chat_quickstart.go

# 4. 启动服务
go run chat_quickstart.go
```

### 验证服务

```bash
# 健康检查
curl http://localhost:8080/health

# 聊天接口
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"system_prompt": "你是一个有帮助的助手", "user_message": "你好"}'
```

### 访问 Swagger UI

启动服务后，打开浏览器访问：**http://localhost:8080/swagger/index.html**

---

## 2. 实现思路

### 整体架构

```
客户端 → Gin Router
         ├── GET  /health        → 健康检查
         ├── POST /chat          → 聊天对话
         └── GET  /swagger/*any  → Swagger UI
```

### 核心流程

```
1. main() 加载 config.yml 配置文件
2. initChatModel() 初始化 DeepSeek ChatModel
3. 启动 Gin HTTP 服务
4. 客户端请求 → chatHandler() → chatModel.Generate() → 返回响应
```

### chatHandler 请求处理流程

```
请求 JSON → ShouldBindJSON() → 构建 messages → context.WithTimeout()
    ↓                                              ↓
返回 400                                       chatModel.Generate()
    ↓                                              ↓
                                         返回 200 + ChatResponse
```

---

## 3. Gin + Swagger 实现指南

### 3.1 安装依赖

```bash
go get -u github.com/swaggo/gin-swagger
go get -u github.com/swaggo/swag/cmd/swag
```

### 3.2 刷新 go.mod

执行 `go get` 后，依赖会被添加到 `go.mod`，但不会自动整理。需要运行 `go mod tidy` 来清理未使用的依赖并整理 `go.sum`。

```bash
# 整理依赖
go mod tidy
```

**执行后的效果：**

- `go.mod` 中新增了 `gin-swagger` 和 `swag` 相关的依赖声明
- `go.sum` 中新增了依赖包的校验和
- 自动移除项目中未使用的依赖包

### 3.3 生成 Swagger 文档

```bash
swag init -g chat_quickstart.go
```

**执行后产生的文件：**

执行后会在当前目录（或 `-g` 指定文件的目录）下创建 `docs` 文件夹，包含以下文件：

| 文件 | 说明 |
|------|------|
| `docs/docs.go` | Go 源码文件，包含 Swagger 文档的 JSON 数据 |
| `docs/swagger.json` | Swagger 2.0 规范的 JSON 格式文档 |
| `docs/swagger.yaml` | Swagger 2.0 规范的 YAML 格式文档 |

**目录结构：**

```
lab01/
├── chat_quickstart.go
├── config.yml
└── docs/
    ├── docs.go       # 自动生成，不要手动修改
    ├── swagger.json  # Swagger UI 读取的 JSON 文档
    └── swagger.yaml  # YAML 格式文档
```

### 3.4 添加 Swagger 注释

在 handler 函数和结构体上添加注释：

```go
// ChatRequest 聊天请求
//
//	@Description	聊天请求结构
type ChatRequest struct {
	// SystemPrompt 系统提示词
	SystemPrompt string `json:"system_prompt" example:"你是一个有帮助的助手"`
	// UserMessage 用户消息
	UserMessage  string `json:"user_message" example:"你好"`
}

// chatHandler 处理聊天请求
//
//	@Summary		聊天接口
//	@Description	与 DeepSeek 大模型对话
//	@Tags			chat
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ChatRequest	true	"聊天请求"
//	@Success		200		{object}	ChatResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/chat [post]
func chatHandler(c *gin.Context) {
    // ...
}
```

### 3.5 注册 Swagger 路由

在 `main()` 函数中注册 Swagger Handler：

```go
import (
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"
    _ "your_module/lab01/docs"  // 导入自动生成的 docs 包
)

func main() {
    r := gin.Default()
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
```

### 常用注释说明

| 注释 | 说明 |
|------|------|
| `@Summary` | 接口概要 |
| `@Description` | 接口详细描述 |
| `@Tags` | 接口分组标签 |
| `@Param` | 请求参数 |
| `@Success` | 成功响应 |
| `@Failure` | 失败响应 |
| `@Router` | 路由路径 |

---

## 4. 配置文件指南

### 4.1 config.yml 结构

```yaml
model:
  base_url: "https://api.minimaxi.com/v1"   # API 基础地址
  api_key: "your-api-key"                    # API 密钥
  model_name: "MiniMax-M2.7"                 # 模型名称

app:
  host: "0.0.0.0"   # 监听地址
  port: 8080         # 监听端口
```

### 4.2 配置定义（Go 代码）

```go
type Config struct {
    Model ModelConfig `yaml:"model"`
    App   AppConfig   `yaml:"app"`
}

type ModelConfig struct {
    BaseURL   string `yaml:"base_url"`
    APIKey    string `yaml:"api_key"`
    ModelName string `yaml:"model_name"`
}

type AppConfig struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}
```

**为什么要这样定义？**

1. **嵌套结构与 YAML 层级对应**
   - `Config.Model` 对应 YAML 中的 `model:` 节
   - `Config.App` 对应 YAML 中的 `app:` 节
   - 嵌套的 `ModelConfig` 和 `AppConfig` 分别对应各自的子配置

2. **`yaml:"xxx"` 标签的作用**
   - `yaml.Unmarshal()` 会根据标签将 YAML 中的字段映射到 Go 结构体
   - 例如：`yaml:"base_url"` 会将 YAML 中的 `base_url` 映射到结构体的 `BaseURL` 字段
   - 字段名可以不同（蛇形 `base_url` → 驼峰 `BaseURL`），但标签必须匹配

3. **类型定义规范**
   - `BaseURL` 使用 `string` 类型
   - `Port` 使用 `int` 类型
   - Go 的 `yaml.v2` 库会自动进行类型转换（字符串 `"8080"` → 整数 `8080`）

**映射关系示意：**

```
YAML:                           Go:
model:                          type Config struct {
  base_url: "xxx"        →         Model ModelConfig  `yaml:"model"`
  api_key: "xxx"          →       }
                               type ModelConfig struct {
  base_url  string  `yaml:"base_url"`   →  "xxx"
  api_key   string  `yaml:"api_key"`    →  "xxx"
  model_name string `yaml:"model_name"`→  "xxx"
                               }
```

### 4.3 读取配置文件

```go
import "gopkg.in/yaml.v2"

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

### 4.4 使用配置

```go
func main() {
    cfg, err := loadConfig("config.yml")
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    log.Printf("base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)

    model, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
        APIKey:  cfg.Model.APIKey,
        Model:   cfg.Model.ModelName,
        BaseURL: cfg.Model.BaseURL,
    })
}
```

---

## 5. DeepSeek ChatModel 使用指南

### 5.1 初始化模型

```go
import "github.com/cloudwego/eino-ext/components/model/deepseek"

model, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
    APIKey:  "your-api-key",
    Model:   "deepseek-chat",
    BaseURL: "https://api.deepseek.com",
})
```

### 5.2 发送聊天请求

```go
import "github.com/cloudwego/eino/schema"

// 构建消息列表
messages := []*schema.Message{
    schema.SystemMessage("你是一个有帮助的助手"),
    schema.UserMessage("你好，介绍一下自己"),
}

// 调用模型生成响应
response, err := model.Generate(ctx, messages)
if err != nil {
    log.Fatalf("生成响应失败: %v", err)
}

// 获取回复内容
fmt.Println(response.Content)

// 获取 Token 使用统计
if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
    fmt.Printf("PromptTokens: %d\n", response.ResponseMeta.Usage.PromptTokens)
    fmt.Printf("CompletionTokens: %d\n", response.ResponseMeta.Usage.CompletionTokens)
    fmt.Printf("TotalTokens: %d\n", response.ResponseMeta.Usage.TotalTokens)
}
```

### 5.3 消息类型

| 函数 | 说明 |
|------|------|
| `schema.SystemMessage(content)` | 系统消息，设置 AI 角色 |
| `schema.UserMessage(content)` | 用户消息 |
| `schema.AssistantMessage(content)` | 助手消息 |
| `schema.ToolMessage(content, toolCallID)` | 工具消息 |

### 5.4 并发安全

`deepseek.ChatModel` 是**并发安全**的，可以同时处理多个请求，无需额外加锁。

### 5.5 配置参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `APIKey` | string | API 密钥（必填） |
| `Model` | string | 模型名称（必填） |
| `BaseURL` | string | API 基础地址 |
| `Timeout` | time.Duration | 请求超时时间 |
| `MaxTokens` | int | 最大生成 Token 数 |
| `Temperature` | float32 | 采样温度，控制随机性 |
| `TopP` | float32 | 核采样参数 |
| `Stop` | []string | 停止生成序列 |

---

## API 接口

### 健康检查

```
GET /health
```

**响应：**
```json
{
    "status": "ok"
}
```

### 聊天接口

```
POST /chat
```

**请求：**
```json
{
    "system_prompt": "你是一个有帮助的助手",
    "user_message": "你好"
}
```

**响应：**
```json
{
    "content": "你好！有什么可以帮助你的吗？",
    "prompt_tokens": 100,
    "output_tokens": 50,
    "total_tokens": 150
}
```
