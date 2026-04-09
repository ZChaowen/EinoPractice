package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	_ "github.com/ZChaowen/EinoPractice/lab02/chain/docs"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gopkg.in/yaml.v2"
)

// Config 模型配置
type Config struct {
	Model ModelConfig `yaml:"model"`
	App   AppConfig   `yaml:"app"`
}

// ModelConfig 大模型配置
type ModelConfig struct {
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	ModelName string `yaml:"model_name"`
}

// AppConfig 应用配置
type AppConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// ChainRequest Chain请求
//
//	@Description	Chain聊天请求结构
type ChainRequest struct {
	// UserQuery 用户查询内容
	UserQuery string `json:"user_query" example:"我叫morning，邮箱是lumworn@gmail.com，帮我制定训练计划" description:"用户查询内容"`
}

// ChainResponse Chain响应
//
//	@Description	Chain聊天响应结构
type ChainResponse struct {
	// Content 模型回复内容
	Content string `json:"content" example:"根据您的信息，我们为您推荐..." description:"模型回复内容"`
	// ReasoningContent 思考内容（如果有）
	ReasoningContent string `json:"reasoning_content,omitempty" example:"" description:"思考内容"`
	// PromptTokens 输入token数量
	PromptTokens int `json:"prompt_tokens" example:"100" description:"输入token数量"`
	// OutputTokens 输出token数量
	OutputTokens int `json:"output_tokens" example:"50" description:"输出token数量"`
	// TotalTokens 总token数量
	TotalTokens int `json:"total_tokens" example:"150" description:"总token数量"`
}

// 工具入参
type playerInfoRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// 工具出参
type playerInfoResponse struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	HeightCM    int    `json:"height_cm"`
	WeightKG    int    `json:"weight_kg"`
	PlayStyle   string `json:"play_style"`
	WeeklyHours int    `json:"weekly_hours"`
}

var (
	cfg      *Config                                           // 全局配置实例
	chain    *compose.Chain[map[string]any, *schema.Message]   // 全局Chain实例
	runnable compose.Runnable[map[string]any, *schema.Message] // 全局编译后的Chain实例
)

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

// initChain 初始化Chain
func initChain(cfg *Config) error {
	ctx := context.Background()

	// 1) 篮球主题：ChatTemplate
	systemTpl := `你是一名篮球教练与比赛分析师。你需要结合用户的基本信息与训练习惯，
使用 player_info 工具补全信息，然后给出适合他的训练计划/位置建议/一套简单战术建议。
注意：邮箱必须出现，用于查询信息。`

	chatTpl := prompt.FromMessages(schema.FString,
		schema.SystemMessage(systemTpl),
		schema.MessagesPlaceholder("histories", true),
		schema.UserMessage("{user_query}"),
	)

	// 2) 推荐模板
	recommendTpl := `
你是一名篮球教练与比赛分析师。请结合工具返回的用户信息，为用户输出建议，要求具体、可执行。

--- 训练资源（可选方案库）---

### A. 训练方向库（按位置/风格）
**1. 后卫（控运与节奏）**
- 核心：运球对抗、挡拆阅读、急停跳投、突破分球
- 训练：左右手变向组合、弱侧手终结、1v1 变速

**2. 锋线（持球终结与防守）**
- 核心：三威胁、低位脚步、协防轮转、错位单打
- 训练：三分接投+一运、背身转身、closeout 防守

**3. 内线（篮下统治与护框）**
- 核心：卡位、顺下吃饼、护框、二次进攻
- 训练：对抗上篮、掩护质量、篮板站位

### B. 一套简单战术（适合大多数业余队）
- **高位挡拆（P&R）**：持球人借掩护突破/投篮/分球，弱侧埋伏投手
- **Spain P&R（简化版）**：挡拆后再给顺下人做背掩护，制造错位/空切
- **5-out（五外）**：拉开空间，强弱侧转移球，靠突破分球创造空位三分

### C. 输出规则
1) 先总结用户画像（身高体重、风格、每周训练时长）
2) 给出建议位置与核心技能树（3-5个技能）
3) 输出一周训练计划（按天、每次45-90分钟）
4) 给一套战术建议 + 业余局实战注意事项（3条）

`

	// 3) 创建 DeepSeek ChatModel
	chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  cfg.Model.APIKey,
		Model:   cfg.Model.ModelName,
		BaseURL: cfg.Model.BaseURL,
	})
	if err != nil {
		return fmt.Errorf("创建 ChatModel 实例失败: %w", err)
	}

	// 4) 创建工具：player_info
	playerInfoTool := utils.NewTool(
		&schema.ToolInfo{
			Name: "player_info",
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

	// 5) 绑定工具到模型
	info, err := playerInfoTool.Info(ctx)
	if err != nil {
		return fmt.Errorf("获取工具信息失败: %w", err)
	}
	if err := chatModel.BindTools([]*schema.ToolInfo{info}); err != nil {
		return fmt.Errorf("绑定工具失败: %w", err)
	}

	// 6) ToolsNode
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{playerInfoTool},
	})
	if err != nil {
		return fmt.Errorf("创建 ToolsNode 失败: %w", err)
	}

	// 7) lambda：从 toolsNode 输出中提取"工具返回内容"
	toolToTextOps := func(
		ctx context.Context,
		input *schema.StreamReader[[]*schema.Message],
	) (output *schema.StreamReader[*schema.Message], err error) {
		return schema.StreamReaderWithConvert(input, func(msgs []*schema.Message) (*schema.Message, error) {
			var toolContents []string
			for _, m := range msgs {
				if m == nil {
					continue
				}
				if m.Role == "tool" {
					toolContents = append(toolContents, m.Content)
				}
			}

			text := "工具未返回有效信息。"
			if len(toolContents) > 0 {
				text = "工具返回的用户信息如下：\n- " + toolContents[0]
				for i := 1; i < len(toolContents); i++ {
					text += "\n- " + toolContents[i]
				}
			}

			return schema.UserMessage(text), nil
		}), nil
	}
	lambdaToolToText := compose.TransformableLambda[[]*schema.Message, *schema.Message](toolToTextOps)

	// 8) lambda：构造第二次模型的输入
	promptTransformOps := func(
		ctx context.Context,
		input *schema.StreamReader[*schema.Message],
	) (output *schema.StreamReader[[]*schema.Message], err error) {
		return schema.StreamReaderWithConvert(input, func(m *schema.Message) ([]*schema.Message, error) {
			out := make([]*schema.Message, 0, 2)
			out = append(out, schema.SystemMessage(recommendTpl))
			out = append(out, m)
			return out, nil
		}), nil
	}
	lambdaPrompt := compose.TransformableLambda[*schema.Message, []*schema.Message](promptTransformOps)

	// 9) Chain 编排
	chain = compose.NewChain[map[string]any, *schema.Message]()
	chain.
		AppendChatTemplate(chatTpl).
		AppendChatModel(chatModel).
		AppendToolsNode(toolsNode).
		AppendLambda(lambdaToolToText).
		AppendLambda(lambdaPrompt).
		AppendChatModel(chatModel)

	log.Printf("Chain 编排完成")

	return nil
}

// compileChain 编译Chain
func compileChain() error {
	ctx := context.Background()
	r, err := chain.Compile(ctx)
	if err != nil {
		return fmt.Errorf("编译 Chain 失败: %w", err)
	}
	runnable = r
	log.Printf("Chain 编译成功")
	return nil
}

// chainHandler 处理Chain请求
//
//	@Summary		Chain聊天接口
//	@Description	基于Chain编排的篮球教练助手
//	@Tags			chain
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ChainRequest	true	"Chain请求"
//	@Success		200		{object}	ChainResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/chain [post]
func chainHandler(c *gin.Context) {
	var req ChainRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	output, err := runnable.Invoke(ctx, map[string]any{
		"histories":  []*schema.Message{},
		"user_query": req.UserQuery,
	})
	if err != nil {
		log.Printf("[ERROR] Chain调用失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成响应失败，请稍后重试"})
		return
	}

	resp := ChainResponse{
		Content:          output.Content,
		ReasoningContent: output.ReasoningContent,
	}

	if output.ResponseMeta != nil && output.ResponseMeta.Usage != nil {
		resp.PromptTokens = output.ResponseMeta.Usage.PromptTokens
		resp.OutputTokens = output.ResponseMeta.Usage.CompletionTokens
		resp.TotalTokens = output.ResponseMeta.Usage.TotalTokens
	}

	c.JSON(http.StatusOK, resp)
}

// healthHandler 健康检查
//
//	@Summary		健康检查
//	@Description	检查服务是否正常运行
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Router			/health [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// recoveryMiddleware panic恢复中间件
func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC RECOVERED] 异常信息: %v\n堆栈跟踪:\n%s", r, getStackTrace())
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "服务器内部错误，请稍后重试",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// getStackTrace 获取堆栈跟踪
func getStackTrace() string {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	return string(buf[:n])
}

// panicIfErr 如果错误不为nil，则抛出panic
func panicIfErr(err error, msg string) {
	if err != nil {
		log.Printf("[PANIC THROW] %s: %v", msg, err)
		panic(fmt.Sprintf("%s: %v", msg, err))
	}
}

func main() {
	// -------------------- 0. 解析命令行参数 --------------------
	logFile := flag.String("log", "", "日志输出文件路径（留空则输出到标准输出）")
	flag.Parse()

	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("打开日志文件失败: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("日志将输出到文件: %s", *logFile)
	} else {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("日志将输出到标准输出")
	}

	// -------------------- 1. 加载配置 --------------------
	configPath := "config.yml"
	var err error
	cfg, err = loadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("配置加载成功: base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)

	// -------------------- 2. 初始化Chain --------------------
	if err := initChain(cfg); err != nil {
		log.Fatalf("初始化Chain失败: %v", err)
	}

	// -------------------- 3. 编译Chain --------------------
	if err := compileChain(); err != nil {
		log.Fatalf("编译Chain失败: %v", err)
	}

	// -------------------- 4. 设置Gin路由 --------------------
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(recoveryMiddleware())

	r.GET("/health", healthHandler)
	r.POST("/chain", chainHandler)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// -------------------- 5. 启动服务 --------------------
	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	log.Printf("服务启动中，监听地址: %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
