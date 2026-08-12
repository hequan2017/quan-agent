package config

import "testing"

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "DeepSeek HTTPS", value: "https://api.deepseek.com/chat/completions"},
		{name: "本机 HTTP", value: "http://127.0.0.1:11434/v1/chat/completions"},
		{name: "拒绝远程 HTTP", value: "http://example.com/v1/chat/completions", wantErr: true},
		{name: "拒绝无主机地址", value: "not-a-url", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateBaseURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateBaseURL() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestMaskKey(t *testing.T) {
	if got := maskKey("sk-1234567890"); got != "sk-1••••7890" {
		t.Fatalf("maskKey() = %q", got)
	}
	if got := maskKey(""); got != "" {
		t.Fatalf("empty maskKey() = %q", got)
	}
}
