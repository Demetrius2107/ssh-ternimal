// Package ai AI 辅助终端: Provider 抽象 + DeepSeek/Ollama 流式实现
//
// 设计: AiProvider 接口统一对话能力, 流式输出通过 onDelta 回调逐段返回,
// 由上层 (app.go) 转发为 wails 事件给前端流式渲染。
// 纯标准库实现 (net/http + bufio 解析 SSE), 不引入外部依赖。
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider 名称常量 (前端配置/展示用)
const (
	ProviderDeepSeek = "deepseek"
	ProviderOllama   = "ollama"
)

// Model 档位 (成本控制: 便宜/深度)
const (
	ModelDeepSeekChat     = "deepseek-chat"
	ModelDeepSeekReasoner = "deepseek-reasoner"
)

// Message 对话消息 (角色 + 内容)
type Message struct {
	Role    string `json:"role"` // system / user / assistant
	Content string `json:"content"`
}

// AiProvider AI 服务接口
type AiProvider interface {
	// Chat 发送对话请求, 流式输出通过 onDelta 回调 (每段为增量文本)
	// ctx 取消即中断请求 (成本控制: 流式中断)
	Chat(ctx context.Context, model string, messages []Message, onDelta func(string)) error
}

// NewProvider 按名称创建 Provider
// deepseek: 需要 apiKey (从 keyring 读取); ollama: 本地服务, 无需密钥
func NewProvider(name, apiKey string) (AiProvider, error) {
	switch name {
	case ProviderDeepSeek:
		if strings.TrimSpace(apiKey) == "" {
			return nil, errors.New("请先在设置中配置 DeepSeek API Key")
		}
		return &deepseekProvider{apiKey: strings.TrimSpace(apiKey)}, nil
	case ProviderOllama:
		return &ollamaProvider{}, nil
	default:
		return nil, fmt.Errorf("未知 AI Provider: %s", name)
	}
}

// httpClient 共享 HTTP 客户端 (超时 120s, 流式读取靠 ctx 控制)
var httpClient = &http.Client{Timeout: 120 * time.Second}

// ---------- DeepSeek (OpenAI 兼容 API, SSE 流式) ----------

const deepseekURL = "https://api.deepseek.com/chat/completions"

type deepseekProvider struct {
	apiKey string
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// chatResponse SSE 中的单条增量消息
type chatResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (p *deepseekProvider) Chat(ctx context.Context, model string, messages []Message, onDelta func(string)) error {
	body, err := json.Marshal(chatRequest{Model: model, Messages: messages, Stream: true})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deepseekURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 DeepSeek 失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("DeepSeek 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return parseSSE(ctx, resp.Body, func(data string) error {
		if data == "[DONE]" {
			return nil
		}
		var cr chatResponse
		if err := json.Unmarshal([]byte(data), &cr); err != nil {
			return nil // 忽略无法解析的行 (如 usage 段)
		}
		for _, c := range cr.Choices {
			if c.Delta.Content != "" {
				onDelta(c.Delta.Content)
			}
		}
		return nil
	})
}

// ---------- Ollama (本地服务, SSE 流式) ----------

const ollamaURL = "http://localhost:11434/api/chat"

type ollamaProvider struct{}

type ollamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// ollamaResponse 单条增量 (done=true 表示结束)
type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (p *ollamaProvider) Chat(ctx context.Context, model string, messages []Message, onDelta func(string)) error {
	if strings.TrimSpace(model) == "" {
		model = "qwen2.5" // Ollama 默认模型
	}
	body, err := json.Marshal(ollamaRequest{Model: model, Messages: messages, Stream: true})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Ollama 失败: %v (本地服务是否已启动?)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("Ollama 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return parseSSE(ctx, resp.Body, func(data string) error {
		var or ollamaResponse
		if err := json.Unmarshal([]byte(data), &or); err != nil {
			return nil
		}
		if or.Message.Content != "" {
			onDelta(or.Message.Content)
		}
		return nil
	})
}

// parseSSE 逐行解析 SSE 流: 收集 "data: xxx" 行, 空行分隔事件后回调
// ctx 取消时提前返回 (流式中断)
func parseSSE(ctx context.Context, r io.Reader, onData func(string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var buf strings.Builder
	flush := func() error {
		if buf.Len() == 0 {
			return nil
		}
		data := strings.TrimSpace(buf.String())
		buf.Reset()
		if data == "" {
			return nil
		}
		return onData(data)
	}
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err() // 流式中断
		default:
		}
		line := sc.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			buf.WriteString(strings.TrimPrefix(line, "data:"))
			buf.WriteString("\n")
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("读取流失败: %v", err)
	}
	return flush()
}
