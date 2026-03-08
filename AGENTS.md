# AGENTS.md - 开发指南

代码代理/AI 助手的开发规范指导。

## 项目概述

基于 Go 后端 + Vue.js 前端的天翼云盘 WebDAV 应用。

## 开发命令

### 后端 (Go)

```bash
make build; make build-frontend; make build-backend; make build-multi-arch  # 构建
make dev; go test -v ./...; go test -v ./internal/services/... -run TestName  # 开发测试
make lint; make lint-clean  # Lint
make docker-build; make docker-run; make docker-stop; make docker-logs  # Docker
make swag-init; make swag-fmt  # Swagger
make clean; make clean-all  # 清理
```

### 前端 (Vue.js)

```bash
cd fe
npm run dev; npm run build; npm run lint; npm run format; npm run lint:css
```

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
unauthorizedBusinessError("自定义消息")           // 未授权错误
invalidParamsBusinessError(err)                  // 参数错误
&businessError{httpCode: http.StatusBadRequest, businessCode: 40000, message: "自定义错误"}  // 自定义错误
```

#### 标签规范

- JSON: `json:"field_name"`
- GORM: `gorm:"column:xxx;primaryKey"`
- Validator: `binding:"required,email"`

#### 测试规范

- 测试文件: `*_test.go`
- 测试数据: 环境变量存储 (见 `internal/consts` 的 `EnvKeyTest*`)
- 测试上下文: 使用 `bootstrap.NewMockServiceContext()`

```bash
export TEST_ACCESS_TOKEN=xxx; export TEST_PERSON_FILE_ID=xxx; go test -v ./... -run TestName
```

### Vue.js 前端

#### 技术栈

Vue 3 + TypeScript, Vite, Pinia, Naive UI, Vue Router

#### 目录结构

```
fe/src/
├── api/; ├── components/; ├── stores/; ├── views/; ├── utils/; └── router/
```

#### 命名规范

- 组件: PascalCase (如 `SubscriptionList.vue`)
- 组合式函数: useXxx (如 `useUser.ts`)
- 工具函数/CSS 类: 小写下划线

#### 规范

- TypeScript 严格模式; `<script setup lang="ts">`; scoped CSS; 使用 Naive UI

## 项目结构

```
.
├── cmd/main.go  # 后端入口
├── internal/    # 后端代码
│   ├── bootstrap/; consts/; framework/; handler/; middleware/; repository/; services/
├── fe/          # 前端
├── etc/         # 配置
└── data/        # SQLite
```

## 数据库

- 默认: SQLite (`data/share.db`); 支持: MySQL, PostgreSQL; ORM: GORM

## 关键约束

1. 不提交敏感信息 (.env, 密钥)
2. 敏感配置用环境变量
3. 用 `go mod tidy` 整理依赖
4. 前后端默认端口: 12395
5. 中文注释
6. 运行 `make lint` 确保代码通过检查
7. 运行 `make build` 确保前后端都能正常构建

## Lint 配置

golangci-lint 配置见 `.golangci.yml`，主要启用:
- nlreturn (return 前必须有空行); errcheck (检查错误处理); wsl (空行风格)