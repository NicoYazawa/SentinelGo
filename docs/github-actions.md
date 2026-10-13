# GitHub CI/CD

## 持续集成

分支推送、PR 和手动运行触发 `.github/workflows/ci.yml`：

- Go 版本读取 `backend/go.mod`，执行 gofmt、模块校验、vet、race 测试及编译。Redis 服务用于现有集成测试。
- Node.js 24 通过 npm ci 安装锁定依赖，执行管理端 Vitest，构建两个前端（包含 TypeScript 检查）。用户端目前没有测试脚本。
- 执行本地启动器的 Node 测试。
- 构建三个 Linux Docker 镜像，不推送。

可在 GitHub 分支保护中将检查设为合并条件。CI 只使用只读仓库权限。

## 发布

将项目放在 GitHub 仓库根目录并推送源码，随后发布版本标签：

```sh
git tag v1.0.0
git push origin v1.0.0
```

标签工作流先运行全部 CI，通过后发布三个 linux/amd64 镜像：

```text
ghcr.io/owner/repository/backend:v1.0.0
ghcr.io/owner/repository/frontend:v1.0.0
ghcr.io/owner/repository/user-frontend:v1.0.0
```

owner/repository 自动转为小写。支持 `v1.0.0-rc.1` 等预发布标签，只发布版本标签，不维护 latest。三个镜像分别推送，某一步失败可能留下部分版本镜像；整个工作流成功后再部署。

发布使用 GitHub 自带的 `GITHUB_TOKEN`，发布任务获得 `packages: write`，无需额外发布 Secret。仓库和组织策略必须允许 Actions 写入 Packages。首次创建的包可能是私有的；公开拉取需要在 GHCR Package 设置改为 public，私有拉取需要有 `read:packages` 权限的令牌。已有同名包需要允许此仓库的 Actions 写入。

## 部署已发布镜像

复制 `.env.example` 为 `.env`，填写应用密钥和初始化密码，再追加：

```dotenv
GHCR_REPOSITORY=owner/repository
IMAGE_TAG=v1.0.0
```

`GHCR_REPOSITORY` 必须全部小写。私有包先执行 `docker login ghcr.io`，然后运行：

```sh
docker compose -f docker-compose.yml -f docker-compose.release.yml pull
docker compose -f docker-compose.yml -f docker-compose.release.yml up -d --no-build --wait
```

两个前端使用 Nginx 服务构建产物，保持管理端 3000、用户端 3001 端口。`VITE_PROXY_TARGET` 在容器启动时配置代理，支持 `/api`、`/uploads`、`/healthz`、`/readyz`；客户端路由回退到 index.html。本地 npm dev 开发流程不变。

升级时修改 `IMAGE_TAG` 并重复部署命令。回退镜像可改回旧标签；数据库迁移和数据变化需单独评估。此方案发布镜像，不自动连接服务器部署。

Actions 参考：[setup-go](https://github.com/actions/setup-go)、[setup-node](https://github.com/actions/setup-node)、[Docker build-push](https://github.com/docker/build-push-action)。
