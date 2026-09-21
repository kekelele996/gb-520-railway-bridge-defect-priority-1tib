
# 铁路桥梁缺陷处置优先级

铁路桥梁、检查批次、缺陷证据和处置优先级平台。项目采用前后端分离和明确的领域分层，重点保证状态迁移、RBAC、审计日志、请求追踪与限流在各层保持一致。

## Docker Compose 快速启动

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

- Web 工作台：http://127.0.0.1:18520
- 后端健康检查：http://127.0.0.1:19520/healthz
- 后端 API：http://127.0.0.1:19520/api
- 演示账号统一密码：`Admin123!`（仅限本地演示，生产环境必须更换）

| 账号 | 角色 | 权限 |
|---|---|---|
| `viewer` | viewer | 查看四类业务数据，不能写入或查看审计 |
| `operator` | operator | 新建、编辑和推进一般业务记录，拟制优先级草稿 |
| `reviewer` | reviewer | operator 权限，并可复核他人拟制的优先级和查看审计 |
| `admin` | admin | 全部权限，包括删除 |

停止并清理本项目容器与数据卷：

```bash
docker compose down -v --remove-orphans
```

## 主要功能

| 业务模块 | 后端实体 | API 前缀 | 状态流 |
|---|---|---|---|
| 桥梁资产 | `BridgeAsset` | `/api/bridges` | active, restricted, closed, retired |
| 检查批次 | `InspectionRound` | `/api/inspections` | planned, running, review, completed |
| 缺陷发现 | `DefectFinding` | `/api/defects` | new, verified, monitoring, mitigated, closed |
| 优先级决定 | `PriorityDecision` | `/api/priorities` | draft → observe/restrict/urgent（终态） |

- JWT 登录和 viewer/operator/reviewer/admin 四级 RBAC，后端路由与前端守卫、导航和按钮保持一致。
- 所有状态变化使用乐观锁并写入审计日志；审计查询仅 reviewer/admin 可见。
- 优先级决定的每次创建、草稿更新和定稿均追加不可变版本，保留证据、状态、操作者、request ID 和完整快照。
- 优先级只能由不同于拟制人的 reviewer/admin 定稿；observe/restrict/urgent 均为不可覆盖终态。
- 请求 ID、结构化日志、全局错误映射和 Redis 分布式限流。
- 提供脱敏运行配置、当前会话、审计汇总和单实体审计历史接口。
- 业务工作台支持查询、新建、状态推进、风险标识及操作审计查看。

## 技术栈

| 层次 | 技术 |
|---|---|
| 前端 | Vue 3 + TypeScript + Vite + Element Plus |
| 后端 | Go 1.22 + Gin + GORM |
| 数据 | PostgreSQL + Redis、MinIO |
| 部署 | Docker Compose + Nginx |

## 本地开发

后端可使用 SQLite 开发模式，不需要先启动数据库：

```bash
cd backend
go mod download
DATABASE_DRIVER=sqlite DATABASE_DSN=local.db REDIS_ADDR='' \
JWT_SECRET=local-development-secret PORT=8080 go run ./cmd/server
```

前端开发服务器：

```bash
cd frontend
npm install
npm run dev
```

质量检查：

```bash
cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./...
cd ../frontend && npm run typecheck && npm run build
cd .. && docker compose config --quiet
```

也可以从项目根目录执行 `./scripts/validate.sh`。脚本会先删除本项目旧卷，从空 PostgreSQL/Redis/MinIO 数据卷构建启动，验证四角色 RBAC、独立复核和三版证据链，并在成功或失败时关闭容器和删除卷。设置 `KEEP_RUNNING=1` 可在 API 验收后暂留服务供浏览器检查。

## 目录结构

```text
.
├── backend/
│   ├── cmd/server/                 # 服务入口与优雅退出
│   └── internal/
│       ├── config/                 # 环境配置
│       ├── constants/              # 状态枚举与迁移图
│       ├── database/               # 连接、迁移与演示数据
│       ├── dto/                    # 输入契约
│       ├── handler/                # HTTP 接口
│       ├── middleware/             # JWT、追踪、限流
│       ├── model/                  # GORM 实体
│       ├── repository/             # 持久化边界
│       ├── router/                 # 路由装配
│       ├── service/                # 业务规则与审计
│       └── util/                   # 统一 HTTP 响应
├── frontend/src/
│   ├── api/                        # 按实体拆分的 API
│   ├── components/common/          # 共享业务组件
│   ├── hooks/                      # 认证与分页 hooks
│   ├── pages/                      # 五个路由页面
│   ├── router/                     # 路由配置
│   ├── stores/                     # 按实体拆分的状态仓库
│   ├── types/                      # 共享类型与枚举
│   └── utils/                      # 格式化与状态工具
├── docker-compose.yml
├── VALIDATION.md
└── runtime_smoke.json
```

`PriorityDecisionRevision` 位于 `backend/internal/model/priority_decision.go`，与主记录在同一事务写入；查询 `/api/priorities` 或 `/api/priorities/:id` 时按版本升序返回 `revisions`。

## 共享枚举位置

| 枚举 | 值 | 前后端出现位置 |
|---|---|---|
| `DefectState` | `new, verified, monitoring, mitigated, closed` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |
| `PriorityLevel` | `observe, restrict, urgent` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |

每个实体自己的完整迁移图同样位于 `backend/internal/constants/status.go`；页面使用的状态列表位于 `frontend/src/types/status.ts`。修改状态时必须同步两处并更新对应服务测试。

## 环境变量

| 变量 | 说明 |
|---|---|
| `COMPOSE_PROJECT_NAME` | 固定英文 Compose 项目名，支持中文父目录 |
| `DB_NAME/DB_USER/DB_PASSWORD` | 数据库名称与业务账号 |
| `DB_ROOT_PASSWORD` | MySQL 管理员密码（PostgreSQL 项目保留统一模板字段） |
| `JWT_SECRET` | JWT 签名密钥，生产环境必须替换 |
| `FRONTEND_PORT/BACKEND_PORT/DB_PORT` | 宿主机端口映射 |
| `REDIS_PORT` | Redis 宿主机端口 |
| `MINIO_*` | 证据对象存储配置（启用 MinIO 的项目） |

## API 使用示例

```bash
token=$(curl -sS -X POST http://127.0.0.1:19520/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin123!"}' | jq -r '.data.token')

curl -sS http://127.0.0.1:19520/api/overview \
  -H "Authorization: Bearer $token"
```

## License

MIT
