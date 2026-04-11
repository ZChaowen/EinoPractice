package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

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
	BaseURL   string  `yaml:"base_url"`
	APIKey    string  `yaml:"api_key"`
	ModelName string  `yaml:"model_name"`
	Timeout   int     `yaml:"timeout"` // 超时时间(秒)
	Temperature float64 `yaml:"temperature"` // 控制输出随机性
	TopP       float64 `yaml:"top_p"` // 核采样参数
	MaxTokens  int     `yaml:"max_tokens"` // 最大生成token数
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
	configPath := flag.String("config", "config.yml", "配置文件路径")
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
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("配置加载成功: base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)

	// -------------------- 2. 初始化 --------------------
	ctx := context.Background()

	// 创建 ChatModel
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.Model.APIKey,
		Model:   cfg.Model.ModelName,
		BaseURL: cfg.Model.BaseURL,
	})
	if err != nil {
		log.Printf("[ERROR] 创建 ChatModel 失败: %v", err)
		panicIfErr(err, "创建 ChatModel 失败")
	}
	log.Printf("ChatModel 创建成功")

	// -------------------- 3. 构建消息 --------------------
	messages := []*schema.Message{
		schema.SystemMessage("你是一个懂得哲学的程序员。"),
		schema.UserMessage("什么是存在主义？"),
	}
	log.Printf("消息构建完成，共 %d 条消息", len(messages))

	// -------------------- 4. 生成响应 --------------------
	log.Printf("开始生成响应...")
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("[ERROR] 生成响应失败: %v", err)
		panicIfErr(err, "生成响应失败")
	}

	// -------------------- 5. 输出结果 --------------------
	fmt.Printf("回答:\n%s\n", response.Content)
	log.Printf("响应生成成功")

	// 输出 Token 使用统计
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		fmt.Printf("\nToken 使用统计:\n")
		fmt.Printf("  输入 Token: %d\n", response.ResponseMeta.Usage.PromptTokens)
		fmt.Printf("  输出 Token: %d\n", response.ResponseMeta.Usage.CompletionTokens)
		fmt.Printf("  总计 Token: %d\n", response.ResponseMeta.Usage.TotalTokens)
		log.Printf("Token 使用统计: prompt=%d, completion=%d, total=%d",
			response.ResponseMeta.Usage.PromptTokens,
			response.ResponseMeta.Usage.CompletionTokens,
			response.ResponseMeta.Usage.TotalTokens)
	}
}