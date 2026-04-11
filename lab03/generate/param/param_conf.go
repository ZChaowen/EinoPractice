package main

import (
	"context"
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

	// 示例1: 基础配置
	fmt.Println("=== 示例1: 基础配置 ===")
	basicExample(ctx, cfg)

	// 示例2: 高级配置
	fmt.Println("\n=== 示例2: 高级配置 ===")
	advancedExample(ctx, cfg)

	// 示例3: 创意写作配置
	fmt.Println("\n=== 示例3: 创意写作配置 ===")
	creativeExample(ctx, cfg)
}

// basicExample 基础配置示例
func basicExample(ctx context.Context, cfg *Config) {
	timeout := time.Duration(cfg.Model.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	temperature := float32(cfg.Model.Temperature)
	maxTokens := cfg.Model.MaxTokens

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.Model.APIKey,
		Model:   cfg.Model.ModelName,
		BaseURL: cfg.Model.BaseURL,
		Timeout: timeout,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		log.Fatalf("创建失败: %v", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage("你是一个友好的 AI 助手"),
		schema.UserMessage("用一句话介绍 Eino 框架"),
	}

	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("生成失败: %v", err)
	}

	fmt.Printf("AI 响应: %s\n", response.Content)
	printTokenUsage(response)
}

// advancedExample 高级配置示例 - 精确控制输出
func advancedExample(ctx context.Context, cfg *Config) {
	timeout := time.Duration(cfg.Model.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	temperature := float32(0.7)
	topP := float32(0.9)
	maxTokens := 500
	presencePenalty := float32(0.6)
	frequencyPenalty := float32(0.5)

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.Model.APIKey,
		Model:   cfg.Model.ModelName,
		BaseURL: cfg.Model.BaseURL,
		Timeout: timeout,

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
	if err != nil {
		log.Fatalf("创建失败: %v", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage("你是一个专业的技术文档撰写专家"),
		schema.UserMessage("详细介绍 Eino 框架的核心特性，包括架构、组件和优势"),
	}

	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("生成失败: %v", err)
	}

	fmt.Printf("AI 响应: %s\n", response.Content)
	printTokenUsage(response)
}

// creativeExample 创意写作配置示例 - 高随机性
func creativeExample(ctx context.Context, cfg *Config) {
	timeout := time.Duration(cfg.Model.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	temperature := float32(1.2)
	topP := float32(0.95)
	maxTokens := 800
	presencePenalty := float32(0.3)
	frequencyPenalty := float32(0.3)

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.Model.APIKey,
		Model:   cfg.Model.ModelName,
		BaseURL: cfg.Model.BaseURL,
		Timeout: timeout,

		// 高温度设置，适合创意写作
		Temperature: &temperature,
		TopP:        &topP,
		MaxTokens:   &maxTokens,

		// 减少重复惩罚，允许一定的重复（适合故事情节）
		PresencePenalty:  &presencePenalty,
		FrequencyPenalty: &frequencyPenalty,
	})
	if err != nil {
		log.Fatalf("创建失败: %v", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage("你是一个富有创造力的故事作家"),
		schema.UserMessage("创作一个关于 AI 框架变成超级英雄的有趣故事开头"),
	}

	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("生成失败: %v", err)
	}

	fmt.Printf("AI 响应: %s\n", response.Content)
	printTokenUsage(response)
}

// printTokenUsage 打印 Token 使用情况
func printTokenUsage(response *schema.Message) {
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		fmt.Printf("\nToken 使用统计:\n")
		fmt.Printf("  输入 Token: %d\n", response.ResponseMeta.Usage.PromptTokens)
		fmt.Printf("  输出 Token: %d\n", response.ResponseMeta.Usage.CompletionTokens)
		fmt.Printf("  总计 Token: %d\n", response.ResponseMeta.Usage.TotalTokens)
		if response.ResponseMeta.Usage.PromptTokenDetails.CachedTokens > 0 {
			fmt.Printf("  缓存 Token: %d\n", response.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
		}
	}
}
