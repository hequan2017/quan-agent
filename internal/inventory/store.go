package inventory

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"quan-agent/internal/config"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Key struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PrivateKey string `json:"-"`
	CreatedAt  string `json:"created_at"`
}

type Machine struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	User      string   `json:"user"`
	KeyID     string   `json:"key_id"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
}

type diskKey struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	EncryptedKey string `json:"encrypted_private_key"`
	CreatedAt    string `json:"created_at"`
}

type diskData struct {
	Keys     []diskKey `json:"keys"`
	Machines []Machine `json:"machines"`
}

type Store struct {
	mu         sync.RWMutex
	keys       map[string]Key
	machines   map[string]Machine
	path       string
	knownHosts string
}

func NewStore() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户配置目录: %w", err)
	}
	dir := filepath.Join(configDir, "QuanAgent")
	store := &Store{
		keys:       make(map[string]Key),
		machines:   make(map[string]Machine),
		path:       filepath.Join(dir, "inventory.json"),
		knownHosts: filepath.Join(dir, "known_hosts"),
	}
	if err := store.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return store, nil
}

func (s *Store) KnownHostsPath() string { return s.knownHosts }

func (s *Store) ListKeys() []Key {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Key, 0, len(s.keys))
	for _, key := range s.keys {
		key.PrivateKey = ""
		result = append(result, key)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (s *Store) AddKey(name, privateKey string) (Key, error) {
	name = strings.TrimSpace(name)
	privateKey = strings.TrimSpace(privateKey)
	if name == "" || len(name) > 100 {
		return Key{}, errors.New("密钥名称不能为空且不能超过 100 个字符")
	}
	if !looksLikePrivateKey(privateKey) {
		return Key{}, errors.New("仅支持 OpenSSH、RSA、EC 或 PKCS#8 PEM 私钥")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range s.keys {
		if strings.EqualFold(key.Name, name) {
			return Key{}, errors.New("密钥名称已存在")
		}
	}
	key := Key{ID: newID(), Name: name, PrivateKey: privateKey + "\n", CreatedAt: time.Now().Format(time.RFC3339)}
	s.keys[key.ID] = key
	if err := s.saveLocked(); err != nil {
		delete(s.keys, key.ID)
		return Key{}, err
	}
	key.PrivateKey = ""
	return key, nil
}

func (s *Store) DeleteKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[id]; !ok {
		return os.ErrNotExist
	}
	for _, machine := range s.machines {
		if machine.KeyID == id {
			return errors.New("该密钥仍被机器使用，请先修改或删除关联机器")
		}
	}
	key := s.keys[id]
	delete(s.keys, id)
	if err := s.saveLocked(); err != nil {
		s.keys[id] = key
		return err
	}
	return nil
}

func (s *Store) GetKey(id string) (Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, ok := s.keys[id]
	return key, ok
}

func (s *Store) ListMachines() []Machine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Machine, 0, len(s.machines))
	for _, machine := range s.machines {
		result = append(result, machine)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (s *Store) GetMachine(id string) (Machine, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	machine, ok := s.machines[id]
	return machine, ok
}

func (s *Store) AddMachine(machine Machine) (Machine, error) {
	machine.Name = strings.TrimSpace(machine.Name)
	machine.Host = strings.TrimSpace(machine.Host)
	machine.User = strings.TrimSpace(machine.User)
	if machine.Port == 0 {
		machine.Port = 22
	}
	if machine.Name == "" || len(machine.Name) > 100 {
		return Machine{}, errors.New("机器名称不能为空且不能超过 100 个字符")
	}
	if !validHost(machine.Host) {
		return Machine{}, errors.New("主机地址格式无效")
	}
	if machine.Port < 1 || machine.Port > 65535 {
		return Machine{}, errors.New("SSH 端口必须在 1-65535 之间")
	}
	if !safeName.MatchString(machine.User) || len(machine.User) > 64 {
		return Machine{}, errors.New("SSH 用户名格式无效")
	}
	machine.Tags = normalizeTags(machine.Tags)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[machine.KeyID]; !ok {
		return Machine{}, errors.New("关联密钥不存在")
	}
	for _, current := range s.machines {
		if strings.EqualFold(current.Name, machine.Name) {
			return Machine{}, errors.New("机器名称已存在")
		}
	}
	machine.ID = newID()
	machine.CreatedAt = time.Now().Format(time.RFC3339)
	s.machines[machine.ID] = machine
	if err := s.saveLocked(); err != nil {
		delete(s.machines, machine.ID)
		return Machine{}, err
	}
	return machine, nil
}

func (s *Store) DeleteMachine(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	machine, ok := s.machines[id]
	if !ok {
		return os.ErrNotExist
	}
	delete(s.machines, id)
	if err := s.saveLocked(); err != nil {
		s.machines[id] = machine
		return err
	}
	return nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var disk diskData
	if err := json.Unmarshal(data, &disk); err != nil {
		return fmt.Errorf("解析机器清单: %w", err)
	}
	for _, diskKey := range disk.Keys {
		ciphertext, err := base64.StdEncoding.DecodeString(diskKey.EncryptedKey)
		if err != nil {
			return fmt.Errorf("解析密钥 %q: %w", diskKey.Name, err)
		}
		plaintext, err := config.Unprotect(ciphertext)
		if err != nil {
			return fmt.Errorf("解密密钥 %q: %w", diskKey.Name, err)
		}
		s.keys[diskKey.ID] = Key{ID: diskKey.ID, Name: diskKey.Name, PrivateKey: string(plaintext), CreatedAt: diskKey.CreatedAt}
	}
	for _, machine := range disk.Machines {
		s.machines[machine.ID] = machine
	}
	return nil
}

func (s *Store) saveLocked() error {
	disk := diskData{Machines: make([]Machine, 0, len(s.machines))}
	for _, key := range s.keys {
		ciphertext, err := config.Protect([]byte(key.PrivateKey))
		if err != nil {
			return fmt.Errorf("加密 SSH 私钥: %w", err)
		}
		disk.Keys = append(disk.Keys, diskKey{ID: key.ID, Name: key.Name, EncryptedKey: base64.StdEncoding.EncodeToString(ciphertext), CreatedAt: key.CreatedAt})
	}
	for _, machine := range s.machines {
		disk.Machines = append(disk.Machines, machine)
	}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return err
	}
	return nil
}

func looksLikePrivateKey(value string) bool {
	headers := []string{
		"-----BEGIN OPENSSH PRIVATE KEY-----",
		"-----BEGIN RSA PRIVATE KEY-----",
		"-----BEGIN EC PRIVATE KEY-----",
		"-----BEGIN PRIVATE KEY-----",
	}
	for _, header := range headers {
		if strings.HasPrefix(value, header) && strings.Contains(value, strings.Replace(header, "BEGIN", "END", 1)) {
			return true
		}
	}
	return false
}

func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) == 0 || len(host) > 253 || strings.HasPrefix(host, "-") || strings.HasSuffix(host, "-") {
		return false
	}
	for _, part := range strings.Split(host, ".") {
		if part == "" || len(part) > 63 || !safeName.MatchString(part) || strings.Contains(part, "_") {
			return false
		}
	}
	return true
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len(tag) > 30 || seen[tag] || len(result) >= 10 {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}
	return result
}

func newID() string {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		panic("无法生成安全随机 ID: " + err.Error())
	}
	return hex.EncodeToString(data)
}
