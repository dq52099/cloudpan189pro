package configs

import (
	"flag"
	"sync"
	"testing"

	"github.com/joho/godotenv"
	"github.com/xxcheng123/cloudpan189-share/internal/consts"

	"github.com/zeromicro/go-zero/core/conf"
)

var (
	BuildDate  string
	Commit     string
	GitBranch  string
	GitSummary string
	Version    string
)

var c = new(Config)

var configPath string

func init() {
	envFiles := []string{".env", ".env.local", ".env.example"}
	for _, envFile := range envFiles {
		_ = godotenv.Load(envFile)
	}

	if testing.Testing() {
		return
	}

	flag.StringVar(&configPath, "config", "etc/config.yaml", "config path")
	flag.Parse()
}

var once sync.Once

func Get() *RuntimeConfig {
	once.Do(func() {
		conf.MustLoad(configPath, c, conf.UseEnv())
		applyDefaults(c)
	})

	return &RuntimeConfig{
		Config:     c,
		BuildDate:  BuildDate,
		Commit:     Commit,
		GitBranch:  GitBranch,
		GitSummary: GitSummary,
		Version:    Version,
	}
}

// applyDefaults 针对无法通过 struct tag 表达的默认值进行补齐。
func applyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	if cfg.Subscription != nil && cfg.Subscription.CronExpression == "" {
		cfg.Subscription.CronExpression = consts.DefaultSubscriptionCronExpression
	}
}
