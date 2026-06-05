package bootstrap

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xxcheng123/cloudpan189-share/internal/configs"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	"github.com/xxcheng123/cloudpan189-share/internal/services/subscription"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestResolveSubscriptionExtensionConfigUsesSharedDefault(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	t.Cleanup(func() {
		closeGormDB(db)
	})

	config := resolveSubscriptionExtensionConfig(db, zap.NewNop(), nil)

	if config.PanSearchURL != subscription.DefaultPanSearchURL {
		t.Fatalf("expected default pan search URL %q, got %q", subscription.DefaultPanSearchURL, config.PanSearchURL)
	}

	if !config.EnableTMDB || !config.EnableDouban {
		t.Fatalf("expected default subscription providers enabled, got %+v", config)
	}

	if config.Enabled {
		t.Fatalf("expected default subscription scheduler disabled, got %+v", config)
	}

	if config.CronExpression != "0 2 * * *" {
		t.Fatalf("expected default cron expression, got %q", config.CronExpression)
	}
}

func TestResolveSubscriptionExtensionConfigPrecedence(t *testing.T) {
	t.Setenv("PAN_SEARCH_URL", "https://env.example.com/api/search")
	t.Setenv("SUBSCRIPTION_ENABLED", "true")
	t.Setenv("SUBSCRIPTION_ENABLE_TMDB", "true")
	t.Setenv("SUBSCRIPTION_ENABLE_DOUBAN", "false")
	t.Setenv("SUBSCRIPTION_CRON", "0 5 * * *")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	t.Cleanup(func() {
		closeGormDB(db)
	})

	if err := db.AutoMigrate(&SystemSetting{}); err != nil {
		t.Fatalf("migrate system setting schema: %v", err)
	}

	if err := db.Create(&SystemSetting{
		Name: "subscription_config",
		Value: models.SubscriptionConfig{
			Enabled:        false,
			PanSearchURL:   "https://db.example.com/api/search",
			CronExpression: "0 4 * * *",
			EnableTMDB:     false,
			EnableDouban:   true,
		},
	}).Error; err != nil {
		t.Fatalf("seed subscription setting: %v", err)
	}

	config := resolveSubscriptionExtensionConfig(db, zap.NewNop(), &configs.Config{
		Subscription: &configs.SubscriptionConfig{
			Enabled:        false,
			PanSearchURL:   "https://cfg.example.com/api/search",
			CronExpression: "0 3 * * *",
			EnableTMDB:     false,
			EnableDouban:   false,
		},
	})

	if config.PanSearchURL != "https://env.example.com/api/search" {
		t.Fatalf("expected env pan search URL to win, got %q", config.PanSearchURL)
	}

	if !config.EnableTMDB || config.EnableDouban {
		t.Fatalf("expected env booleans to win, got %+v", config)
	}

	if !config.Enabled {
		t.Fatalf("expected env enabled flag to win, got %+v", config)
	}

	if config.CronExpression != "0 5 * * *" {
		t.Fatalf("expected env cron to win, got %q", config.CronExpression)
	}
}
