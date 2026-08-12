package remote

import "sort"

type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"-"`
	TimeoutSec  int    `json:"timeout_sec"`
}

var skills = map[string]Skill{
	"system_overview": {
		ID: "system_overview", Name: "系统概览", Description: "系统版本、内核、主机名、启动时间和当前用户",
		Command:    "printf '%s\\n' '=== OS ==='; (cat /etc/os-release 2>/dev/null || true); printf '%s\\n' '=== KERNEL ==='; uname -a; printf '%s\\n' '=== UPTIME ==='; uptime; printf '%s\\n' '=== USER ==='; id",
		TimeoutSec: 20,
	},
	"cpu_load": {
		ID: "cpu_load", Name: "CPU 与负载", Description: "CPU 数量、负载与占用最高的进程",
		Command:    "printf '%s\\n' '=== CPU COUNT ==='; getconf _NPROCESSORS_ONLN 2>/dev/null || nproc; printf '%s\\n' '=== LOAD ==='; cat /proc/loadavg; printf '%s\\n' '=== TOP CPU ==='; ps -eo pid,user,comm,%cpu,%mem --sort=-%cpu | head -n 12",
		TimeoutSec: 20,
	},
	"memory": {
		ID: "memory", Name: "内存检查", Description: "内存、Swap 与高内存进程",
		Command:    "printf '%s\\n' '=== MEMORY ==='; free -h; printf '%s\\n' '=== TOP MEMORY ==='; ps -eo pid,user,comm,%mem,%cpu --sort=-%mem | head -n 12",
		TimeoutSec: 20,
	},
	"disk": {
		ID: "disk", Name: "磁盘检查", Description: "文件系统容量、inode 使用率与块设备",
		Command:    "printf '%s\\n' '=== FILESYSTEM ==='; df -hPT -x tmpfs -x devtmpfs; printf '%s\\n' '=== INODES ==='; df -hiP -x tmpfs -x devtmpfs; printf '%s\\n' '=== BLOCK DEVICES ==='; lsblk -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINTS 2>/dev/null || true",
		TimeoutSec: 25,
	},
	"failed_services": {
		ID: "failed_services", Name: "异常服务", Description: "systemd 失败单元与最近高优先级日志",
		Command:    "printf '%s\\n' '=== FAILED UNITS ==='; systemctl --failed --no-pager 2>&1 || true; printf '%s\\n' '=== RECENT ERRORS ==='; journalctl -p 0..3 -n 80 --no-pager 2>&1 || true",
		TimeoutSec: 30,
	},
	"listening_ports": {
		ID: "listening_ports", Name: "监听端口", Description: "TCP/UDP 监听端口及关联进程",
		Command:    "ss -lntup 2>&1 || netstat -lntup 2>&1 || true",
		TimeoutSec: 20,
	},
	"docker_status": {
		ID: "docker_status", Name: "Docker 状态", Description: "Docker 版本、容器状态与磁盘占用",
		Command:    "printf '%s\\n' '=== VERSION ==='; docker version --format '{{.Server.Version}}' 2>&1 || true; printf '%s\\n' '=== CONTAINERS ==='; docker ps -a --format 'table {{.Names}}\\t{{.Image}}\\t{{.Status}}\\t{{.Ports}}' 2>&1 || true; printf '%s\\n' '=== DISK ==='; docker system df 2>&1 || true",
		TimeoutSec: 35,
	},
}

func ListSkills() []Skill {
	result := make([]Skill, 0, len(skills))
	for _, skill := range skills {
		skill.Command = ""
		result = append(result, skill)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func getSkill(id string) (Skill, bool) {
	skill, ok := skills[id]
	return skill, ok
}
