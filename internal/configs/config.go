package configs

type Config struct {
	Port         int                 `json:"port,default=12395,env=PORT"`
	DBFile       string              `json:"dbFile,default=data/data.db,env=DBFILE"`
	LogFile      string              `json:"logFile,default=logs/share.log,env=LOGFILE"`
	MediaDir     string              `json:"mediaDir,default=media_dir,env=MEDIADIR"`
	DBType       string              `json:"dbType,default=sqlite,env=DBTYPE"`
	MySQL        *MySQLConfig        `json:"mysql,omitempty"`
	Postgres     *PostgresConfig     `json:"postgres,omitempty"`
	TaskEngine   *TaskEngineConfig   `json:"taskEngine,omitempty"`
	Telegram     *TelegramConfig     `json:"telegram,omitempty"`
	OpenAI       *OpenAIConfig       `json:"openai,omitempty"`
	Subscription *SubscriptionConfig `json:"subscription,omitempty"`
}

type MySQLConfig struct {
	Host   string `json:"host,default=127.0.0.1,env=MYSQL_HOST"`
	Port   int    `json:"port,default=3306,env=MYSQL_PORT"`
	User   string `json:"user,default=root,env=MYSQL_USER"`
	Pass   string `json:"pass,default=root,env=MYSQL_PASS"`
	DBName string `json:"dbName,default=share,env=MYSQL_DBNAME"`
}

type PostgresConfig struct {
	Host    string `json:"host,default=127.0.0.1,env=POSTGRES_HOST"`
	Port    int    `json:"port,default=5432,env=POSTGRES_PORT"`
	User    string `json:"user,default=postgres,env=POSTGRES_USER"`
	Pass    string `json:"pass,default=postgres,env=POSTGRES_PASS"`
	DBName  string `json:"dbName,default=share,env=POSTGRES_DBNAME"`
	SSLMode string `json:"sslMode,default=disable,env=POSTGRES_SSLMODE"`
}

type TaskEngineConfig struct {
	WorkerCount    int `json:"workerCount,default=4,env=TASKENGINE_WORKERCOUNT"`
	BufferSize     int `json:"bufferSize,default=89120,env=TASKENGINE_BUFFERSIZE"`
	ProcessTimeout int `json:"processTimeout,default=1800,env=TASKENGINE_PROCESSTIMEOUT"`
	MaxRetry       int `json:"maxRetry,default=3,env=TASKENGINE_MAXRETRY"`
}

type TelegramConfig struct {
	BotToken         string `json:"botToken,default='',env=TG_BOT_TOKEN"`
	ProxyURL         string `json:"proxyURL,default='',env=TG_PROXY"`
	ProxyType        string `json:"proxyType,default='',env=TG_PROXY_TYPE"` // http, https, socks5
	ChatID           string `json:"chatID,default='',env=TG_CHAT_ID"`
	DefaultMountPath string `json:"defaultMountPath,default=/转存,env=TG_DEFAULT_MOUNT_PATH"`
	APIURL           string `json:"apiURL,default=https://api.telegram.org,env=TG_API_URL"`
}

type OpenAIConfig struct {
	APIKey  string `json:"apiKey,default='',env=OPENAI_API_KEY"`
	BaseURL string `json:"baseURL,default=https://api.openai.com,env=OPENAI_BASE_URL"`
	Model   string `json:"model,default=gpt-4o-mini,env=OPENAI_MODEL"`
}

type SubscriptionConfig struct {
	Enabled bool `json:"enabled,default=false,env=SUBSCRIPTION_ENABLED"`
	// CronExpression 的默认值通过代码设置，避免 struct tag 中的空格触发 go vet 警告。
	CronExpression string `json:"cronExpression,optional,env=SUBSCRIPTION_CRON"`
	PanSearchURL   string `json:"panSearchURL,default=https://tg.252035.xyz,env=PAN_SEARCH_URL"`
	EnableTMDB     bool   `json:"enableTMDB,default=true,env=SUBSCRIPTION_ENABLE_TMDB"`
	EnableDouban   bool   `json:"enableDouban,default=true,env=SUBSCRIPTION_ENABLE_DOUBAN"`
}

type RuntimeConfig struct {
	*Config
	BuildDate  string
	Commit     string
	GitBranch  string
	GitSummary string
	Version    string
}
