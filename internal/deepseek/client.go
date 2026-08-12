package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"quan-agent/internal/config"
	"quan-agent/internal/ops"
)

const systemPrompt = `你是 Quan Agent，一个运行在用户本机 Windows 上的专业运维助手。
回答必须基于提供的实时主机快照；无法从快照判断时应明确说明并给出只读排查步骤，不得编造。
默认使用简体中文，输出简洁、可执行。命令优先使用 PowerShell。
你只能提供诊断和建议，不能声称已经执行命令、修改配置、重启服务或完成修复。
涉及删除、服务启停、权限、注册表、网络防火墙、安装卸载等高风险动作时，必须说明影响并要求用户确认。`

type SettingsProvider interface {
	Get() config.Settings
}

type Client struct {
	settings SettingsProvider
	toolbox  *ops.Toolbox
	http     *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

type apiResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewClient(settings SettingsProvider, toolbox *ops.Toolbox) *Client {
	return &Client{
		settings: settings,
		toolbox:  toolbox,
		http:     &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *Client) Chat(ctx context.Context, history []Message) (string, error) {
	settings := c.settings.Get()
	if settings.APIKey == "" {
		return "", errors.New("请先在设置中填写 DeepSeek API Key")
	}
	if len(history) == 0 {
		return "", errors.New("消息不能为空")
	}
	if len(history) > 20 {
		history = history[len(history)-20:]
	}

	messages := make([]Message, 0, len(history)+2)
	messages = append(messages,
		Message{Role: "system", Content: systemPrompt},
		Message{Role: "system", Content: "当前主机只读快照：\n" + c.toolbox.SnapshotJSON(ctx)},
	)
	for _, message := range history {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if (role != "user" && role != "assistant") || content == "" {
			continue
		}
		if len(content) > 8000 {
			content = content[:8000]
		}
		messages = append(messages, Message{Role: role, Content: content})
	}

	body, err := json.Marshal(apiRequest{
		Model: settings.Model, Messages: messages, Temperature: 0.2, Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("构造请求: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.BaseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+settings.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 DeepSeek 失败: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("读取 DeepSeek 响应: %w", err)
	}
	var result apiResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("解析 DeepSeek 响应失败，HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := http.StatusText(resp.StatusCode)
		if result.Error != nil && result.Error.Message != "" {
			message = result.Error.Message
		}
		return "", fmt.Errorf("DeepSeek 返回 HTTP %d: %s", resp.StatusCode, message)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", errors.New("DeepSeek 未返回有效内容")
	}
	return result.Choices[0].Message.Content, nil
}
