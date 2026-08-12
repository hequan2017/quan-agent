package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Toolbox struct{}

type Snapshot struct {
	CollectedAt string `json:"collected_at"`
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	CPUCount    int    `json:"cpu_count"`
	GoVersion   string `json:"agent_go_version"`
	Uptime      string `json:"uptime,omitempty"`
	Memory      string `json:"memory,omitempty"`
	Disks       string `json:"disks,omitempty"`
}

func NewToolbox() *Toolbox {
	return &Toolbox{}
}

func (t *Toolbox) Snapshot(ctx context.Context) Snapshot {
	hostname, _ := os.Hostname()
	snapshot := Snapshot{
		CollectedAt: time.Now().Format(time.RFC3339),
		Hostname:    hostname,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CPUCount:    runtime.NumCPU(),
		GoVersion:   runtime.Version(),
	}
	if runtime.GOOS == "windows" {
		snapshot.Uptime = runPowerShell(ctx, "(Get-CimInstance Win32_OperatingSystem | Select-Object -ExpandProperty LastBootUpTime).ToString('o')")
		snapshot.Memory = runPowerShell(ctx, "Get-CimInstance Win32_OperatingSystem | Select-Object @{N='TotalGB';E={[math]::Round($_.TotalVisibleMemorySize/1MB,2)}},@{N='FreeGB';E={[math]::Round($_.FreePhysicalMemory/1MB,2)}} | ConvertTo-Json -Compress")
		snapshot.Disks = runPowerShell(ctx, "Get-CimInstance Win32_LogicalDisk -Filter \"DriveType=3\" | Select-Object DeviceID,@{N='SizeGB';E={[math]::Round($_.Size/1GB,2)}},@{N='FreeGB';E={[math]::Round($_.FreeSpace/1GB,2)}} | ConvertTo-Json -Compress")
	}
	return snapshot
}

func (t *Toolbox) SnapshotJSON(ctx context.Context) string {
	snapshot := t.Snapshot(ctx)
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func runPowerShell(parent context.Context, script string) string {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "采集失败: " + err.Error()
		}
		return "采集失败: " + text
	}
	return text
}
