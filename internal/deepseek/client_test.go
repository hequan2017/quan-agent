package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"quan-agent/internal/config"
	"quan-agent/internal/ops"
)

type staticSettings struct {
	settings config.Settings
}

func (s staticSettings) Get() config.Settings {
	return s.settings
}

func TestChatBuildsDeepSeekRequest(t *testing.T) {
	var received apiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"状态正常"}}]}`))
	}))
	defer server.Close()

	client := NewClient(staticSettings{config.Settings{
		APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-chat",
	}}, ops.NewToolbox())
	reply, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "检查主机"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if reply != "状态正常" {
		t.Fatalf("Chat() reply = %q", reply)
	}
	if received.Model != "deepseek-chat" {
		t.Fatalf("model = %q", received.Model)
	}
	if len(received.Messages) < 3 || received.Messages[len(received.Messages)-1].Content != "检查主机" {
		t.Fatalf("messages were not assembled correctly: %#v", received.Messages)
	}
}

func TestChatRequiresAPIKey(t *testing.T) {
	client := NewClient(staticSettings{config.Settings{}}, ops.NewToolbox())
	_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "检查主机"}})
	if err == nil {
		t.Fatal("Chat() should reject missing API key")
	}
}
