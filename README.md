# SentinelGo

Go 后端与 Vue 管理端、用户端组成的三端全栈应用。优先支持 Linux，同时支持 Windows 和 macOS 本地开发。

SentinelGo is a full-stack application with a Go backend plus a Vue admin frontend and a Vue user frontend. Linux is the primary target, while Windows and macOS are supported for local development.

- [中文文档](#中文文档)
- [English](#english)

---

## 中文文档

### GitHub CI/CD

GitHub Actions 检查后端、两个前端和 Docker 构建；推送 `v1.0.0` 等版本标签时，检查通过后将三个生产镜像发布到 GHCR。配置和部署步骤见 [GitHub CI/CD](docs/github-actions.md)。

### 目录结构

```text
SentinelGo/
├── backend/            # Go 后端（cmd / internal / pkg / config）
├── frontend/          # Vue 管理端
├── user-frontend/     # Vue 用户端
├── scripts/           # 本地启动器 local.mjs、验证脚本、lint
├── docs/              # 设计 / 安全文档
├── docker-compose.yml # Linux 全栈编排
├── .env.example       # 部署环境变量模板
└── README.md
```

### 技术栈

- 后端：Go 1.25（在 Linux 容器内多阶段编译为静态二进制）
- 前端：Vue 3 + Vite（管理端与用户端各一个工程）
- 存储：MySQL 8.4、Redis 7.4
- 编排：Docker Engine + Compose v2

### 环境要求

- Go 1.25.5 或更新版本
- Node.js 24 LTS
- Docker Engine 与 Compose v2

> Linux 不需要 Docker Desktop、PowerShell 或 sudo 来运行项目脚本；当前用户需有 Docker 访问权限。

### 本地开发

在项目根目录依次运行（所有平台命令相同）：

```text
npm --prefix frontend ci
npm --prefix user-frontend ci
node scripts/local.mjs start
```

启动命令会检查端口、启动 MySQL/Redis、等待数据库健康、编译当前平台的 Go 二进制，再启动三个服务并等待就绪。保持终端运行；按 Ctrl+C 停止三个应用服务，或在另一个终端运行：

```text
node scripts/local.mjs stop
```

停止命令使用本次启动的本地控制器，不依赖 Windows 进程命令，也不会按过期 PID 误杀其他程序。数据库容器和数据保留。日志位于 `.runtime/*.log`。已有数据库、Redis 服务时可使用 `start --skip-dependencies`，连接参数通过 `KS_DATABASE_MASTER_DSN`、`KS_REDIS_ADDR` 等环境变量设置。

- 管理端：http://127.0.0.1:3000
- 用户端：http://127.0.0.1:3001
- 后端：http://127.0.0.1:8080

### 首次启动与改密

第一次启动创建账号 `admin`，随机临时密码保存于 `.runtime/admin-credentials.json`，JWT 密钥保存于 `.runtime/jwt-secret.txt`。Linux 文件权限为 600，运行目录为 700。可先执行 `node scripts/local.mjs init` 生成凭据。已有文件会保留，包括旧 PowerShell 脚本生成的文件；已有管理员密码不会重置。

首次登录会进入「设置正式密码」页面。在完成前，后端只允许临时会话设置正式密码或退出，无法访问管理接口。正式密码至少 15 个字符、UTF-8 编码不超过 72 字节，且不能与临时密码相同。完成后所有旧会话撤销，需要使用新密码重新登录。本地启动器会在几秒内删除凭据文件中的明文密码，仅保留 `username` 和 `initialized` 标记；正式密码不会写入该文件。重启不会重新生成初始密码。初始化完成后，可重复验证脚本通过环境变量临时接收正式密码。

升级旧安装时，仅当 `admin` 仍使用本地保存的已知初始化密码，才要求首次设置正式密码；已经更换密码的账户不会重置。若清空数据库重新安装，需要移除旧的已完成初始化标记文件，再运行 `init` 生成该新安装的临时密码。

原 PowerShell 脚本启动的服务需要先停止，再改用新命令；新控制器不会接管未知旧进程。如果启动终端被强制杀死，应确认三个端口和旧进程都已停止，再清理 `.runtime/local-supervisor.json` 残留记录。

### Linux 容器部署

将 `.env.example` 复制为 `.env`，填入至少 32 字节的随机 `KS_JWT_SECRET` 和 12–72 字节的 `KS_BOOTSTRAP_ADMIN_PASSWORD`，然后运行：

```text
docker compose up -d --build --wait
```

后端采用多阶段构建，在容器内生成 Linux 二进制；不需要先在宿主机生成 `backend/bin/server`。前端使用 lockfile 与 `npm ci`，不复制宿主机 `node_modules`。需要代理时通过标准 Docker 构建参数 `HTTP_PROXY` / `HTTPS_PROXY` 配置，不依赖 `http.docker.internal`。OAuth HTTP 代理默认关闭，需要时设置 `KS_OAUTH_GITHUB_HTTP_PROXY`。

Docker 部署中的 `KS_BOOTSTRAP_ADMIN_PASSWORD` 同样只是临时密码；完成首次改密后，应从部署配置或 Secret 中移除该变量。应用不会修改宿主机的 `.env` 文件，也不会记录你设置的正式密码。

当前 Compose 前端运行 Vite 开发服务器，适用于本地或测试环境；正式对外部署需要单独配置静态站点服务器、TLS 和实际数据库凭据。Compose 的 MySQL/Redis 端口仅绑定本机，头像存储通过命名卷持久化。

### 验证

```text
node --test scripts/local.test.mjs
```

Linux 本地 dev 短信环境中，在三个服务就绪后运行：

```sh
SENTINEL_VERIFY_ADMIN_PASSWORD='你的正式密码' SENTINEL_VERIFY_DEV_SMS=1 node scripts/verify-auth.mjs
```

Windows PowerShell 对应设置 `$env:SENTINEL_VERIFY_DEV_SMS='1'` 和 `$env:SENTINEL_VERIFY_ADMIN_PASSWORD='你的正式密码'`，随后运行相同 Node 命令。完成后清除该环境变量。验证脚本创建并清理临时用户、角色，不修改管理员密码。结果保存于 `.runtime/verification.json`。

其他检查：`go test ./...`、`go vet ./...`（在 backend）；`npm run build`（分别在两个前端）。

### 仓库约定

- 已通过 `.gitignore` 排除：`.runtime/`、`.env`、`backend/uploads/`、Go 覆盖率产物（`*.out`、`cov`、`cover`、`coverage`、`cp`、`hp`、`hfunc.txt`）、操作系统垃圾（`.DS_Store` 等）。
- 各服务均配有 `.dockerignore`，避免把 `node_modules`、`.git`、本地数据带入构建上下文。
- 提交前请勿把真实密钥（JWT 密钥、数据库密码、OAuth 凭据）写入仓库；生产环境请通过 Secret / 环境变量注入。

---

## English

### Project structure

```text
SentinelGo/
├── backend/            # Go backend (cmd / internal / pkg / config)
├── frontend/          # Vue admin frontend
├── user-frontend/     # Vue user frontend
├── scripts/           # Local launcher local.mjs, verification scripts, lint
├── docs/              # Design / security docs
├── docker-compose.yml # Full-stack orchestration for Linux
├── .env.example       # Deployment env template
└── README.md
```

### Tech stack

- Backend: Go 1.25 (multi-stage build into a static Linux binary inside the container)
- Frontend: Vue 3 + Vite (two separate projects: admin and user)
- Storage: MySQL 8.4, Redis 7.4
- Orchestration: Docker Engine + Compose v2

### Requirements

- Go 1.25.5 or newer
- Node.js 24 LTS
- Docker Engine and Compose v2

> On Linux you do not need Docker Desktop, PowerShell, or sudo to run the project scripts; the current user only needs Docker access.

### Local development

From the project root (same commands on every platform):

```text
npm --prefix frontend ci
npm --prefix user-frontend ci
node scripts/local.mjs start
```

The start command checks ports, brings up MySQL/Redis, waits for the database to be healthy, compiles the Go binary for the current platform, then starts all three services and waits until they are ready. Keep the terminal running; press Ctrl+C to stop the three application services, or run the following in another terminal:

```text
node scripts/local.mjs stop
```

The stop command uses the local supervisor started in this run. It does not rely on Windows process commands and will not kill unrelated processes by stale PID. Database containers and data are preserved. Logs live under `.runtime/*.log`. If you already have MySQL/Redis services, use `start --skip-dependencies` and supply connection parameters via environment variables such as `KS_DATABASE_MASTER_DSN` and `KS_REDIS_ADDR`.

- Admin: http://127.0.0.1:3000
- User:  http://127.0.0.1:3001
- API:   http://127.0.0.1:8080

### First start and password setup

On first start the account `admin` is created. A random temporary password is stored in `.runtime/admin-credentials.json` and the JWT secret in `.runtime/jwt-secret.txt`. On Linux those files are mode 600 and the runtime directory is mode 700. You can pre-generate credentials with `node scripts/local.mjs init`. Existing files are preserved, including those produced by the older PowerShell scripts; an existing admin password is not reset.

The first login lands on the "set a permanent password" page. Until it is completed, the backend only allows the temporary session to set the permanent password or sign out, and blocks access to management APIs. The permanent password must be at least 15 characters, UTF-8 encoded within 72 bytes, and different from the temporary password. After completion all prior sessions are revoked and you must sign in again. The local launcher removes the plaintext password from the credentials file within a few seconds, keeping only the `username` and `initialized` flags; the permanent password is never written to that file. A restart does not regenerate the initial password. Once initialized, the verification script can receive the permanent password through an environment variable on repeated runs.

When upgrading an existing install, the first-time password setup is only required if `admin` still uses the known locally-stored initialization password; accounts that already changed their password are not reset. If you wipe the database and reinstall, remove the old initialization flag file first, then run `init` to generate a fresh temporary password for that install.

Services started by the old PowerShell scripts must be stopped before switching to the new command; the new supervisor does not take over unknown legacy processes. If the start terminal was force-killed, confirm that the three ports and legacy processes are stopped, then clean up the leftover `.runtime/local-supervisor.json` record.

### Linux container deployment

Copy `.env.example` to `.env` and fill in a random `KS_JWT_SECRET` of at least 32 bytes and a `KS_BOOTSTRAP_ADMIN_PASSWORD` of 12–72 bytes, then run:

```text
docker compose up -d --build --wait
```

The backend uses a multi-stage build and produces the Linux binary inside the container; you do not need to build `backend/bin/server` on the host first. The frontends use the lockfile with `npm ci` and do not copy the host `node_modules`. To use a proxy, pass the standard Docker build args `HTTP_PROXY` / `HTTPS_PROXY`; it does not depend on `http.docker.internal`. The OAuth HTTP proxy is off by default; set `KS_OAUTH_GITHUB_HTTP_PROXY` when needed.

In the Docker deployment `KS_BOOTSTRAP_ADMIN_PASSWORD` is also only a temporary password; after the first password change you should remove that variable from your deployment config or Secret. The application never modifies the host `.env` file and never records the permanent password you set.

The Compose frontends currently run the Vite dev server, which is fine for local or test environments. A public-facing deployment needs a separate static-site server, TLS, and real database credentials. Compose binds MySQL/Redis ports to the host loopback only, and avatars are persisted through a named volume.

### Verification

```text
node --test scripts/local.test.mjs
```

In a local dev SMS environment on Linux, once the three services are ready run:

```sh
SENTINEL_VERIFY_ADMIN_PASSWORD='your-permanent-password' SENTINEL_VERIFY_DEV_SMS=1 node scripts/verify-auth.mjs
```

On Windows PowerShell set `$env:SENTINEL_VERIFY_DEV_SMS='1'` and `$env:SENTINEL_VERIFY_ADMIN_PASSWORD='your-permanent-password'`, then run the same Node command. Clear those environment variables afterward. The verification script creates and cleans up temporary users and roles and never modifies the admin password. Results are saved to `.runtime/verification.json`.

Other checks: `go test ./...` and `go vet ./...` (in backend); `npm run build` (in each frontend).

### Repository conventions

- `.gitignore` already excludes: `.runtime/`, `.env`, `backend/uploads/`, Go coverage artifacts (`*.out`, `cov`, `cover`, `coverage`, `cp`, `hp`, `hfunc.txt`), and OS noise (`.DS_Store`, etc.).
- Each service ships a `.dockerignore` so `node_modules`, `.git`, and local data are not pulled into the build context.
- Do not commit real secrets (JWT secret, database password, OAuth credentials). Inject them via Secrets / environment variables for production.
