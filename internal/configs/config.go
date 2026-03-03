package configs

type Config struct {
	Port       int               `json:"port,default=12395,env=PORT"`
	DBFile     string            `json:"dbFile,default=data/data.db,env=DBFILE"`
	LogFile    string            `json:"logFile,default=logs/share.log,env=LOGFILE"`
	MediaDir   string            `json:"mediaDir,default=media_dir,env=MEDIADIR"`
	DBType     string            `json:"dbType,default=sqlite,env=DBTYPE"`
	MySQL      *MySQLConfig      `json:"mysql,omitempty"`
	Postgres   *PostgresConfig   `json:"postgres,omitempty"`
	TaskEngine *TaskEngineConfig `json:"taskEngine,omitempty"`
}

type MySQLConfig struct {
	Host   string `json:"host,default=127.0.0.1,env=MYSQL_HOST"`
	Port   int    `json:"port,default=3306,env=MYSQL_PORT"`
	User   string `json:"user,default=root,env=MYSQL_USER"`
	Pass   string `json:"pass,default=root,env=MYSQL_PASS"`
	DBName string `json:"dbName,default=share,env=MYSQL_DBNAME"`
}

type PostgresConfig struct {
	Host    string `json:"host,default=192.168.31.51,env=POSTGRES_HOST"`
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

type RuntimeConfig struct {
	*Config
	BuildDate  string
	Commit     string
	GitBranch  string
	GitSummary string
	Version    string
}
