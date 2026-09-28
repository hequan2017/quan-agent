[简体中文](README.md) | [English](README.en.md)

# Quan Agent

使用 Go 开发、面向 Windows 的本地智能运维助手：单 EXE 内嵌 Web 管理页，AI（DeepSeek）只做分析与建议，Linux 巡检通过预定义只读 SSH Skill 完成。

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows-0078D6?logo=windows&logoColor=white)](#)

## 简介

Quan Agent 把三件事组合在一个程序里：本地主机信息采集、面向 Linux 机器的只读巡检 Skill、以及基于你自己 DeepSeek API Key 的 AI 诊断建议。程序编译为单个 Windows EXE（前端资源内嵌），双击即用，无需安装运行时。

安全是第一设计原则：AI 仅输出分析文本，**不执行任何 AI 生成的命令**；远程 Linux 只能运行 Go 代码中预定义的只读 Skill，前端只能提交 Skill ID；所有敏感数据（DeepSeek API Key、SSH 私钥）使用 Windows DPAPI 加密，仅当前 Windows 用户可解密。

## ✨ 功能特性

- **单文件 Windows EXE**：前端资源内嵌（go:embed），无外部依赖
- **本地 Web 管理页**：默认仅监听 `127.0.0.1:18090`；`-listen` 只接受 `127.0.0.1` / `::1` / `localhost`，杜绝管理接口暴露到局域网/公网
- **DeepSeek 可配置**：用户自备 API Key、API 地址与模型（默认模型 `deepseek-chat`）
- **DPAPI 加密存储**：API Key 与 SSH 私钥按当前 Windows 用户加密，不回显、不写日志
- **本机只读信息采集**：主机名、操作系统与架构、CPU 核数、内存、磁盘、运行时长（uptime）
- **Linux 机器清单**：机器 + 关联密钥 ID + 标签管理；SSH 私钥仅在执行 Skill 期间写入随机临时目录，执行结束立即删除
- **内置 7 个只读巡检 Skill**：系统概览、CPU 与负载、内存检查、磁盘与 inode、systemd 异常服务与最近错误日志、TCP/UDP 监听端口、Docker 容器状态与空间占用
- **主机公钥校验**：首次连接用 OpenSSH `accept-new` 策略记录到独立 `known_hosts`，公钥变化即拒绝连接，防中间人

## 🛠 技术栈

| 端 | 技术 |
| --- | --- |
| 后端 | Go 1.24+ · net/http · Windows DPAPI（crypt32）· Windows OpenSSH 客户端（外部 `ssh` 进程） |
| 前端 | 原生 HTML/JS，go:embed 内嵌 |
| AI | DeepSeek Chat API（用户自备 Key） |

## 🚀 快速开始

### 使用

双击 `quan-agent.exe`，程序会打开 `http://127.0.0.1:18090`。首次使用点击「设置 DeepSeek」填写 API Key 后即可对话。关闭启动它的命令窗口即停止 Agent。

可选参数：

```powershell
./quan-agent.exe -listen "127.0.0.1:18090" -no-browser
```

配置保存在 `%AppData%\QuanAgent\config.json`（API Key 为 DPAPI 密文）。

### 录入 Linux 机器

1. 「SSH 密钥」页面 → 录入密钥（名称 + 粘贴私钥内容）
2. 「Linux 机器」页面 → 录入机器（名称、IP/域名、SSH 端口、Linux 用户、关联密钥、标签）
3. 保存后在机器卡片上选择一个只读 Skill 运行

运行环境需启用 Windows OpenSSH Client（PowerShell 中 `ssh -V` 检查）。当前不支持带口令的 SSH 私钥；推荐为 Agent 创建权限受限的 Linux 用户和专用密钥，不要使用具有无限制 sudo 权限的 root 密钥。

### 开发与构建

```powershell
go test ./...
go vet ./...
New-Item -ItemType Directory -Force "dist"
go build -trimpath -ldflags="-s -w" -o "dist/quan-agent.exe" "./cmd/quan-agent"
```

## 📁 目录结构

```
├── cmd/quan-agent/      入口：监听校验、浏览器拉起
├── internal/config/     DPAPI 加密与配置存储
├── internal/deepseek/   DeepSeek API 客户端
├── internal/inventory/  机器/密钥清单与 known_hosts
├── internal/ops/        本机只读信息采集
├── internal/remote/     SSH Skill 定义与执行器（超时/512KiB 输出上限）
├── internal/server/     本地 Web 服务与 API
└── web/                 内嵌前端（index.html / app.js）
```

## 🔗 相关项目与开源方案调研

- [OliveTin](https://github.com/OliveTin/OliveTin)：借鉴其「仅暴露预定义动作」的安全理念；项目为 AGPL-3.0，本项目未复制其代码。
- [Sshwifty](https://github.com/nirui/sshwifty)：参考其本地监听、连接超时和输入不可信原则；定位为交互式 Web SSH，未作为本项目依赖。
- [gopsutil](https://github.com/shirou/gopsutil)：BSD 许可的跨平台指标库，可用于未来增强本机指标；当前远程 Linux 采用零依赖 SSH Skill，暂未引入。

## ⚠️ 安全边界

当前版本按 KISS/YAGNI 原则只实现「预定义只读 Skill + AI 建议」，不会执行 AI 生成的命令。删除机器和密钥只删除本地记录，不修改远程服务器。后续如增加变更类 Skill，应引入参数白名单、权限分层、持久化审计和危险操作二次确认，不能直接将模型输出交给 Shell。

## 📄 License

[Apache License 2.0](LICENSE)
