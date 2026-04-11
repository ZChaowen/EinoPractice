package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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

	// -------------------- 3. 构建消息 --------------------
	messages := []*schema.Message{
		schema.SystemMessage("你是一个懂得哲学的程序员。"),
		schema.UserMessage("什么是存在主义？"),
	}

	// -------------------- 4. 流式生成 --------------------
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		log.Fatalf("流式生成失败: %v", err)
	}
	defer stream.Close()

	fmt.Print("AI 回复: ")

	// 逐块接收并打印
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// 流结束
				break
			}
			log.Fatalf("接收失败: %v", err)
		}

		// 打印内容（打字机效果）
		fmt.Print(chunk.Content)
	}

	fmt.Println("\n\n完成！")
}
