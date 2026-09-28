[简体中文](README.md) | [English](README.en.md)

# Quan Agent

A local, Windows-focused intelligent ops assistant written in Go: a single EXE with an embedded web UI, where AI (DeepSeek) only analyzes and advises, and Linux inspections run through predefined read-only SSH skills.

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows-0078D6?logo=windows&logoColor=white)](#)

## Introduction

Quan Agent combines three things in one program: local host information collection, read-only inspection skills for Linux machines, and AI diagnostic advice driven by your own DeepSeek API key. It compiles into a single Windows EXE (frontend embedded) — double-click to run, no runtime installation needed.

Security is the first design principle: the AI only produces analysis text and **never executes AI-generated commands**; remote Linux machines can only run read-only skills predefined in Go code, and the frontend can only submit skill IDs; all sensitive data (DeepSeek API key, SSH private keys) is encrypted with Windows DPAPI, decryptable only by the current Windows user.

## ✨ Features

- **Single-file Windows EXE**: frontend embedded via go:embed, zero external dependencies
- **Local web UI**: listens on `127.0.0.1:18090` by default; `-listen` only accepts `127.0.0.1` / `::1` / `localhost`, so the admin interface can never be exposed to a LAN or the internet
- **Configurable DeepSeek**: bring your own API key, API base URL and model (default model `deepseek-chat`)
- **DPAPI-encrypted storage**: the API key and SSH private keys are encrypted per Windows user, never echoed back and never logged
- **Read-only local host info**: hostname, OS and architecture, CPU count, memory, disks, uptime
- **Linux machine inventory**: machines with linked key IDs and tags; SSH private keys are written to a random temporary directory only while a skill runs, then deleted immediately
- **7 built-in read-only inspection skills**: system overview, CPU & load, memory check, disks & inodes, failed systemd services and recent error logs, TCP/UDP listening ports, Docker container status and space usage
- **Host key verification**: first connections are recorded into a dedicated `known_hosts` via OpenSSH's `accept-new` policy; any later host key change fails the connection, blocking man-in-the-middle attacks

## 🛠 Tech Stack

| Layer | Technologies |
| --- | --- |
| Backend | Go 1.24+ · net/http · Windows DPAPI (crypt32) · Windows OpenSSH client (external `ssh` process) |
| Frontend | Plain HTML/JS embedded via go:embed |
| AI | DeepSeek Chat API (user-provided key) |

## 🚀 Quick Start

### Usage

Double-click `quan-agent.exe`; it opens `http://127.0.0.1:18090`. On first use, click "Set DeepSeek" and fill in your API key to start chatting. Closing the console window that launched the agent stops it.

Optional flags:

```powershell
./quan-agent.exe -listen "127.0.0.1:18090" -no-browser
```

Configuration lives in `%AppData%\QuanAgent\config.json` (the API key is stored as DPAPI ciphertext).

### Adding a Linux Machine

1. On the "SSH Keys" page → add a key (name + paste the private key)
2. On the "Linux Machines" page → add a machine (name, IP/hostname, SSH port, Linux user, linked key, tags)
3. After saving, pick a read-only skill on the machine card to run it

The environment needs the Windows OpenSSH client enabled (check with `ssh -V` in PowerShell). Passphrase-protected SSH private keys are not supported yet; create a restricted Linux user and a dedicated key for the agent, and never use a root key with unlimited sudo.

### Development & Build

```powershell
go test ./...
go vet ./...
New-Item -ItemType Directory -Force "dist"
go build -trimpath -ldflags="-s -w" -o "dist/quan-agent.exe" "./cmd/quan-agent"
```

## 📁 Directory Structure

```
├── cmd/quan-agent/      Entry point: listen validation, browser launch
├── internal/config/     DPAPI encryption and config storage
├── internal/deepseek/   DeepSeek API client
├── internal/inventory/  Machine/key inventory and known_hosts
├── internal/ops/        Read-only local host info collection
├── internal/remote/     SSH skill definitions and runner (timeouts / 512 KiB output cap)
├── internal/server/     Local web server and APIs
└── web/                 Embedded frontend (index.html / app.js)
```

## 🔗 Related Projects & Research

- [OliveTin](https://github.com/OliveTin/OliveTin): adopted its "expose only predefined actions" security philosophy; the project is AGPL-3.0 and no code was copied from it.
- [Sshwifty](https://github.com/nirui/sshwifty): referenced its local listening, connection timeouts and untrusted-input principles; it is an interactive web SSH client and is not a dependency of this project.
- [gopsutil](https://github.com/shirou/gopsutil): a BSD-licensed cross-platform metrics library that could enhance local metrics in the future; remote Linux currently uses zero-dependency SSH skills, so it is not yet imported.

## ⚠️ Security Boundary

Following KISS/YAGNI, the current version only implements "predefined read-only skills + AI advice" and never executes AI-generated commands. Deleting machines or keys only removes local records and never touches remote servers. If mutation skills are added later, they must come with parameter allowlists, permission tiers, persistent auditing and double confirmation for dangerous operations — never pipe model output straight into a shell.

## 📄 License

[Apache License 2.0](LICENSE)
