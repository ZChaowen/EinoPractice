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

func main() {
	// -------------------- 0. 解析命令行参数 --------------------
	configPath := flag.String("config", "config.yml", "配置文件路径")
	flag.Parse()

	// -------------------- 1. 加载配置 --------------------
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("配置加载成功: base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)

	ctx := context.Background()

	// -------------------- 2. 创建 ChatModel --------------------
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

	// -------------------- 3. 带重试的生成 --------------------
	response, err := generateWithRetry(ctx, chatModel, messages, 3)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Fatalf("请求超时")
		}
		log.Fatalf("生成失败: %v", err)
	}

	fmt.Printf("成功! 回答: %s\n", response.Content)

	// -------------------- 4. 输出 Token 使用统计 --------------------
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		fmt.Printf("\nToken 使用统计:\n")
		fmt.Printf("  输入 Token: %d\n", response.ResponseMeta.Usage.PromptTokens)
		fmt.Printf("  输出 Token: %d\n", response.ResponseMeta.Usage.CompletionTokens)
		fmt.Printf("  总计 Token: %d\n", response.ResponseMeta.Usage.TotalTokens)
	}
}
