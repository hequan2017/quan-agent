# Quan Agent

Quan Agent 是一个使用 Go 开发、面向 Windows 的本地智能运维助手。程序编译为单个 EXE，启动后自动打开管理页面，通过用户自己的 DeepSeek API Key 提供诊断建议，并通过 SSH 管理 Linux 机器。

## 当前能力

- 单文件 Windows EXE，前端资源内嵌，无需安装运行时。
- 本地 Web 管理页，默认仅监听 `127.0.0.1:18090`。
- 用户可配置 DeepSeek API Key、API 地址与模型。
- API Key 使用 Windows DPAPI 加密保存，只能由当前 Windows 用户解密。
- 自动采集主机名、系统架构、CPU、内存、磁盘和启动时间等只读信息。
- 使用 Windows DPAPI 加密管理 SSH 私钥，机器清单只关联密钥 ID。
- 通过 Windows OpenSSH 对 Linux 机器执行预定义只读 Skill。
- 内置系统概览、CPU/负载、内存、磁盘、异常服务、监听端口和 Docker 状态检查。
- AI 仅进行分析和建议，不执行任意命令，不自动修改系统。

## 使用

直接双击：

```text
quan-agent.exe
```

程序会打开 `http://127.0.0.1:18090`。首次使用时点击“设置 DeepSeek”，填写 API Key 后即可对话。关闭启动程序的命令窗口即可停止 Agent。

可选参数：

```powershell
./quan-agent.exe -listen "127.0.0.1:18090" -no-browser
```

配置保存在：

```text
%AppData%\QuanAgent\config.json
```

配置文件中的 API Key 是 DPAPI 密文，不会通过设置查询接口返回，也不会写入日志。

为避免无认证管理接口暴露到网络，`-listen` 仅接受 `127.0.0.1`、`::1` 或 `localhost`，不允许监听局域网或公网地址。

## 录入 Linux 机器

1. 在“SSH 密钥”页面点击“录入密钥”，填写名称并粘贴私钥内容。
2. 在“Linux 机器”页面点击“录入机器”。
3. 填写机器名称、IP 或域名、SSH 端口、Linux 用户、关联密钥和标签。
4. 保存后，在机器卡片上选择一个只读 Skill 运行。

运行环境需启用 Windows OpenSSH Client，可在 PowerShell 中检查：

```powershell
ssh -V
```

首次连接某台机器时，Quan Agent 会使用 OpenSSH 的 `accept-new` 策略将主机公钥记录到独立的：

```text
%AppData%\QuanAgent\known_hosts
```

如果主机公钥后续发生变化，连接会失败，避免静默接受中间人攻击。SSH 私钥仅在执行 Skill 期间写入随机临时目录，执行结束立即删除。

当前不支持带口令的 SSH 私钥。推荐为 Agent 创建权限受限的 Linux 用户和专用密钥，不要直接使用具有无限制 sudo 权限的 root 密钥。

## 运维 Skill

所有 Skill 均在 Go 代码中预定义，前端只能提交 Skill ID，无法提交命令文本。每个 Skill 都有独立超时，输出最大 512 KiB。目前仅包含只读巡检：

- 系统概览
- CPU 与负载
- 内存检查
- 磁盘与 inode
- systemd 异常服务与最近错误日志
- TCP/UDP 监听端口
- Docker 容器状态与空间占用

## 开发与构建

要求 Go 1.24 或更高版本：

```powershell
go test ./...
go vet ./...
New-Item -ItemType Directory -Force "dist"
go build -trimpath -ldflags="-s -w" -o "dist/quan-agent.exe" "./cmd/quan-agent"
```

## 安全边界

当前版本按 KISS/YAGNI 原则只实现“预定义只读 Skill + AI 建议”。它不会执行 AI 生成的命令。删除机器和密钥只删除本地记录，不会修改远程服务器。后续如增加变更类 Skill，应引入参数白名单、权限分层、持久化审计和危险操作二次确认，不能直接将模型输出交给 Shell。

## 开源方案调研

- [OliveTin](https://github.com/OliveTin/OliveTin)：采用其“仅暴露预定义动作”的安全理念；项目为 AGPL-3.0，本项目未复制其代码。
- [Sshwifty](https://github.com/nirui/sshwifty)：参考其本地监听、连接超时和输入不可信原则；它定位于交互式 Web SSH，未作为本项目依赖。
- [gopsutil](https://github.com/shirou/gopsutil)：BSD 许可的跨平台指标库，可用于未来增强本机指标；当前远程 Linux 采用零依赖 SSH Skill，暂未引入。
