请生成 `railway-bridge-defect-priority`「铁路桥梁缺陷处置优先级」Go 全栈项目，面向铁路基础设施单位管理桥梁、检查批次、缺陷等级和处置优先级决定。它是安全基础设施决策系统，不做票务、仓库、订单、工单客服或大众看板。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`BridgeAsset`（桥梁与结构部位）、`InspectionRound`（检查批次与人员）、`DefectFinding`（缺陷、证据、等级）、`PriorityDecision`（限速/观察/立即处置决定）贯穿数据库、Go model/service/handler 和前端。

### 核心页面

`/bridges` 桥梁；`/inspections` 检查批次；`/defects` 缺陷工作台；`/priorities` 优先级审核；`/audit` 审计。`SeverityBadge` 在缺陷和优先级页共用，`EvidenceGallery` 在检查和缺陷页共用。

### 横切关注点

RBAC 联动角色表、Go 认证授权中间件、前端路由守卫和按钮；优先级决定必须版本化审计，记录证据、状态迁移、操作者和 request ID；实现全局错误处理和限流。

### 共享枚举/组件

同步 `DefectState`（new/verified/monitoring/mitigated/closed）与 `PriorityLevel`（observe/restrict/urgent）。共享 `StatusBadge`、`EvidenceGallery`、`ConfirmDialog`，hooks 为 `useAuth`、`usePagination`。

### 技术与规模要求

前端 Vue 3 + TypeScript + Vite + Element Plus；后端 Go 1.22 + Gin + GORM；PostgreSQL + Redis + MinIO。目标 3000–4200 行、30–42 个 `.go` 文件。

### 文件结构强制清单

前端必须有 `api/stores/types/components/common/hooks/pages/router/utils`；后端必须有 `model/dto/repository/service/handler/router/middleware/constants/util`，README 列出枚举的前后端位置，禁止职责合并。

### 结构红线

严禁合并职责到单一文件；缺陷证据和优先级决定必须跨层拆分。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: railway-bridge-defect-priority`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=railway-bridge-defect-priority`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18520`、后端端口 `19520`；Nginx `/api` 反代、数据库健康检查、命名卷和 `condition: service_healthy` 齐全，提供真实 `/healthz`、Git 初始化。
