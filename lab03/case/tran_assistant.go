package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
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

// Translator 基于大模型的翻译助手
type Translator struct {
	chatModel *openai.ChatModel
}

// TranslatorConfig 翻译器配置
type TranslatorConfig struct {
	APIKey   string
	Model    string
	BaseURL  string
	Timeout  time.Duration
	Retries  int
}

// NewTranslator 创建一个新的翻译器实例
func NewTranslator(cfg TranslatorConfig) (*Translator, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("missing api key")
	}
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

// Translate 翻译文本到目标语言
func (t *Translator) Translate(ctx context.Context, text, targetLang string) (string, error) {
	text = strings.TrimSpace(text)
	targetLang = strings.TrimSpace(targetLang)
	if text == "" {
		return "", errors.New("empty text")
	}
	if targetLang == "" {
		return "", errors.New("empty target language")
	}

	// 超时控制
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 更严格的提示词：只输出译文；保留格式；不添加引号/解释
	system := fmt.Sprintf(
		"你是一个专业翻译引擎。将用户输入翻译成%s。"+
			"要求：只输出译文，不要解释；保留原有换行与列表格式；不要添加引号；不要输出多余内容。",
		targetLang,
	)

	messages := []*schema.Message{
		schema.SystemMessage(system),
		schema.UserMessage(text),
	}

	// 简单重试
	var lastErr error
	for attempt := 0; attempt <= 2; attempt++ {
		resp, err := t.chatModel.Generate(ctx, messages)
		if err == nil {
			return strings.TrimSpace(resp.Content), nil
		}
		lastErr = err

		// ctx 超时/取消就别重试
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			break
		}

		// 退避
		time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
	}
	return "", lastErr
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

	// -------------------- 2. 创建翻译器 --------------------
	translator, err := NewTranslator(TranslatorConfig{
		APIKey:  cfg.Model.APIKey,
		Model:   cfg.Model.ModelName,
		BaseURL: cfg.Model.BaseURL,
		Timeout: time.Duration(cfg.Model.Timeout) * time.Second,
		Retries: 2,
	})
	if err != nil {
		log.Fatalf("创建翻译器失败: %v", err)
	}

	// -------------------- 3. 测试翻译 --------------------
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
		if err != nil {
			log.Printf("翻译失败: %v", err)
			continue
		}
		fmt.Printf("原文: %s\n翻译: %s\n\n", item.content, result)
	}
}
