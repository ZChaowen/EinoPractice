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
	"github.com/cloudwego/eino/components/model"
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

// -----------------------------
// 1) 自定义 Options（实现特定）
// -----------------------------
type MyChatModelOptions struct {
	RetryCount int
	Timeout    time.Duration
}

// -----------------------------
// 2) 自定义 Option：WithRetryCount / WithTimeout
// -----------------------------
func WithRetryCount(count int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *MyChatModelOptions) {
		o.RetryCount = count
	})
}

func WithTimeout(timeout time.Duration) model.Option {
	return model.WrapImplSpecificOptFn(func(o *MyChatModelOptions) {
		o.Timeout = timeout
	})
}

// -----------------------------
// 3) Callback 机制
// -----------------------------
type CallbackInput struct {
	Model    string
	Messages []*schema.Message
	Extra    map[string]any
}

type CallbackOutput struct {
	Message *schema.Message
	Usage   *schema.TokenUsage
	Extra   map[string]any
}

type ChatCallback interface {
	OnStart(ctx context.Context, in CallbackInput)
	OnEnd(ctx context.Context, out CallbackOutput, err error)
}

type LoggingCallback struct{}

func (LoggingCallback) OnStart(ctx context.Context, in CallbackInput) {
	fmt.Println("== [callback] start ==")
	fmt.Println("model:", in.Model)
	for i, m := range in.Messages {
		fmt.Printf("  [%d] role=%s content=%q\n", i, m.Role, m.Content)
	}
	if len(in.Extra) > 0 {
		fmt.Println("extra:", in.Extra)
	}
}

func (LoggingCallback) OnEnd(ctx context.Context, out CallbackOutput, err error) {
	fmt.Println("== [callback] end ==")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if out.Message != nil {
		fmt.Println("assistant:", out.Message.Content)
	}
	if out.Usage != nil {
		fmt.Printf("usage: prompt=%d completion=%d total=%d\n",
			out.Usage.PromptTokens, out.Usage.CompletionTokens, out.Usage.TotalTokens)
	}
	if len(out.Extra) > 0 {
		fmt.Println("extra:", out.Extra)
	}
}

// -----------------------------
// 4) WrappedChatModel：在 Generate 中应用 options + callbacks + retry/timeout
// -----------------------------
type WrappedChatModel struct {
	inner     *openai.ChatModel
	modelName string
	callbacks []ChatCallback
}

func NewWrappedChatModel(cfg *Config, callbacks ...ChatCallback) (*WrappedChatModel, error) {
	if cfg.Model.APIKey == "" {
		return nil, errors.New("missing API_KEY")
	}

	timeout := time.Duration(cfg.Model.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	temperature := float32(cfg.Model.Temperature)
	maxTokens := cfg.Model.MaxTokens

	inner, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		APIKey:     cfg.Model.APIKey,
		Model:      cfg.Model.ModelName,
		BaseURL:    cfg.Model.BaseURL,
		Timeout:    timeout,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, err
	}

	return &WrappedChatModel{
		inner:     inner,
		modelName: cfg.Model.ModelName,
		callbacks: callbacks,
	}, nil
}

func (m *WrappedChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// 1) 使用公开 API 解析 options（通用 + 实现特定）
	_ = model.GetCommonOptions(nil, opts...) // 通用选项（Temperature, MaxTokens 等由 inner 模型处理）
	myOpts := model.GetImplSpecificOptions(&MyChatModelOptions{}, opts...)

	// 2) timeout（用 option 创建子 context）
	callCtx := ctx
	cancel := func() {}
	if myOpts.Timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, myOpts.Timeout)
		defer cancel()
	}

	// 3) callback start
	in := CallbackInput{
		Model:    m.modelName,
		Messages: messages,
		Extra: map[string]any{
			"retry_count": myOpts.RetryCount,
			"timeout":     myOpts.Timeout.String(),
		},
	}
	for _, cb := range m.callbacks {
		cb.OnStart(callCtx, in)
	}

	// 4) retry
	var (
		resp    *schema.Message
		usage   *schema.TokenUsage
		lastErr error
	)
	start := time.Now()

	retries := myOpts.RetryCount
	if retries < 0 {
		retries = 0
	}

	for attempt := 0; attempt <= retries; attempt++ {
		r, err := m.inner.Generate(callCtx, messages)
		if err == nil {
			resp = r
			if r != nil && r.ResponseMeta != nil && r.ResponseMeta.Usage != nil {
				usage = r.ResponseMeta.Usage
			}
			lastErr = nil
			break
		}
		lastErr = err

		// ctx 超时/取消就别重试
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.Canceled) {
			break
		}
		// 简单退避
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}

	// 5) callback end
	out := CallbackOutput{
		Message: resp,
		Usage:   usage,
		Extra: map[string]any{
			"latency_ms": time.Since(start).Milliseconds(),
		},
	}
	for _, cb := range m.callbacks {
		cb.OnEnd(callCtx, out, lastErr)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return resp, nil
}

// -----------------------------
// 5) main：运行 demo
// -----------------------------
func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.yml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("配置加载成功: base_url=%s, model=%s", cfg.Model.BaseURL, cfg.Model.ModelName)

	// 创建 WrappedChatModel
	w, err := NewWrappedChatModel(cfg, LoggingCallback{})
	if err != nil {
		log.Fatal(err)
	}

	msgs := []*schema.Message{
		schema.SystemMessage("你是一个简洁、专业的篮球教练。"),
		schema.UserMessage("我每周打球2次，想提升运球和终结，请给我一周训练计划。"),
	}

	_, err = w.Generate(context.Background(), msgs,
		WithRetryCount(2),
		WithTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatal("generate failed:", err)
	}
}
