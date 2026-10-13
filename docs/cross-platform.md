# 跨平台适配记录

日期：2026-10-10。Linux 优先，保留 Windows 本地开发能力。

| 原有问题 | 修改 |
| --- | --- |
| 启停和认证验证只提供 PowerShell 脚本 | 改为 Node 标准库入口 `scripts/local.mjs`、`scripts/verify-auth.mjs`，移除旧 `.ps1` |
| Windows 专属端口查询、进程管理及 `.exe` 名称 | 使用 TCP 端口检测、当前平台二进制名称、每次启动独立的本地控制器；停止不依赖 PID |
| 旧 PowerShell JSON 日期隐式转换导致身份误判 | 新入口不比较字符串化进程时间，并兼容旧凭据文件的 BOM |
| Docker 后端依赖宿主机预编译的 Linux 文件 | 容器内多阶段构建，不指定固定 CPU 架构，CGO 关闭 |
| Dockerfile 默认使用 Docker Desktop 专属代理 | 去除固定代理，支持标准构建参数；使用 lockfile 和 `npm ci` |
| OAuth 写死某台机器的本机代理端口 | 默认关闭代理，环境变量按需设置 |
| 最小 Linux 系统可能缺少 IANA 时区数据库 | 后端嵌入 `time/tzdata`，容器也安装 tzdata |
| 用户端相对头像 URL 没有转发 | 补齐 `/uploads` 代理；Compose 头像目录使用持久卷 |
| 运行凭据在 Linux 的访问权限没有明确限制 | 运行目录 700，密码和密钥文件 600 |
| 文档运行命令绑定 PowerShell | README 提供统一命令、Linux Compose 启动及首次管理员登录说明 |

验证结果：

- Windows：新入口完整启动（含 Compose 就绪等待）、停止与重启通过；本地认证验证 37 项通过；Go 测试和 vet 通过。
- Linux amd64 容器：脚本测试 3 项通过，包含带空格的路径、密码不被重置、文件权限、端口保护、控制器鉴权、启停和启动失败清理。
- 三个 Linux Docker 镜像从源码构建通过；两个前端在 Linux 下的 vue-tsc 与 Vite 构建通过。
- 独立 Linux MySQL/Redis 与三个应用容器首次启动通过；从 Linux Node 容器运行认证验证，37 项通过。测试容器与独立测试卷已清理，本地数据库保留。

边界：未在 macOS、Linux ARM64 或实体 Linux 主机实测。当前 Compose 前端仍为 Vite 开发服务器；正式部署的静态站点、TLS 与 systemd 运维配置不属于本次变更。后端文件路径采用 Go filepath，启动器明确设置工作目录；Linux 容器构建也验证了现有源文件导入路径的大小写。
