# AGENTS.md - 开发指南

代码代理/AI 助手的开发规范指导。

## 项目概述

基于 Go 后端 + Vue.js 前端的天翼云盘 WebDAV 应用。

## 开发命令

### 后端 (Go)

```bash
# 构建
make build              # 构建前端+后端
make build-frontend     # 仅构建前端
make build-backend      # 仅构建后端
make build-multi-arch   # 多架构构建 (linux/windows/darwin, amd64/arm64)

# 开发测试
make dev                # 启动开发服务器
go test -v ./...        # 运行所有测试
go test -v ./internal/services/... -run TestName  # 运行单个测试

# Lint
make lint               # 运行 linter
make lint-clean         # 清理 linter 缓存

# Docker
make docker-build       # 构建 Docker 镜像
make docker-run         # 运行容器
make docker-stop        # 停止容器
make docker-logs        # 查看日志

# Swagger
make swag-init          # 生成 Swagger 文档
make swag-fmt           # 格式化 Swagger 注释

# 清理
make clean              # 清理构建产物
make clean-all          # 完整清理
```

### 前端 (Vue.js)

```bash
cd fe
npm run dev           # 开发服务器
npm run build         # 构建 (vue-tsc && vite build)
npm run lint          # ESLint (自动修复)
npm run format        # Prettier 格式化
npm run lint:css      # Stylelint CSS 检查
```

### 测试环境变量

测试数据通过环境变量存储 (见 `internal/consts` 的 `EnvKeyTest*`):

```bash
export TEST_ACCESS_TOKEN=xxx
export TEST_PERSON_FILE_ID=xxx
go test -v ./... -run TestName
```

使用 `bootstrap.NewMockServiceContext()` 创建测试上下文。

## 代码规范

### Go 后端

#### 导入顺序

标准库 → 第三方库 → 项目内部包。冲突时用别名:

```go
import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
    "go.uber.org/zap"
    "gorm.io/gorm"
)
```

#### 命名规范

- 包: 小写单词 (如 `tmdb`, `repository`)
- 类型/接口: PascalCase (如 `Service`)
- 变量/函数: camelCase
- 接口: 以 `er` 结尾 (如 `Reader`)
- 常量: 驼峰或全大写下划线

#### 错误处理

实现 `httpcontext.BusinessError` 接口，使用内置函数创建错误:

```go
unauthorizedBusinessError("自定义消息")     // 未授权错误
invalidParamsBusinessError(err)             // 参数错误
&businessError{
    httpCode:     http.StatusBadRequest,
    businessCode: 40000,
    message:      "自定义错误",
}                                           // 自定义错误
```

#### 标签规范

- JSON: `json:"field_name"`
- GORM: `gorm:"column:xxx;primaryKey"`
- Validator: `binding:"required,email"`

#### 测试规范

- 测试文件: `*_test.go`
- 测试数据: 环境变量存储
- 测试上下文: 使用 `bootstrap.NewMockServiceContext()`

### Vue.js 前端

#### 技术栈

Vue 3 + TypeScript, Vite, Pinia, Naive UI, Vue Router, ESLint, Prettier

#### 目录结构

```
fe/src/
├── api/          # API 请求
├── components/   # 公共组件
├── stores/       # Pinia 状态管理
├── views/        # 页面视图
├── utils/        # 工具函数
└── router/       # 路由配置
```

#### 命名规范

- 组件: PascalCase (如 `SubscriptionList.vue`)
- 组合式函数: useXxx (如 `useUser.ts`)
- 工具函数/CSS 类: 小写下划线

#### 规范

- TypeScript 严格模式
- `<script setup lang="ts">`
- scoped CSS
- 使用 Naive UI 组件库

## 项目结构

```
.
├── cmd/main.go              # 后端入口
├── internal/                # 后端代码
│   ├── bootstrap/           # 启动配置
│   ├── consts/              # 常量定义
│   ├── framework/           # 框架层
│   ├── handler/             # 处理器 (http/consumer/scheduler)
│   ├── middleware/          # 中间件
│   ├── pkgs/                # 公共包
│   ├── repository/          # 数据层
│   ├── services/            # 业务逻辑
│   └── types/               # 类型定义
├── fe/                      # 前端代码
├── etc/                     # 配置文件
└── data/                    # SQLite 数据目录
```

## 数据库

- 默认: SQLite (`data/share.db`)
- 支持: MySQL, PostgreSQL
- ORM: GORM

## 关键约束

1. 不提交敏感信息 (.env, 密钥)
2. 敏感配置用环境变量
3. 用 `go mod tidy` 整理依赖
4. 前后端默认端口: 12395
5. 中文注释
6. 运行 `make lint` 确保代码通过检查
7. 运行 `make build` 确保前后端都能正常构建
8. 确保所有测试通过后再提交

## Lint 配置

golangci-lint 配置见 `.golangci.yml`，主要启用:

- `nlreturn`: return 前必须有空行
- `errcheck`: 检查错误处理
- `wsl`: 空行风格 (allow-first-in-block: true)

## 常用开发流程

1. 修改代码后运行 `make lint` 检查代码
2. 运行 `go test ./...` 确保测试通过
3. 运行 `make build` 确保构建成功
4. 提交前确认前后端都能正常构建