package config

import (
	"flag"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/utils/envname"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type RedisConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Storage  StorageConfig  `mapstructure:"storage"`
	AI       AIConfig       `mapstructure:"ai"`
	Search   SearchConfig   `mapstructure:"search"`
	Text2SQL Text2SQLConfig `mapstructure:"text2sql"`
	Security SecurityConfig `mapstructure:"security"`
}

// Text2SQLConfig Text2SQL 独立配置段
// 所有参数均有默认值，旧配置文件不配置 text2sql 段时使用默认值
type Text2SQLConfig struct {
	// 生成策略
	MaxRetries                  int  `mapstructure:"max_retries"`                    // 主生成循环重试次数（默认 3）
	NumCandidates               int  `mapstructure:"num_candidates"`                 // 多候选生成数量（默认 3）
	EnableMultiCandidate        bool `mapstructure:"enable_multi_candidate"`         // 是否启用多候选（默认 true）
	EnableParallelCandidate     bool `mapstructure:"enable_parallel_candidate"`      // 是否并行生成候选（默认 true）
	EnableMultiHop              bool `mapstructure:"enable_multi_hop"`               // 是否启用多跳推理（默认 true）
	MultiHopMaxHops             int  `mapstructure:"multi_hop_max_hops"`             // 多跳最大跳数（默认 3）
	MultiHopComplexityThreshold int  `mapstructure:"multi_hop_complexity_threshold"` // 触发多跳的复杂度阈值（默认 50）

	// 模型参数
	TemperatureGeneration float64 `mapstructure:"temperature_generation"` // 生成温度（默认 0.1）
	TemperatureCandidate  float64 `mapstructure:"temperature_candidate"`  // 多候选温度（默认 0.3）
	TemperatureMultiHop   float64 `mapstructure:"temperature_multi_hop"`  // 多跳温度（默认 0.2）
	MaxTokens             int     `mapstructure:"max_tokens"`             // 单次生成最大 token（默认 1000）

	// Schema Linking
	SchemaTopKTables      int     `mapstructure:"schema_top_k_tables"`     // 表级向量检索 TopK（默认 20）
	SchemaTopKColumns     int     `mapstructure:"schema_top_k_columns"`    // 列级向量检索 TopK（默认 40）
	SchemaMaxTables       int     `mapstructure:"schema_max_tables"`       // Schema 上下文最大表数（默认 15）
	SchemaMaxChars        int     `mapstructure:"schema_max_chars"`        // Schema 上下文最大字符数（默认 45000）
	FKConfidenceThreshold float64 `mapstructure:"fk_confidence_threshold"` // FK 发现置信度阈值（默认 0.6）

	// Few-shot
	FewShotCount       int `mapstructure:"few_shot_count"`         // Few-shot 示例数量（默认 3）
	FewShotSQLMaxChars int `mapstructure:"few_shot_sql_max_chars"` // 示例 SQL 截断长度（默认 500）

	// 执行
	ExecutionMaxRows     int `mapstructure:"execution_max_rows"`     // 执行结果最大行数（默认 200）
	ExecutionTimeoutSec  int `mapstructure:"execution_timeout_sec"`  // 执行超时秒数（默认 15）
	ExecutionWarningRows int `mapstructure:"execution_warning_rows"` // Warning 行数阈值（默认 10000）

	// 反馈循环
	EnableExecutionFeedbackLoop bool `mapstructure:"enable_execution_feedback_loop"` // 启用执行反馈循环（默认 true）
	FeedbackMaxRetry            int  `mapstructure:"feedback_max_retry"`             // 反馈修正最大重试（默认 2）

	// P1: 执行计划分析（EXPLAIN 评分）
	EnableExecutionPlanAnalysis bool `mapstructure:"enable_execution_plan_analysis"` // 启用执行计划分析（默认 false，需数据源支持 EXPLAIN）

	// P1: 意图-结果一致性校验
	EnableIntentCheck bool `mapstructure:"enable_intent_check"` // 启用意图校验（默认 false，需消耗 LLM 调用）

	// P1: LLM Refiner（AST 修正失败时用 LLM 修正）
	EnableLLMRefiner bool `mapstructure:"enable_llm_refiner"` // 启用 LLM 修正（默认 false，需消耗 LLM 调用）

	// Trace
	EnableTrace      bool `mapstructure:"enable_trace"`       // 是否启用 Trace（默认 true）
	EnableTraceSteps bool `mapstructure:"enable_trace_steps"` // 是否记录中间步骤（默认 true）
}

// DefaultText2SQLConfig 返回 Text2SQL 默认配置
func DefaultText2SQLConfig() Text2SQLConfig {
	return Text2SQLConfig{
		MaxRetries:                  3,
		NumCandidates:               3,
		EnableMultiCandidate:        true,
		EnableParallelCandidate:     true,
		EnableMultiHop:              true,
		MultiHopMaxHops:             3,
		MultiHopComplexityThreshold: 50,
		TemperatureGeneration:       0.1,
		TemperatureCandidate:        0.3,
		TemperatureMultiHop:         0.2,
		MaxTokens:                   1000,
		SchemaTopKTables:            20,
		SchemaTopKColumns:           40,
		SchemaMaxTables:             15,
		SchemaMaxChars:              45000,
		FKConfidenceThreshold:       0.6,
		FewShotCount:                3,
		FewShotSQLMaxChars:          500,
		ExecutionMaxRows:            200,
		ExecutionTimeoutSec:         15,
		ExecutionWarningRows:        10000,
		EnableExecutionFeedbackLoop: true,
		FeedbackMaxRetry:            2,
		EnableExecutionPlanAnalysis: false,
		EnableIntentCheck:           false,
		EnableLLMRefiner:            false,
		EnableTrace:                 true,
		EnableTraceSteps:            true,
	}
}

type ServerConfig struct {
	Host           string   `mapstructure:"host"`
	Port           int      `mapstructure:"port"`
	Mode           string   `mapstructure:"mode"`
	AllowedOrigins []string `mapstructure:"allowed_origins"` // CORS 允许的域名列表
	// TrustedProxies 可信反向代理网段（CIDR 或 IP）。留空表示不信任任何代理头，
	// c.ClientIP() 只取真实连接 IP；否则 X-Forwarded-For 可被伪造，
	// 使 IP 白名单与按来源 IP 的登录锁定同时失效。
	TrustedProxies []string `mapstructure:"trusted_proxies"`
	// SwaggerOpen 显式开启 /swagger 接口文档。文档会完整枚举 API 面且无需鉴权，
	// 因此不能只靠 mode != release 就暴露，必须显式打开（或用 AIWIKI_SWAGGER=1）。
	SwaggerOpen bool `mapstructure:"swagger_open"`
}

type DatabaseConfig struct {
	Driver          string `mapstructure:"driver"`
	DSN             string `mapstructure:"dsn"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`  // 连接最大存活时间（秒）
	ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time"` // 连接最大空闲时间（秒）
}

type StorageConfig struct {
	RootDir       string `mapstructure:"root_dir"`
	IndexDir      string `mapstructure:"index_dir"`
	DocumentDir   string `mapstructure:"document_dir"`
	ImageDir      string `mapstructure:"image_dir"`
	AttachmentDir string `mapstructure:"attachment_dir"`
	// ContentSource 正文来源：file（默认，读 data/markdowns）/ db（读 aw_document_content）。
	// 切回 file 即可回滚，磁盘文件在观察期内不删除。
	ContentSource string `mapstructure:"content_source"`
	// AttachmentSource 附件二进制来源：file（默认，读 data/attachment|images）/ db（读 aw_attachment_chunk）。
	// storage_kind=2 的历史记录在任何模式下都回退读磁盘。
	AttachmentSource string `mapstructure:"attachment_source"`
}

type AIProviderConfig struct {
	Provider    string  `mapstructure:"provider"`
	BaseURL     string  `mapstructure:"base_url"`
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

type AIEmbeddingConfig struct {
	Provider string `mapstructure:"provider"`
	BaseURL  string `mapstructure:"base_url"`
	APIKey   string `mapstructure:"api_key"`
	Model    string `mapstructure:"model"`
}

type AIMCPServerConfig struct {
	Name      string            `mapstructure:"name" json:"name"`
	Transport string            `mapstructure:"transport" json:"transport"`
	Command   string            `mapstructure:"command" json:"command"`
	Args      []string          `mapstructure:"args" json:"args"`
	Env       map[string]string `mapstructure:"env" json:"env"`
	Endpoint  string            `mapstructure:"endpoint" json:"endpoint"`
}

type AIMCPConfig struct {
	Enabled bool                `mapstructure:"enabled"`
	Servers []AIMCPServerConfig `mapstructure:"servers"`
}

type AIConfig struct {
	Enabled           bool    `mapstructure:"enabled"`
	Provider          string  `mapstructure:"provider"`
	BaseURL           string  `mapstructure:"base_url"`
	APIKey            string  `mapstructure:"api_key"`
	Model             string  `mapstructure:"model"`
	MaxTokens         int     `mapstructure:"max_tokens"`
	Temperature       float64 `mapstructure:"temperature"`
	EmbeddingURL      string  `mapstructure:"embedding_url"`
	EmbeddingKey      string  `mapstructure:"embedding_key"`
	EmbeddingModel    string  `mapstructure:"embedding_model"`
	DailyTokenLimit   int     `mapstructure:"daily_token_limit"`
	PerUserTokenLimit int     `mapstructure:"per_user_token_limit"`

	Primary    *AIProviderConfig   `mapstructure:"primary"`
	Fast       *AIProviderConfig   `mapstructure:"fast"`
	Reasoning  *AIProviderConfig   `mapstructure:"reasoning"`
	Embedding  *AIEmbeddingConfig  `mapstructure:"embedding"`
	MCPServers []AIMCPServerConfig `mapstructure:"mcp_servers"` // 兼容旧格式
	MCP        *AIMCPConfig        `mapstructure:"mcp"`

	VectorWeight        float64 `mapstructure:"vector_weight"`
	KeywordWeight       float64 `mapstructure:"keyword_weight"`
	SearchTopK          int     `mapstructure:"search_top_k"`
	SimilarityThreshold float64 `mapstructure:"similarity_threshold"`
	RerankEnabled       bool    `mapstructure:"rerank_enabled"`
	RerankEndpoint      string  `mapstructure:"rerank_endpoint"`
	RerankApiKey        string  `mapstructure:"rerank_api_key"`
	RerankModel         string  `mapstructure:"rerank_model"`
	VectorStoreType     string  `mapstructure:"vector_store_type"`
	VectorMinScore      float64 `mapstructure:"vector_min_score"` // HNSW cosine-distance threshold (score > threshold means similarity > 1-threshold)
	ChromaEndpoint      string  `mapstructure:"chroma_endpoint"`
	QdrantEndpoint      string  `mapstructure:"qdrant_endpoint"`
	QdrantApiKey        string  `mapstructure:"qdrant_api_key"`
	MaxContextChars     int     `mapstructure:"max_context_chars"`

	// P4 图扩展：主问答链路是否叠加 wiki 图谱 2-hop 扩展上下文（默认关）
	GraphExpansion bool `mapstructure:"graph_expansion"`

	// P5 MinerU 可选 PDF 解析后端（复杂版面保留结构）；空 = 不启用
	MinerUEndpoint string `mapstructure:"mineru_endpoint"`

	// P6 OTLP/HTTP 导出端点（如 http://localhost:4318/v1/traces）；空 = 不启用
	OtlpEndpoint string `mapstructure:"otlp_endpoint"`

	MinMaturity string `mapstructure:"min_maturity"`

	AgentTools AgentToolsConfig `mapstructure:"agent_tools"`
}

// AgentToolsConfig 定义 Agent 工具的能力边界。
//
// 高危工具的输入直接来自模型输出：一旦开放，等价于把服务器 shell、任意文件读写与
// 业务库全表查询交给「能发起对话的任何人」（或被检索文档里的 prompt 注入）。
// 因此这四个工具默认全部关闭，需要用的时候显式打开并配置白名单。
type AgentToolsConfig struct {
	// EnableCodeExecutor 允许 execute_code（Python/Node 裸执行）
	EnableCodeExecutor bool `mapstructure:"enable_code_executor"`
	// EnableSystemCommand 允许 execute_command（shell 命令）
	EnableSystemCommand bool `mapstructure:"enable_system_command"`
	// EnableFileOperation 允许 file_operation（服务器文件读写）
	EnableFileOperation bool `mapstructure:"enable_file_operation"`
	// EnableDatabaseQuery 允许 query_database（对业务库执行模型自撰 SQL）
	EnableDatabaseQuery bool `mapstructure:"enable_database_query"`

	// AllowedCommands 允许 execute_command 使用的命令白名单；为空表示使用内置最小集合
	AllowedCommands []string `mapstructure:"allowed_commands"`
	// AllowedDirs 允许 file_operation 读写的目录前缀；为空时该工具拒绝一切路径（fail-closed）
	AllowedDirs []string `mapstructure:"allowed_dirs"`
	// AllowedDomains 允许 api_call / fetch_web 访问的域名白名单；为空表示仅允许公开 HTTPS 站点且禁止内网地址
	AllowedDomains []string `mapstructure:"allowed_domains"`
	// MaxResultRows query_database 单次返回行数硬上限（<=0 使用默认 500）
	MaxResultRows int `mapstructure:"max_result_rows"`
	// TimeoutSeconds 工具执行超时（<=0 使用默认 30 秒）
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
}

type SynonymGroup struct {
	Words []string `mapstructure:"words" json:"words"`
}

type SearchConfig struct {
	Synonyms []SynonymGroup `mapstructure:"synonyms"`
}

// SecurityConfig 安全加固配置（P1-G115 IP 白名单、P1-G031 静态字段加密）
type SecurityConfig struct {
	// IP 白名单（G115）
	IPWhitelistEnabled bool     `mapstructure:"ip_whitelist_enabled"` // 是否启用，默认 false
	IPWhitelist        []string `mapstructure:"ip_whitelist"`         // 允许的 IP 或 CIDR 列表
	IPWhitelistMode    string   `mapstructure:"ip_whitelist_mode"`    // allow（默认，仅放行列表内）| deny（封锁列表内）
	// 静态字段加密（G031，AES-GCM）
	EncryptKey string `mapstructure:"encrypt_key"` // 32 字节密钥（原始/hex），环境变量 CITEBASE_ENCRYPT_KEY 优先（改名前的旧名兜底）
}

var AppConfig Config

// InitConfig 初始化配置
// 查找顺序：
//  1. 命令行参数 -config 指定的文件
//  2. <exe_dir>/configs/config.yaml（发布目录）
//  3. <exe_dir>/../configs/config.yaml（开发目录 bin/../configs/）
//  4. ./configs/config.yaml（当前工作目录）
func InitConfig() {
	// 解析命令行参数
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	var configPaths []string

	// 如果指定了配置文件路径，优先使用
	if *configPath != "" {
		configPaths = append(configPaths, *configPath)
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)

		// 1. exe 同级 configs/
		configPaths = append(configPaths, filepath.Join(exeDir, "configs", "config.yaml"))

		// 2. exe 上级目录 configs/（开发场景：bin/server.exe -> configs/）
		configPaths = append(configPaths, filepath.Join(exeDir, "..", "configs", "config.yaml"))
	}

	// 3. 当前工作目录
	cwd, _ := os.Getwd()
	configPaths = append(configPaths, filepath.Join(cwd, "configs", "config.yaml"))
	configPaths = append(configPaths, "configs/config.yaml")

	var lastErr error
	for _, p := range configPaths {
		viper.SetConfigFile(p)
		if err := viper.ReadInConfig(); err == nil {
			logx.Infof("Config loaded from: %s", p)
			break
		}
		lastErr = err
		// 尝试下一个路径
	}

	if !viper.IsSet("server") && lastErr != nil {
		logx.Fatalf("Error reading config file (tried: %v): %v", configPaths, lastErr)
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		logx.Fatalf("Unable to decode into struct: %v", err)
	}

	// Text2SQL 配置应用默认值（旧配置文件无 text2sql 段时使用默认值）
	applyText2SQLDefaults()

	if dsnEnv := envname.Getenv("CITEBASE_DB_DSN", "AIWIKI_DB_DSN"); dsnEnv != "" {
		AppConfig.Database.DSN = dsnEnv
	}

	// AI 配置环境变量覆盖
	if aiEnabled := os.Getenv("AI_ENABLED"); aiEnabled != "" {
		if AppConfig.AI.Enabled {
			logx.Info("[Config] WARNING: AI_ENABLED env overrides config file value")
		}
		AppConfig.AI.Enabled = aiEnabled == "true" || aiEnabled == "1"
	}
	if aiProvider := os.Getenv("AI_PROVIDER"); aiProvider != "" {
		if AppConfig.AI.Provider != "" {
			logx.Infof("[Config] WARNING: AI_PROVIDER env (%s) overrides config file (%s)", aiProvider, AppConfig.AI.Provider)
		}
		AppConfig.AI.Provider = aiProvider
	}
	if aiBaseURL := os.Getenv("AI_BASE_URL"); aiBaseURL != "" {
		if AppConfig.AI.BaseURL != "" {
			logx.Infof("[Config] WARNING: AI_BASE_URL env overrides config file (%s)", AppConfig.AI.BaseURL)
		}
		AppConfig.AI.BaseURL = aiBaseURL
	}
	if aiAPIKey := os.Getenv("AI_API_KEY"); aiAPIKey != "" {
		if AppConfig.AI.APIKey != "" {
			logx.Info("[Config] WARNING: AI_API_KEY env overrides config file value")
		}
		AppConfig.AI.APIKey = aiAPIKey
	}
	if aiModel := os.Getenv("AI_MODEL"); aiModel != "" {
		if AppConfig.AI.Model != "" {
			logx.Infof("[Config] WARNING: AI_MODEL env (%s) overrides config file (%s)", aiModel, AppConfig.AI.Model)
		}
		AppConfig.AI.Model = aiModel
	}

	applyAIDefaults()

	// 安全段默认值（G115/G031）
	if AppConfig.Security.IPWhitelistMode == "" {
		AppConfig.Security.IPWhitelistMode = "allow"
	}
	if ek := envname.Getenv("CITEBASE_ENCRYPT_KEY", "AIWIKI_ENCRYPT_KEY"); ek != "" {
		AppConfig.Security.EncryptKey = ek
	}

	logx.Info("Config loaded successfully")
}

func applyAIDefaults() {
	if AppConfig.AI.VectorWeight == 0 {
		AppConfig.AI.VectorWeight = 0.7
	}
	if AppConfig.AI.KeywordWeight == 0 {
		AppConfig.AI.KeywordWeight = 0.3
	}
	if AppConfig.AI.SearchTopK == 0 {
		AppConfig.AI.SearchTopK = 20
	}
	if AppConfig.AI.SimilarityThreshold == 0 {
		AppConfig.AI.SimilarityThreshold = 0.3
	}
	if AppConfig.AI.VectorStoreType == "" {
		AppConfig.AI.VectorStoreType = "bolt"
	}
	if AppConfig.AI.VectorMinScore == 0 {
		AppConfig.AI.VectorMinScore = 0.3 // cosine distance; similarity > 0.7
	}
	if AppConfig.AI.ChromaEndpoint == "" {
		AppConfig.AI.ChromaEndpoint = "http://localhost:8000"
	}
	if AppConfig.AI.MaxContextChars <= 0 {
		AppConfig.AI.MaxContextChars = 6000
	}
}

// applyText2SQLDefaults 为 Text2SQL 配置应用默认值
// 逐字段检查，仅当字段为零值时应用默认值，允许用户部分覆盖
func applyText2SQLDefaults() {
	d := DefaultText2SQLConfig()
	c := &AppConfig.Text2SQL

	if c.MaxRetries == 0 {
		c.MaxRetries = d.MaxRetries
	}
	if c.NumCandidates == 0 {
		c.NumCandidates = d.NumCandidates
	}
	// bool 字段无法区分"未设置"和"设置为 false"，使用 EnableMultiCandidate == false 且 NumCandidates == 0 的组合判断
	// 简化处理：如果用户配置了 NumCandidates > 0 但没启用 MultiCandidate，视为有意禁用
	// 对于 bool 字段，默认 true，仅当配置文件显式设置为 false 时才禁用（viper 会正确解析）
	// 这里不做强制覆盖，保留用户的显式 false

	if c.MultiHopMaxHops == 0 {
		c.MultiHopMaxHops = d.MultiHopMaxHops
	}
	if c.MultiHopComplexityThreshold == 0 {
		c.MultiHopComplexityThreshold = d.MultiHopComplexityThreshold
	}
	if c.TemperatureGeneration == 0 {
		c.TemperatureGeneration = d.TemperatureGeneration
	}
	if c.TemperatureCandidate == 0 {
		c.TemperatureCandidate = d.TemperatureCandidate
	}
	if c.TemperatureMultiHop == 0 {
		c.TemperatureMultiHop = d.TemperatureMultiHop
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = d.MaxTokens
	}
	if c.SchemaTopKTables == 0 {
		c.SchemaTopKTables = d.SchemaTopKTables
	}
	if c.SchemaTopKColumns == 0 {
		c.SchemaTopKColumns = d.SchemaTopKColumns
	}
	if c.SchemaMaxTables == 0 {
		c.SchemaMaxTables = d.SchemaMaxTables
	}
	if c.SchemaMaxChars == 0 {
		c.SchemaMaxChars = d.SchemaMaxChars
	}
	if c.FKConfidenceThreshold == 0 {
		c.FKConfidenceThreshold = d.FKConfidenceThreshold
	}
	if c.FewShotCount == 0 {
		c.FewShotCount = d.FewShotCount
	}
	if c.FewShotSQLMaxChars == 0 {
		c.FewShotSQLMaxChars = d.FewShotSQLMaxChars
	}
	if c.ExecutionMaxRows == 0 {
		c.ExecutionMaxRows = d.ExecutionMaxRows
	}
	if c.ExecutionTimeoutSec == 0 {
		c.ExecutionTimeoutSec = d.ExecutionTimeoutSec
	}
	if c.ExecutionWarningRows == 0 {
		c.ExecutionWarningRows = d.ExecutionWarningRows
	}
	if c.FeedbackMaxRetry == 0 {
		c.FeedbackMaxRetry = d.FeedbackMaxRetry
	}
}
