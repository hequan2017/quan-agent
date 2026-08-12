package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const defaultBaseURL = "https://api.deepseek.com/chat/completions"

type Settings struct {
	APIKey  string `json:"-"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

type PublicSettings struct {
	BaseURL       string `json:"base_url"`
	Model         string `json:"model"`
	KeyConfigured bool   `json:"key_configured"`
	MaskedAPIKey  string `json:"masked_api_key,omitempty"`
}

type diskSettings struct {
	BaseURL         string `json:"base_url"`
	Model           string `json:"model"`
	EncryptedAPIKey string `json:"encrypted_api_key,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	settings Settings
	path     string
}

func NewStore() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户配置目录: %w", err)
	}
	store := &Store{
		settings: Settings{BaseURL: defaultBaseURL, Model: "deepseek-chat"},
		path:     filepath.Join(configDir, "QuanAgent", "config.json"),
	}
	if err := store.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return store, nil
}

func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) Public() PublicSettings {
	settings := s.Get()
	return PublicSettings{
		BaseURL:       settings.BaseURL,
		Model:         settings.Model,
		KeyConfigured: settings.APIKey != "",
		MaskedAPIKey:  maskKey(settings.APIKey),
	}
}

func (s *Store) Update(apiKey, baseURL, model string) error {
	baseURL = strings.TrimSpace(baseURL)
	model = strings.TrimSpace(model)
	if err := validateBaseURL(baseURL); err != nil {
		return err
	}
	if model == "" || len(model) > 100 {
		return errors.New("模型名称不能为空且不能超过 100 个字符")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if apiKey != "" {
		s.settings.APIKey = strings.TrimSpace(apiKey)
	}
	s.settings.BaseURL = baseURL
	s.settings.Model = model
	return s.saveLocked()
}

func (s *Store) ClearAPIKey() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings.APIKey = ""
	return s.saveLocked()
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var disk diskSettings
	if err := json.Unmarshal(data, &disk); err != nil {
		return fmt.Errorf("解析配置文件: %w", err)
	}
	if disk.BaseURL != "" {
		s.settings.BaseURL = disk.BaseURL
	}
	if disk.Model != "" {
		s.settings.Model = disk.Model
	}
	if disk.EncryptedAPIKey != "" {
		ciphertext, err := base64.StdEncoding.DecodeString(disk.EncryptedAPIKey)
		if err != nil {
			return fmt.Errorf("解析加密密钥: %w", err)
		}
		plaintext, err := unprotect(ciphertext)
		if err != nil {
			return fmt.Errorf("解密 API Key: %w", err)
		}
		s.settings.APIKey = string(plaintext)
	}
	return nil
}

func (s *Store) saveLocked() error {
	disk := diskSettings{BaseURL: s.settings.BaseURL, Model: s.settings.Model}
	if s.settings.APIKey != "" {
		ciphertext, err := protect([]byte(s.settings.APIKey))
		if err != nil {
			return fmt.Errorf("加密 API Key: %w", err)
		}
		disk.EncryptedAPIKey = base64.StdEncoding.EncodeToString(ciphertext)
	}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("保存配置: %w", err)
	}
	return nil
}

func validateBaseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return errors.New("API 地址格式无效")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && (host == "localhost" || net.ParseIP(host).IsLoopback()) {
		return nil
	}
	return errors.New("API 地址必须使用 HTTPS；本机地址可使用 HTTP")
}

func maskKey(key string) string {
	if len(key) < 9 {
		if key == "" {
			return ""
		}
		return "已配置"
	}
	return key[:4] + "••••" + key[len(key)-4:]
}
