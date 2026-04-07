package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/ZChaowen/EinoPractice/lab01/docs"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
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

// ChatRequest 聊天请求
//
//	@Description	聊天请求结构
type ChatRequest struct {
	// SystemPrompt 系统提示词，用于设定 AI 角色和行为
	SystemPrompt string `json:"system_prompt" example:"你是一个有帮助的助手" description:"系统提示词"`
	// UserMessage 用户消息，即用户的输入
	UserMessage string `json:"user_message" example:"你好" description:"用户消息"`
}

// ChatResponse 聊天响应
//
//	@Description	聊天响应结构
type ChatResponse struct {
	// Content 模型回复内容
	Content string `json:"content" example:"你好！有什么可以帮助你的吗？" description:"模型回复内容"`
	// PromptTokens 输入 token 数量
	PromptTokens int `json:"prompt_tokens" example:"100" description:"输入 token 数量"`
	// OutputTokens 输出 token 数量
	OutputTokens int `json:"output_tokens" example:"50" description:"输出 token 数量"`
	// TotalTokens 总 token 数量
	TotalTokens int `json:"total_tokens" example:"150" description:"总 token 数量"`
}

var (
	cfg       *Config             // 全局配置实例
	chatModel *deepseek.ChatModel // 全局聊天模型实例（并发安全）
)

// loadConfig 加载配置文件
//
//	@Description	从指定路径读取 YAML 配置文件并解析为 Config 结构体
func loadConfig(configPath string) (*Config, error) {
	// 读取配置文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML 格式的配置
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// initChatModel 初始化聊天模型
//
//	@Description	根据配置创建 DeepSeek ChatModel 实例
func initChatModel(cfg *Config) error {
	ctx := context.Background()

	// 创建 DeepSeek 聊天模型实例
	model, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  cfg.Model.APIKey,    // API 密钥
		Model:   cfg.Model.ModelName, // 模型名称
		BaseURL: cfg.Model.BaseURL,   // API 基础地址
	})
	if err != nil {
		return fmt.Errorf("创建 ChatModel 实例失败: %w", err)
	}

	chatModel = model
	return nil
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
	var req ChatRequest

	// 绑定并验证 JSON 请求体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 构建消息列表：系统提示 + 用户消息
	messages := []*schema.Message{
		schema.SystemMessage(req.SystemPrompt),
		schema.UserMessage(req.UserMessage),
	}

	// 创建带超时的 context，防止模型调用耗时过长
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 调用聊天模型生成响应
	// 注意：deepseek.ChatModel 本身是并发安全的，无需额外加锁
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		// 仅返回通用错误信息，避免泄露内部实现细节
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成响应失败，请稍后重试"})
		return
	}

	// 构建响应结构
	resp := ChatResponse{
		Content: response.Content,
	}

	// 提取 token 使用统计信息
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		resp.PromptTokens = response.ResponseMeta.Usage.PromptTokens
		resp.OutputTokens = response.ResponseMeta.Usage.CompletionTokens
		resp.TotalTokens = response.ResponseMeta.Usage.TotalTokens
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
	// 返回服务正常运行状态
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func main() {
	// -------------------- 1. 加载配置 --------------------
	configPath := "config.yml"
	var err error
	cfg, err = loadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("配置加载成功: base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)

	// -------------------- 2. 初始化聊天模型 --------------------
	if err := initChatModel(cfg); err != nil {
		log.Fatalf("初始化聊天模型失败: %v", err)
	}
	log.Println("聊天模型初始化成功")

	// -------------------- 3. 设置 Gin 路由 --------------------
	// 使用 ReleaseMode 减少日志输出
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 注册 HTTP 处理函数
	r.GET("/health", healthHandler) // 健康检查
	r.POST("/chat", chatHandler)    // 聊天接口

	// 注册 Swagger UI（用于 API 调试）
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// -------------------- 4. 启动服务 --------------------
	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	log.Printf("服务启动中，监听地址: %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
