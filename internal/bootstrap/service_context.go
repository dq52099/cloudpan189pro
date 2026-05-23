package bootstrap

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"

	"github.com/casbin/casbin/v2"
	"github.com/xxcheng123/cloudpan189-share/internal/configs"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"moul.io/zapgorm2"
)

type ServiceContext interface {
	GetDB(ctx context.Context) *gorm.DB
	GetDBWithoutContext() *gorm.DB
	GetLogger(name string, fields ...zap.Field) *zap.Logger
	Close()
	GetPort() int
	GetTaskEngine() taskengine.TaskEngine
	GetHTTPEngine() *gin.Engine
}

type serviceContext struct {
	config     *configs.RuntimeConfig
	db         *gorm.DB
	logger     *zap.Logger
	taskEngine taskengine.TaskEngine
	httpEngine *gin.Engine
}

func (s *serviceContext) GetDB(ctx context.Context) *gorm.DB {
	return DBFromContext(ctx, s.db)
}

func (s *serviceContext) GetDBWithoutContext() *gorm.DB {
	return s.db
}

func (s *serviceContext) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return s.logger.Named(name).With(fields...)
}

func (s *serviceContext) Close() {
	_ = s.logger.Sync()
}

func (s *serviceContext) GetPort() int {
	return s.config.Port
}

func (s *serviceContext) GetTaskEngine() taskengine.TaskEngine {
	return s.taskEngine
}

func (s *serviceContext) GetHTTPEngine() *gin.Engine {
	return s.httpEngine
}

func New(c *configs.RuntimeConfig) (ServiceContext, error) {
	return newServiceContext(c)
}

func newServiceContext(c *configs.RuntimeConfig) (ServiceContext, error) {
	var (
		db     *gorm.DB
		logger *zap.Logger
		err    error
	)

	// 连接 db
	if db, err = connectDB(c.Config); err != nil {
		return nil, err
	}

	// 执行数据迁移（创建表结构）
	if err = migrateDB(db); err != nil {
		return nil, err
	}

	if err = migrateSQLiteDataIfNeeded(c.Config, ShouldMigrateData, MigrateFromSQLite); err != nil {
		return nil, err
	}

	// 初始化日志
	if logger, err = initLogger(c); err != nil {
		return nil, err
	}

	// 初始化 setting
	if err = initSetting(db); err != nil {
		return nil, err
	}

	// 初始化全局共享变量
	if err = assignShared(db); err != nil {
		return nil, err
	}

	taskEngine := initTaskEngine(logger, c.TaskEngine)

	// 如果数据库中设置了 WorkerCount，优先使用数据库值
	if shared.SettingAddition.WorkerCount > 0 {
		if err := taskEngine.SetWorkerCount(shared.SettingAddition.WorkerCount); err != nil {
			logger.Warn("从数据库设置工作流数失败", zap.Error(err))
		} else {
			logger.Info("已从数据库加载工作流数", zap.Int("workerCount", shared.SettingAddition.WorkerCount))
		}
	}

	gLogger := zapgorm2.New(logger)
	gLogger.SetAsDefault()
	gLogger.IgnoreRecordNotFoundError = true
	gLogger.SlowThreshold = time.Second * 3

	db = db.Session(&gorm.Session{
		Logger: gLogger,
	})

	httpEngine := gin.New()
	httpEngine.Use(gin.Recovery())

	return &serviceContext{
		config:     c,
		db:         db,
		logger:     logger,
		taskEngine: taskEngine,
		httpEngine: httpEngine,
	}, nil
}

func migrateSQLiteDataIfNeeded(
	cfg *configs.Config,
	shouldMigrate func(*configs.Config) bool,
	migrate func(*configs.Config) error,
) error {
	if !shouldMigrate(cfg) {
		return nil
	}

	fmt.Println("检测到 SQLite 数据，正在迁移到 PostgreSQL...")

	if err := migrate(cfg); err != nil {
		return fmt.Errorf("数据迁移失败: %w", err)
	}

	fmt.Println("数据迁移完成!")

	return nil
}

type mockServiceContext struct {
}

func (m *mockServiceContext) GetDB(ctx context.Context) *gorm.DB {
	return nil
}

func (m *mockServiceContext) GetDBWithoutContext() *gorm.DB {
	return nil
}

func (m *mockServiceContext) GetLogger(name string, fields ...zap.Field) *zap.Logger {
	return zap.NewNop()
}

func (m *mockServiceContext) GetFileEnforcer() *casbin.Enforcer {
	return nil
}

func (m *mockServiceContext) Close() {
}

func (m *mockServiceContext) GetPort() int {
	return 9999
}

func (m *mockServiceContext) GetTaskEngine() taskengine.TaskEngine {
	return nil
}

func (m *mockServiceContext) GetHTTPEngine() *gin.Engine {
	return nil
}

func NewMockServiceContext() ServiceContext {
	return &mockServiceContext{}
}
