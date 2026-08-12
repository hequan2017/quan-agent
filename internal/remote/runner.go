package remote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"quan-agent/internal/inventory"
)

const maxOutputBytes = 512 * 1024

type Result struct {
	MachineID string `json:"machine_id"`
	SkillID   string `json:"skill_id"`
	SkillName string `json:"skill_name"`
	StartedAt string `json:"started_at"`
	Duration  int64  `json:"duration_ms"`
	Success   bool   `json:"success"`
	Output    string `json:"output"`
}

type Runner struct {
	inventory *inventory.Store
}

func NewRunner(store *inventory.Store) *Runner {
	return &Runner{inventory: store}
}

func (r *Runner) Run(ctx context.Context, machineID, skillID string) (Result, error) {
	machine, ok := r.inventory.GetMachine(machineID)
	if !ok {
		return Result{}, errors.New("机器不存在")
	}
	key, ok := r.inventory.GetKey(machine.KeyID)
	if !ok {
		return Result{}, errors.New("机器关联的 SSH 密钥不存在")
	}
	skill, ok := getSkill(skillID)
	if !ok {
		return Result{}, errors.New("运维 Skill 不存在")
	}
	if _, err := exec.LookPath("ssh"); err != nil {
		return Result{}, errors.New("未找到 Windows OpenSSH 客户端，请先启用系统的 OpenSSH Client 可选功能")
	}

	tempDir, err := os.MkdirTemp("", "quan-agent-ssh-")
	if err != nil {
		return Result{}, fmt.Errorf("创建临时密钥目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	keyPath := filepath.Join(tempDir, "identity")
	if err := os.WriteFile(keyPath, []byte(key.PrivateKey), 0o600); err != nil {
		return Result{}, fmt.Errorf("准备临时 SSH 密钥: %w", err)
	}
	if err := restrictPrivateKey(keyPath); err != nil {
		return Result{}, fmt.Errorf("限制临时 SSH 密钥权限: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(r.inventory.KnownHostsPath()), 0o700); err != nil {
		return Result{}, fmt.Errorf("准备 known_hosts: %w", err)
	}

	timeout := time.Duration(skill.TimeoutSec) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	target := machine.User + "@" + machine.Host
	args := []string{
		"-T", "-p", fmt.Sprintf("%d", machine.Port),
		"-i", keyPath,
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=2",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=" + r.inventory.KnownHostsPath(),
		target, skill.Command,
	}
	command := exec.CommandContext(runCtx, "ssh", args...)
	output, runErr := command.CombinedOutput()
	duration := time.Since(started)
	if len(output) > maxOutputBytes {
		output = append(output[:maxOutputBytes], []byte("\n... 输出已截断 ...")...)
	}
	result := Result{
		MachineID: machine.ID, SkillID: skill.ID, SkillName: skill.Name,
		StartedAt: started.Format(time.RFC3339), Duration: duration.Milliseconds(),
		Success: runErr == nil, Output: strings.TrimSpace(string(output)),
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("Skill 执行超过 %d 秒，已终止", skill.TimeoutSec)
	}
	if runErr != nil {
		if result.Output == "" {
			result.Output = runErr.Error()
		}
		return result, fmt.Errorf("SSH Skill 执行失败: %w", runErr)
	}
	return result, nil
}

func SSHAvailable() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	_, err := exec.LookPath("ssh")
	return err == nil
}
