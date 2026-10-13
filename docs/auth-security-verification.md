# 权限与认证修复验证

修复日期：2026-10-10。

## 行为变化

- API 使用主库中的当前用户状态、启用角色和数据库登录会话鉴权。
- 初始化管理员使用临时凭据，首次登录强制设置正式密码；后端限制临时会话只能改密或退出，完成后事务性撤销全部旧会话。
- 用户降权、角色禁用/删除立即影响已有 Access Token；重启不会恢复已撤销的角色或权限。
- 退出只撤销当前会话；禁用、删除用户和改密撤销该用户所有会话。
- Refresh Token 每次使用后失效；会话撤销持久化在数据库，不依赖 Redis 黑名单。
- 新注册用户在同一事务中初始化密码认证记录、普通用户角色和 Casbin 映射。
- OAuth state 随机生成、绑定 HttpOnly SameSite=Lax Cookie，在数据库中保存五分钟且只可消费一次。
- 手机绑定使用独立 bind 验证码，并要求已登录；验证码校验与消费使用 Redis Lua 或内存互斥锁保证原子性。
- 认证身份使用联合唯一索引；数据库迁移失败阻止服务启动。
- 管理员创建用户使用请求中的密码；密码哈希错误不会覆盖已有密码。
- 个人资料和 OAuth 头像更新只写指定字段，避免覆盖并发改密或禁用操作。
- 两个前端使用共享刷新 Promise；失败时所有等待请求均 reject，重试次数受限。

已有无 sid 的令牌会被拒绝，需要重新登录。新增表为 auth_sessions 和 oauth_states。

## 本地启动

项目根目录运行：

```text
node scripts/local.mjs start

# 停止脚本记录的三个本地进程（保留数据库容器和数据）
node scripts/local.mjs stop
```

需要 Docker Engine / Desktop 与 Compose v2、Go、Node.js 和两个前端已安装的依赖。跨平台 Node 脚本启动 MySQL/Redis，构建后端并托管三个服务；保持终端打开，Ctrl+C 或 stop 命令停止服务。已有同端口服务时拒绝覆盖。详见根目录 README.md。

- 管理端：http://127.0.0.1:3000
- 用户端：http://127.0.0.1:3001
- 后端：http://127.0.0.1:8080

首次启动生成随机 JWT 密钥和管理员初始密码，保存于被 gitignore 排除的 `.runtime` 目录。管理员凭据在 `.runtime/admin-credentials.json`，本次启动的控制器记录在 `.runtime/local-supervisor.json`，日志分别为 backend/frontend/user-frontend 的 `.log`（包含标准输出和错误）。

完成首次改密后，本地启动器清除凭据文件中的初始密码，不保存正式密码；文件仅保留完成标记。正式密码最低 15 个字符，UTF-8 编码最多 72 字节，不能复用临时密码。完成前刷新令牌不提供管理权限。升级旧安装时仅对仍使用已知初始化密码的管理员启用该流程，已改密的账户保持原密码。

Docker 全栈部署时复制 `.env.example` 为 `.env`，配置新的 KS_JWT_SECRET 和首次管理员密码。JWT 密钥至少 32 字节，首次管理员密码为 12–72 字节。现有管理员密码不会被重置。

GitHub OAuth 需要通过环境变量提供新颁发的 Client ID/Secret。此前源码中存在的 OAuth Secret 应在提供商侧撤销；本次仅移除了项目中的明文值，没有执行提供商账户操作。

## 可重复验证

```sh
# 后端
cd backend
go test ./...
go vet ./...

# 前端（从项目根目录）
cd frontend
npm run build
node --experimental-vm-modules --test src/utils/request.test.mjs
cd ../user-frontend
npm run build

# 服务启动后（从项目根目录）
SENTINEL_VERIFY_ADMIN_PASSWORD='你的正式密码' SENTINEL_VERIFY_DEV_SMS=1 node scripts/verify-auth.mjs
```

实际 MySQL/Redis 验证完成 37 项检查，涵盖两个前端 HTTP 服务、注册、默认权限、角色分配/降权/禁用/删除、账户禁用/重新启用/删除、会话退出、刷新重放、手机号绑定、验证码重放、改密撤销和 OAuth 缺失 state 拒绝。脚本仅创建临时测试账户与角色，并在结束时清理它们。结果保存在 `.runtime/verification.json`。

浏览器检查完成：管理端登录及个人中心、用户端登录及个人资料、用户端退出登录。

短信验证使用本地 dev 通道，没有向真实手机发送短信。没有进行 GitHub 真实授权或阿里云短信网关的端到端调用。前端构建仍有依赖 PURE 注释及产物大小警告，不影响构建完成。

## 首次管理员改密验证

2026-10-10 新增验证：Go 覆盖临时会话限制、刷新不能绕过、旧密码确认、禁止复用临时密码、长度校验、改密撤销全部会话、正式密码登录、重启不重新初始化、旧安装迁移以及并发迁移不覆盖已完成状态。两个前端的请求拦截测试确认 20003 会进入改密页且不刷新或丢弃临时会话。

Windows 与 Linux 凭据文件清理测试通过；Linux 运行监督器检测到完成状态后清理文件，重启不生成新密码。独立 Linux/MySQL 临时数据库也验证了真实改密流程：并发提交仅一次成功、旧密码及会话失效、正式密码登录正常、重启状态持久化。完成后回归原有 37 项认证检查，临时容器、数据库及测试密码文件均已清理。

当前本地管理员只迁移为待设置正式密码状态，没有代用户选择正式密码；用户需要自行在页面完成设置。
