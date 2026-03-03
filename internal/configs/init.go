package configs

import (
	"flag"
	"sync"
	"testing"

	"github.com/joho/godotenv"

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
