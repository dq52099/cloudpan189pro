package bootstrap

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/configs"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func MigrateFromSQLite(cfg *configs.Config) error {
	sqlitePath := cfg.DBFile
	if sqlitePath == "" {
		sqlitePath = "data/data.db"
	}

	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Postgres.Host,
		cfg.Postgres.User,
		cfg.Postgres.Pass,
		cfg.Postgres.DBName,
		cfg.Postgres.Port,
		cfg.Postgres.SSLMode,
	)

	pgDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	if err := migrateUsers(sqliteDB, pgDB); err != nil {
		fmt.Printf("迁移用户数据失败: %v\n", err)
	} else {
		fmt.Println("用户数据迁移完成")
	}

	if err := migrateUserGroups(sqliteDB, pgDB); err != nil {
		fmt.Printf("迁移用户组数据失败: %v\n", err)
	} else {
		fmt.Println("用户组数据迁移完成")
	}

	if err := migrateCloudTokens(sqliteDB, pgDB); err != nil {
		fmt.Printf("迁移云盘令牌数据失败: %v\n", err)
	} else {
		fmt.Println("云盘令牌数据迁移完成")
	}

	if err := migrateAutoIngestPlans(sqliteDB, pgDB); err != nil {
		fmt.Printf("迁移自动订阅计划数据失败: %v\n", err)
	} else {
		fmt.Println("自动订阅计划数据迁移完成")
	}

	if err := migrateSettings(sqliteDB, pgDB); err != nil {
		fmt.Printf("迁移设置数据失败: %v\n", err)
	} else {
		fmt.Println("设置数据迁移完成")
	}

	return nil
}

func migrateUsers(src, dst *gorm.DB) error {
	var users []models.User
	if err := src.Find(&users).Error; err != nil {
		return err
	}
	if len(users) == 0 {
		return nil
	}
	for i := range users {
		users[i].ID = 0
	}
	return dst.Create(&users).Error
}

func migrateUserGroups(src, dst *gorm.DB) error {
	var groups []models.UserGroup
	if err := src.Find(&groups).Error; err != nil {
		return err
	}
	if len(groups) == 0 {
		return nil
	}
	for i := range groups {
		groups[i].ID = 0
	}
	return dst.Create(&groups).Error
}

func migrateGroup2Files(src, dst *gorm.DB) error {
	var relations []models.Group2File
	if err := src.Find(&relations).Error; err != nil {
		return err
	}
	if len(relations) == 0 {
		return nil
	}
	for i := range relations {
		relations[i].ID = 0
	}
	return dst.Create(&relations).Error
}

func migrateCloudTokens(src, dst *gorm.DB) error {
	var tokens []models.CloudToken
	if err := src.Find(&tokens).Error; err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}
	for i := range tokens {
		tokens[i].ID = 0
	}
	return dst.Create(&tokens).Error
}

func migrateAutoIngestPlans(src, dst *gorm.DB) error {
	var plans []models.AutoIngestPlan
	if err := src.Find(&plans).Error; err != nil {
		return err
	}
	if len(plans) == 0 {
		return nil
	}
	for i := range plans {
		plans[i].ID = 0
	}
	return dst.Create(&plans).Error
}

func migrateSettings(src, dst *gorm.DB) error {
	var settings []models.Setting
	if err := src.Find(&settings).Error; err != nil {
		return err
	}
	if len(settings) == 0 {
		return nil
	}
	for i := range settings {
		settings[i].ID = 0
	}
	return dst.Create(&settings).Error
}

func migrateMediaConfig(src, dst *gorm.DB) error {
	var configs []models.MediaConfig
	if err := src.Find(&configs).Error; err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}
	for i := range configs {
		configs[i].ID = 0
	}
	return dst.Create(&configs).Error
}

func migrateLoginLogs(src, dst *gorm.DB) error {
	var logs []models.LoginLog
	if err := src.Find(&logs).Error; err != nil {
		return err
	}
	if len(logs) == 0 {
		return nil
	}
	for i := range logs {
		logs[i].ID = 0
	}
	return dst.Create(&logs).Error
}

func migrateFileTaskLogs(src, dst *gorm.DB) error {
	var logs []models.FileTaskLog
	if err := src.Find(&logs).Error; err != nil {
		return err
	}
	if len(logs) == 0 {
		return nil
	}
	for i := range logs {
		logs[i].ID = 0
	}
	return dst.Create(&logs).Error
}

func migrateMountPoints(src, dst *gorm.DB) error {
	var mountPoints []models.MountPoint
	if err := src.Find(&mountPoints).Error; err != nil {
		return err
	}
	if len(mountPoints) == 0 {
		return nil
	}
	for i := range mountPoints {
		mountPoints[i].ID = 0
	}
	return dst.Create(&mountPoints).Error
}

func migrateVirtualFiles(src, dst *gorm.DB) error {
	var files []models.VirtualFile
	if err := src.Find(&files).Error; err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	batchSize := 1000
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}
		batch := files[i:end]
		for j := range batch {
			batch[j].ID = 0
		}
		if err := dst.Create(&batch).Error; err != nil {
			fmt.Printf("批量迁移虚拟文件失败: %v\n", err)
		}
	}
	return nil
}

func DataMigrationEnabled(cfg *configs.Config) bool {
	if cfg.DBType != "postgresql" && cfg.DBType != "postgres" {
		return false
	}
	if cfg.Postgres == nil {
		return false
	}
	return true
}

func ShouldMigrateData(cfg *configs.Config) bool {
	if !DataMigrationEnabled(cfg) {
		return false
	}
	sqlitePath := cfg.DBFile
	if sqlitePath == "" {
		sqlitePath = "data/data.db"
	}

	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		return false
	}

	var userCount int64
	sqliteDB.Model(&models.User{}).Count(&userCount)

	if userCount == 0 {
		return false
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Postgres.Host,
		cfg.Postgres.User,
		cfg.Postgres.Pass,
		cfg.Postgres.DBName,
		cfg.Postgres.Port,
		cfg.Postgres.SSLMode,
	)

	pgDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return false
	}

	var pgUserCount int64
	pgDB.Model(&models.User{}).Count(&pgUserCount)

	return userCount > 0 && pgUserCount == 0
}
