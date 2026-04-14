package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/prompt"
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

// PromptTemplate 提示词模板管理
type PromptTemplates struct{}

// 翻译助手模板
func (p *PromptTemplates) Translator(sourceLang, targetLang string) prompt.ChatTemplate {
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(fmt.Sprintf(
			"你是一个专业的翻译助手。请将%s翻译成%s。\n"+
				"要求：\n"+
				"1. 保持原文的语气和风格\n"+
				"2. 确保翻译准确、流畅\n"+
				"3. 只返回翻译结果，不要添加解释",
			sourceLang, targetLang,
		)),
		schema.UserMessage("{text}"),
	)
}

// 代码审查模板
func (p *PromptTemplates) CodeReviewer(language string) prompt.ChatTemplate {
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(fmt.Sprintf(
			"你是一个资深的%s开发专家。请审查以下代码，并提供：\n"+
				"1. 潜在的bug或问题\n"+
				"2. 性能优化建议\n"+
				"3. 代码风格改进建议\n"+
				"4. 安全性评估",
			language,
		)),
		schema.UserMessage("请审查以下代码：\n\n```{language}\n{code}\n```"),
	)
}

// 技术面试官模板
func (p *PromptTemplates) TechInterviewer(position, level string) prompt.ChatTemplate {
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(fmt.Sprintf(
			"你是一位%s职位的面试官，针对%s级别的候选人。\n"+
				"请根据候选人的回答：\n"+
				"1. 评估答案的准确性和深度\n"+
				"2. 提出有针对性的追问\n"+
				"3. 给出建设性的反馈",
			position, level,
		)),
		schema.UserMessage("候选人回答：{answer}\n\n请评估并追问。"),
	)
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
		log.Fatalf("创建模型失败: %v", err)
	}

	templates := &PromptTemplates{}

	// -------------------- 3. 示例1: 使用翻译模板 --------------------
	fmt.Println("===== 翻译示例 =====")
	translatorTemplate := templates.Translator("中文", "英文")
	messages, _ := translatorTemplate.Format(ctx, map[string]any{
		"text": "Eino 是一个强大的 AI 开发框架",
	})
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("翻译失败: %v", err)
	} else {
		fmt.Printf("翻译结果: %s\n\n", response.Content)
	}

	// -------------------- 4. 示例2: 使用代码审查模板 --------------------
	fmt.Println("===== 代码审查示例 =====")
	reviewerTemplate := templates.CodeReviewer("Go")
	messages, _ = reviewerTemplate.Format(ctx, map[string]any{
		"language": "go",
		"code": `func add(a, b int) int {
	return a + b
}`,
	})
	response, err = chatModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("代码审查失败: %v", err)
	} else {
		fmt.Printf("审查结果:\n%s\n\n", response.Content)
	}

	// -------------------- 5. 示例3: 使用面试官模板 --------------------
	fmt.Println("===== 面试官示例 =====")
	interviewerTemplate := templates.TechInterviewer("Go后端开发", "中级")
	messages, _ = interviewerTemplate.Format(ctx, map[string]any{
		"answer": "goroutine 是 Go 语言的轻量级线程，由 Go 运行时管理",
	})
	response, err = chatModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("面试失败: %v", err)
	} else {
		fmt.Printf("面试官反馈:\n%s\n\n", response.Content)
	}

	// -------------------- 6. 输出 Token 使用统计 --------------------
	if response != nil && response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		fmt.Printf("Token 使用统计:\n")
		fmt.Printf("  输入 Token: %d\n", response.ResponseMeta.Usage.PromptTokens)
		fmt.Printf("  输出 Token: %d\n", response.ResponseMeta.Usage.CompletionTokens)
		fmt.Printf("  总计 Token: %d\n", response.ResponseMeta.Usage.TotalTokens)
	}
}
