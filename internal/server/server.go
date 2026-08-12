package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"quan-agent/internal/config"
	"quan-agent/internal/deepseek"
	"quan-agent/internal/inventory"
	"quan-agent/internal/ops"
	"quan-agent/internal/remote"
	"quan-agent/web"
)

type Server struct {
	settings  *config.Store
	inventory *inventory.Store
	chat      *deepseek.Client
	toolbox   *ops.Toolbox
	runner    *remote.Runner
	logger    *slog.Logger
}

func New(settings *config.Store, inventoryStore *inventory.Store, chat *deepseek.Client, toolbox *ops.Toolbox, runner *remote.Runner, logger *slog.Logger) http.Handler {
	server := &Server{settings: settings, inventory: inventoryStore, chat: chat, toolbox: toolbox, runner: runner, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", server.health)
	mux.HandleFunc("GET /api/settings", server.getSettings)
	mux.HandleFunc("PUT /api/settings", server.updateSettings)
	mux.HandleFunc("DELETE /api/settings/key", server.clearAPIKey)
	mux.HandleFunc("GET /api/snapshot", server.snapshot)
	mux.HandleFunc("POST /api/chat", server.handleChat)
	mux.HandleFunc("GET /api/keys", server.listKeys)
	mux.HandleFunc("POST /api/keys", server.addKey)
	mux.HandleFunc("DELETE /api/keys/{id}", server.deleteKey)
	mux.HandleFunc("GET /api/machines", server.listMachines)
	mux.HandleFunc("POST /api/machines", server.addMachine)
	mux.HandleFunc("DELETE /api/machines/{id}", server.deleteMachine)
	mux.HandleFunc("GET /api/skills", server.listSkills)
	mux.HandleFunc("POST /api/machines/{id}/skills/{skill}", server.runSkill)
	assets, _ := fs.Sub(web.Assets, ".")
	mux.Handle("/", http.FileServer(http.FS(assets)))
	return server.securityHeaders(mux)
}

func (s *Server) listKeys(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.inventory.ListKeys())
}

func (s *Server) addKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name       string `json:"name"`
		PrivateKey string `json:"private_key"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	key, err := s.inventory.AddKey(input.Name, input.PrivateKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.logger.Info("SSH 密钥已录入", "key_id", key.ID, "name", key.Name)
	writeJSON(w, http.StatusCreated, key)
}

func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.inventory.DeleteKey(id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, fs.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	s.logger.Info("SSH 密钥已删除", "key_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listMachines(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.inventory.ListMachines())
}

func (s *Server) addMachine(w http.ResponseWriter, r *http.Request) {
	var machine inventory.Machine
	if err := decodeJSON(r, &machine); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.inventory.AddMachine(machine)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.logger.Info("Linux 机器已录入", "machine_id", created.ID, "name", created.Name, "host", created.Host)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) deleteMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.inventory.DeleteMachine(id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, fs.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	s.logger.Info("Linux 机器已删除", "machine_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSkills(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ssh_available": remote.SSHAvailable(),
		"skills":        remote.ListSkills(),
	})
}

func (s *Server) runSkill(w http.ResponseWriter, r *http.Request) {
	machineID := r.PathValue("id")
	skillID := r.PathValue("skill")
	s.logger.Info("开始执行只读 Skill", "machine_id", machineID, "skill_id", skillID)
	result, err := s.runner.Run(r.Context(), machineID, skillID)
	if err != nil {
		s.logger.Warn("只读 Skill 执行失败", "machine_id", machineID, "skill_id", skillID, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "result": result})
		return
	}
	s.logger.Info("只读 Skill 执行完成", "machine_id", machineID, "skill_id", skillID, "duration_ms", result.Duration)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.settings.Public())
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.settings.Update(input.APIKey, input.BaseURL, input.Model); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, s.settings.Public())
}

func (s *Server) clearAPIKey(w http.ResponseWriter, _ *http.Request) {
	if err := s.settings.ClearAPIKey(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.toolbox.Snapshot(r.Context()))
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Messages []deepseek.Message `json:"messages"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	reply, err := s.chat.Chat(r.Context(), input.Messages)
	if err != nil {
		s.logger.Warn("AI 对话失败", "error", err)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("请求 JSON 格式无效")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("请求只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "未知错误"
	}
	writeJSON(w, status, map[string]string{"error": message})
}
