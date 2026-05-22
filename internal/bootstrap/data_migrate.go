package bootstrap

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/configs"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errMissingSourceTable = errors.New("source table is missing")

const dataMigrationBatchSize = 1000

type dataMigrationStep struct {
	name string
	run  func(*gorm.DB) error
}

type migrationNaturalUniqueConflict struct {
	table string
	key   string
	ids   []int64
}

type migrationNaturalUniqueConflictError struct {
	conflicts []migrationNaturalUniqueConflict
}

func (e *migrationNaturalUniqueConflictError) Error() string {
	parts := make([]string, 0, len(e.conflicts))
	for _, conflict := range e.conflicts {
		parts = append(parts, fmt.Sprintf("%s %s ids=%v", conflict.table, conflict.key, conflict.ids))
	}

	return "SQLite 源数据存在自然唯一键冲突，无法安全自动合并，请先清理源数据: " + strings.Join(parts, "; ")
}

func MigrateFromSQLite(cfg *configs.Config) error {
	sqlitePath := cfg.DBFile
	if sqlitePath == "" {
		sqlitePath = "data/data.db"
	}

	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer closeGormDB(sqliteDB)

	if err := sqliteDB.Exec("PRAGMA encoding = 'UTF-8'").Error; err != nil {
		return fmt.Errorf("failed to set SQLite encoding: %w", err)
	}

	if err := preflightSQLiteNaturalUniqueKeys(sqliteDB); err != nil {
		return err
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
	defer closeGormDB(pgDB)

	if err := pgDB.Exec("SET client_encoding = 'UTF8'").Error; err != nil {
		return fmt.Errorf("failed to set PostgreSQL encoding: %w", err)
	}

	var userCount int64
	if err := sqliteDB.Model(&models.User{}).Count(&userCount).Error; err != nil {
		fmt.Printf("检查用户数量失败: %v\n", err)
	} else {
		fmt.Printf("检测到 SQLite 用户数: %d\n", userCount)
	}

	steps := []dataMigrationStep{
		{name: "用户组", run: func(dst *gorm.DB) error { return migrateUserGroups(sqliteDB, dst) }},
		{name: "用户", run: func(dst *gorm.DB) error { return migrateUsers(sqliteDB, dst) }},
		{name: "云盘令牌", run: func(dst *gorm.DB) error { return migrateCloudTokens(sqliteDB, dst) }},
		{name: "虚拟文件", run: func(dst *gorm.DB) error { return migrateVirtualFiles(sqliteDB, dst) }},
		{name: "挂载点", run: func(dst *gorm.DB) error { return migrateMountPoints(sqliteDB, dst) }},
		{name: "用户组文件关系", run: func(dst *gorm.DB) error { return migrateGroup2Files(sqliteDB, dst) }},
		{name: "用户挂载点令牌", run: func(dst *gorm.DB) error { return migrateUserMountPointTokens(sqliteDB, dst) }},
		{name: "自动订阅计划", run: func(dst *gorm.DB) error { return migrateAutoIngestPlans(sqliteDB, dst) }},
		{name: "自动订阅日志", run: func(dst *gorm.DB) error { return migrateAutoIngestLogs(sqliteDB, dst) }},
		{name: "媒体文件", run: func(dst *gorm.DB) error { return migrateMediaFiles(sqliteDB, dst) }},
		{name: "媒体配置", run: func(dst *gorm.DB) error { return migrateMediaConfig(sqliteDB, dst) }},
		{name: "文件任务日志", run: func(dst *gorm.DB) error { return migrateFileTaskLogs(sqliteDB, dst) }},
		{name: "登录日志", run: func(dst *gorm.DB) error { return migrateLoginLogs(sqliteDB, dst) }},
		{name: "Telegram 设置", run: func(dst *gorm.DB) error { return migrateTelegramSettings(sqliteDB, dst) }},
		{name: "Telegram 用户", run: func(dst *gorm.DB) error { return migrateTelegramUsers(sqliteDB, dst) }},
		{name: "订阅", run: func(dst *gorm.DB) error { return migrateSubscriptions(sqliteDB, dst) }},
		{name: "订阅匹配历史", run: func(dst *gorm.DB) error { return migrateMatchHistory(sqliteDB, dst) }},
		{name: "每日热门历史", run: func(dst *gorm.DB) error { return migrateDailyHotHistory(sqliteDB, dst) }},
		{name: "系统设置", run: func(dst *gorm.DB) error { return migrateSystemSettings(sqliteDB, dst) }},
		{name: "设置", run: func(dst *gorm.DB) error { return migrateSettings(sqliteDB, dst, userCount) }},
	}

	return runDataMigrationSteps(pgDB, steps)
}

func preflightSQLiteNaturalUniqueKeys(src *gorm.DB) error {
	checks := []func(*gorm.DB) ([]migrationNaturalUniqueConflict, error){
		checkUserNaturalUniqueKeys,
		checkUserGroupNaturalUniqueKeys,
		checkVirtualFileNaturalUniqueKeys,
		checkMountPointNaturalUniqueKeys,
		checkMediaFileNaturalUniqueKeys,
		checkTelegramUserNaturalUniqueKeys,
		checkSystemSettingNaturalUniqueKeys,
	}

	conflicts := make([]migrationNaturalUniqueConflict, 0)

	for _, check := range checks {
		found, err := check(src)
		if err != nil {
			return err
		}

		conflicts = append(conflicts, found...)
	}

	if len(conflicts) == 0 {
		return nil
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].table == conflicts[j].table {
			return conflicts[i].key < conflicts[j].key
		}

		return conflicts[i].table < conflicts[j].table
	})

	return &migrationNaturalUniqueConflictError{conflicts: conflicts}
}

type userNaturalKeyRow struct {
	ID       int64  `gorm:"column:id"`
	Username string `gorm:"column:username"`
}

type userGroupNaturalKeyRow struct {
	ID   int64  `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

type virtualFileNaturalKeyRow struct {
	ID       int64  `gorm:"column:id"`
	ParentID int64  `gorm:"column:parent_id"`
	Name     string `gorm:"column:name"`
}

type mountPointNaturalKeyRow struct {
	ID     int64  `gorm:"column:id"`
	FileID int64  `gorm:"column:file_id"`
	Name   string `gorm:"column:name"`
}

type mediaFileNaturalKeyRow struct {
	ID   int64  `gorm:"column:id"`
	Path string `gorm:"column:path"`
}

type telegramUserNaturalKeyRow struct {
	ID     int64 `gorm:"column:id"`
	UserID int64 `gorm:"column:user_id"`
}

type systemSettingNaturalKeyRow struct {
	ID   int64  `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

func checkUserNaturalUniqueKeys(src *gorm.DB) ([]migrationNaturalUniqueConflict, error) {
	var rows []userNaturalKeyRow

	return collectNaturalUniqueConflicts(src,
		new(models.User).TableName(),
		[]string{"id", "username"},
		&rows,
		func(row userNaturalKeyRow) string { return fmt.Sprintf("username=%q", row.Username) },
		func(row userNaturalKeyRow) int64 { return row.ID },
	)
}

func checkUserGroupNaturalUniqueKeys(src *gorm.DB) ([]migrationNaturalUniqueConflict, error) {
	var rows []userGroupNaturalKeyRow

	return collectNaturalUniqueConflicts(src,
		new(models.UserGroup).TableName(),
		[]string{"id", "name"},
		&rows,
		func(row userGroupNaturalKeyRow) string { return fmt.Sprintf("name=%q", row.Name) },
		func(row userGroupNaturalKeyRow) int64 { return row.ID },
	)
}

func checkVirtualFileNaturalUniqueKeys(src *gorm.DB) ([]migrationNaturalUniqueConflict, error) {
	var rows []virtualFileNaturalKeyRow

	return collectNaturalUniqueConflicts(src,
		new(models.VirtualFile).TableName(),
		[]string{"id", "parent_id", "name"},
		&rows,
		func(row virtualFileNaturalKeyRow) string {
			return fmt.Sprintf("parent_id=%d,name=%q", row.ParentID, utils.SanitizeFileName(row.Name))
		},
		func(row virtualFileNaturalKeyRow) int64 { return row.ID },
	)
}

func checkMountPointNaturalUniqueKeys(src *gorm.DB) ([]migrationNaturalUniqueConflict, error) {
	var rows []mountPointNaturalKeyRow

	return collectNaturalUniqueConflicts(src,
		new(models.MountPoint).TableName(),
		[]string{"id", "file_id", "name"},
		&rows,
		func(row mountPointNaturalKeyRow) string {
			return fmt.Sprintf("file_id=%d,name=%q", row.FileID, row.Name)
		},
		func(row mountPointNaturalKeyRow) int64 { return row.ID },
	)
}

func checkMediaFileNaturalUniqueKeys(src *gorm.DB) ([]migrationNaturalUniqueConflict, error) {
	var rows []mediaFileNaturalKeyRow

	return collectNaturalUniqueConflicts(src,
		new(models.MediaFile).TableName(),
		[]string{"id", "path"},
		&rows,
		func(row mediaFileNaturalKeyRow) string { return fmt.Sprintf("path=%q", row.Path) },
		func(row mediaFileNaturalKeyRow) int64 { return row.ID },
	)
}

func checkTelegramUserNaturalUniqueKeys(src *gorm.DB) ([]migrationNaturalUniqueConflict, error) {
	var rows []telegramUserNaturalKeyRow

	return collectNaturalUniqueConflicts(src,
		new(models.TelegramUser).TableName(),
		[]string{"id", "user_id"},
		&rows,
		func(row telegramUserNaturalKeyRow) string { return fmt.Sprintf("user_id=%d", row.UserID) },
		func(row telegramUserNaturalKeyRow) int64 { return row.ID },
	)
}

func checkSystemSettingNaturalUniqueKeys(src *gorm.DB) ([]migrationNaturalUniqueConflict, error) {
	var rows []systemSettingNaturalKeyRow

	return collectNaturalUniqueConflicts(src,
		new(SystemSetting).TableName(),
		[]string{"id", "name"},
		&rows,
		func(row systemSettingNaturalKeyRow) string { return fmt.Sprintf("name=%q", row.Name) },
		func(row systemSettingNaturalKeyRow) int64 { return row.ID },
	)
}

func collectNaturalUniqueConflicts[T any](
	src *gorm.DB,
	tableName string,
	columns []string,
	rows *[]T,
	keyFn func(T) string,
	idFn func(T) int64,
) ([]migrationNaturalUniqueConflict, error) {
	if !src.Migrator().HasTable(tableName) {
		return nil, nil
	}

	if err := src.Table(tableName).Select(strings.Join(columns, ", ")).Find(rows).Error; err != nil {
		return nil, fmt.Errorf("检查%s自然唯一键失败: %w", tableName, err)
	}

	idsByKey := make(map[string][]int64, len(*rows))
	for _, row := range *rows {
		key := keyFn(row)
		idsByKey[key] = append(idsByKey[key], idFn(row))
	}

	keys := make([]string, 0, len(idsByKey))
	for key := range idsByKey {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	conflicts := make([]migrationNaturalUniqueConflict, 0)

	for _, key := range keys {
		ids := idsByKey[key]
		if len(ids) <= 1 {
			continue
		}

		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		conflicts = append(conflicts, migrationNaturalUniqueConflict{
			table: tableName,
			key:   key,
			ids:   ids,
		})
	}

	return conflicts, nil
}

func runDataMigrationSteps(dst *gorm.DB, steps []dataMigrationStep) error {
	return dst.Transaction(func(tx *gorm.DB) error {
		for _, step := range steps {
			if err := runMigrationStep(step.name, func() error { return step.run(tx) }); err != nil {
				return err
			}
		}

		return nil
	})
}

func migrateUsers(src, dst *gorm.DB) error {
	return migrateRows[models.User](src, dst, new(models.User).TableName())
}

func migrateUserGroups(src, dst *gorm.DB) error {
	return migrateRows[models.UserGroup](src, dst, new(models.UserGroup).TableName())
}

func migrateGroup2Files(src, dst *gorm.DB) error {
	return migrateRows[models.Group2File](src, dst, new(models.Group2File).TableName())
}

func migrateCloudTokens(src, dst *gorm.DB) error {
	return migrateRows[models.CloudToken](src, dst, new(models.CloudToken).TableName())
}

func migrateAutoIngestPlans(src, dst *gorm.DB) error {
	return migrateRows[models.AutoIngestPlan](src, dst, new(models.AutoIngestPlan).TableName())
}

func migrateAutoIngestLogs(src, dst *gorm.DB) error {
	return migrateRows[models.AutoIngestLog](src, dst, new(models.AutoIngestLog).TableName())
}

func migrateSettings(src, dst *gorm.DB, userCount int64) error {
	var settings []models.Setting
	if err := findSourceRows(src, &settings, new(models.Setting).TableName()); err != nil {
		return err
	}

	if len(settings) == 0 {
		return nil
	}

	for i := range settings {
		if userCount > 0 {
			settings[i].Initialized = true
		}
	}

	if err := upsertRows(dst, settings); err != nil {
		return err
	}

	return resetPostgresSequence(dst, new(models.Setting).TableName())
}

func migrateMediaFiles(src, dst *gorm.DB) error {
	return migrateRows[models.MediaFile](src, dst, new(models.MediaFile).TableName())
}

func migrateMediaConfig(src, dst *gorm.DB) error {
	return migrateRows[models.MediaConfig](src, dst, new(models.MediaConfig).TableName())
}

func migrateLoginLogs(src, dst *gorm.DB) error {
	return migrateRows[models.LoginLog](src, dst, new(models.LoginLog).TableName())
}

func migrateFileTaskLogs(src, dst *gorm.DB) error {
	return migrateRows[models.FileTaskLog](src, dst, new(models.FileTaskLog).TableName())
}

func migrateMountPoints(src, dst *gorm.DB) error {
	return migrateRows[models.MountPoint](src, dst, new(models.MountPoint).TableName())
}

func migrateUserMountPointTokens(src, dst *gorm.DB) error {
	var rows []models.UserMountPointToken
	if err := findSourceRows(src, &rows, new(models.UserMountPointToken).TableName()); err != nil {
		return err
	}

	rows = dedupeUserMountPointTokenRows(rows)
	if len(rows) == 0 {
		return nil
	}

	if err := upsertUserMountPointTokenRows(dst, rows); err != nil {
		return err
	}

	return resetPostgresSequence(dst, new(models.UserMountPointToken).TableName())
}

func upsertUserMountPointTokenRows(dst *gorm.DB, rows []models.UserMountPointToken) error {
	for i := range rows {
		row := rows[i]

		var existing models.UserMountPointToken

		err := dst.
			Where("user_id = ? AND mount_point_id = ?", row.UserID, row.MountPointID).
			Take(&existing).Error
		if err == nil {
			if err := dst.Model(new(models.UserMountPointToken)).
				Where("id = ?", existing.ID).
				Updates(map[string]any{
					"token_id":   row.TokenID,
					"updated_at": row.UpdatedAt,
				}).Error; err != nil {
				return err
			}

			continue
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := dst.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
			return err
		}
	}

	return nil
}

func dedupeUserMountPointTokenRows(rows []models.UserMountPointToken) []models.UserMountPointToken {
	if len(rows) <= 1 {
		return rows
	}

	type key struct {
		userID       int64
		mountPointID int64
	}

	kept := make(map[key]models.UserMountPointToken, len(rows))
	order := make([]key, 0, len(rows))

	for _, row := range rows {
		k := key{userID: row.UserID, mountPointID: row.MountPointID}

		current, ok := kept[k]
		if !ok {
			order = append(order, k)
			kept[k] = row

			continue
		}

		if row.ID > current.ID {
			kept[k] = row
		}
	}

	result := make([]models.UserMountPointToken, 0, len(kept))
	for _, k := range order {
		result = append(result, kept[k])
	}

	return result
}

func migrateTelegramSettings(src, dst *gorm.DB) error {
	return migrateRows[models.TelegramSetting](src, dst, new(models.TelegramSetting).TableName())
}

func migrateTelegramUsers(src, dst *gorm.DB) error {
	return migrateRows[models.TelegramUser](src, dst, new(models.TelegramUser).TableName())
}

func migrateSubscriptions(src, dst *gorm.DB) error {
	return migrateRows[models.Subscription](src, dst, new(models.Subscription).TableName())
}

func migrateMatchHistory(src, dst *gorm.DB) error {
	return migrateRows[models.MatchHistory](src, dst, new(models.MatchHistory).TableName())
}

func migrateDailyHotHistory(src, dst *gorm.DB) error {
	return migrateRows[models.DailyHotHistory](src, dst, new(models.DailyHotHistory).TableName())
}

func migrateSystemSettings(src, dst *gorm.DB) error {
	return migrateRows[SystemSetting](src, dst, new(SystemSetting).TableName())
}

func migrateVirtualFiles(src, dst *gorm.DB) error {
	var files []models.VirtualFile
	if err := findSourceRows(src, &files, new(models.VirtualFile).TableName()); err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	for i := 0; i < len(files); i += dataMigrationBatchSize {
		end := i + dataMigrationBatchSize
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]
		if err := upsertRows(dst, batch); err != nil {
			return fmt.Errorf("批量迁移虚拟文件失败: %w", err)
		}
	}

	return resetPostgresSequence(dst, new(models.VirtualFile).TableName())
}

func runMigrationStep(name string, run func() error) error {
	if err := run(); err != nil {
		if errors.Is(err, errMissingSourceTable) {
			fmt.Printf("%s数据表不存在，跳过\n", name)

			return nil
		}

		return fmt.Errorf("迁移%s数据失败: %w", name, err)
	}

	fmt.Printf("%s数据迁移完成\n", name)

	return nil
}

func migrateRows[T any](src, dst *gorm.DB, tableName string) error {
	var rows []T
	if err := findSourceRows(src, &rows, tableName); err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	if err := upsertRows(dst, rows); err != nil {
		return err
	}

	return resetPostgresSequence(dst, tableName)
}

func findSourceRows[T any](src *gorm.DB, rows *[]T, tableName string) error {
	if !src.Migrator().HasTable(tableName) {
		return errMissingSourceTable
	}

	if err := src.Table(tableName).Find(rows).Error; err != nil {
		return err
	}

	return nil
}

func upsertRows[T any](dst *gorm.DB, rows []T) error {
	return dst.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(&rows, dataMigrationBatchSize).Error
}

func resetPostgresSequence(db *gorm.DB, tableName string) error {
	dialector := db.Dialector
	if dialector == nil || dialector.Name() != "postgres" {
		return nil
	}

	var sequence sql.NullString
	if err := db.Raw("SELECT pg_get_serial_sequence(?, 'id')", tableName).Scan(&sequence).Error; err != nil {
		return fmt.Errorf("failed to find PostgreSQL sequence for %s: %w", tableName, err)
	}

	if !sequence.Valid || sequence.String == "" {
		return nil
	}

	query := fmt.Sprintf(
		"SELECT setval(?::regclass, COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)",
		quotePostgresIdentifier(tableName),
	)
	if err := db.Exec(query, sequence.String).Error; err != nil {
		return fmt.Errorf("failed to reset PostgreSQL sequence for %s: %w", tableName, err)
	}

	return nil
}

func closeGormDB(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}

	_ = sqlDB.Close()
}

func quotePostgresIdentifier(identifier string) string {
	parts := strings.Split(identifier, ".")
	for i, part := range parts {
		parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}

	return strings.Join(parts, ".")
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
	defer closeGormDB(sqliteDB)

	var settingCount int64
	if err := sqliteDB.Model(&models.Setting{}).Count(&settingCount).Error; err != nil {
		return false
	}

	if settingCount == 0 {
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
	defer closeGormDB(pgDB)

	var pgUserCount int64
	if err := pgDB.Model(&models.User{}).Count(&pgUserCount).Error; err != nil {
		return false
	}

	return pgUserCount == 0
}
