package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"nofx-lite/crypto"
	"nofx-lite/market"
	"os"
	"slices"
	"strings"
	"time"
	"net/url"
	"regexp"

	_ "github.com/lib/pq"
)

// DatabaseInterface 定义了数据库实现需要提供的方法集合
type DatabaseInterface interface {
	SetCryptoService(cs *crypto.CryptoService)
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(userID string) (*User, error)
	GetAllUsers() ([]string, error)
	UpdateUserOTPVerified(userID string, verified bool) error
	GetAIModels(userID string) ([]*AIModelConfig, error)
	UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error
	GetExchanges(userID string) ([]*ExchangeConfig, error)
	UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error
	CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error
	CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error
	CreateTrader(trader *TraderRecord) error
	GetTraders(userID string) ([]*TraderRecord, error)
	UpdateTraderStatus(userID, id string, isRunning bool) error
	UpdateTrader(trader *TraderRecord) error
	UpdateTraderInitialBalance(userID, id string, newBalance float64) error
	UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error
	DeleteTrader(userID, id string) error
	GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error)
	GetSystemConfig(key string) (string, error)
	SetSystemConfig(key, value string) error
	CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GetUserSignalSource(userID string) (*UserSignalSource, error)
	UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GetCustomCoins() []string
	LoadBetaCodesFromFile(filePath string) error
	ValidateBetaCode(code string) (bool, error)
	UseBetaCode(code, userEmail string) error
	GetBetaCodeStats() (total, used int, err error)
	Close() error
}

// Database 配置数据库
type Database struct {
	db            *sql.DB
	cryptoService *crypto.CryptoService
}

// NewDatabase 创建配置数据库
func NewDatabase(dsn string) (*Database, error) {
	if err := ensurePostgresDatabase(dsn); err != nil {
		return nil, err
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	if err := database.initDefaultData(); err != nil {
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	log.Printf("✅ PostgreSQL 数据库连接已建立")
	return database, nil
}

func ensurePostgresDatabase(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return err
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return nil
	}
	nameOk, _ := regexp.MatchString(`^[A-Za-z0-9_]+$`, dbName)
	if !nameOk {
		return fmt.Errorf("invalid database name")
	}
	admin := *u
	admin.Path = "/postgres"
	adminDSN := admin.String()
	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return err
	}
	defer adminDB.Close()
	if err := adminDB.Ping(); err != nil {
		return err
	}
	var exists int
	err = adminDB.QueryRow("SELECT 1 FROM pg_database WHERE datname = $1", dbName).Scan(&exists)
	if err == nil && exists == 1 {
		return nil
	}
	_, err = adminDB.Exec("CREATE DATABASE " + dbName)
	return err
}

// createTables 创建数据库表
func (d *Database) createTables() error {
    // PostgreSQL DDL
    queries := []string{
        `CREATE TABLE IF NOT EXISTS users (
            id TEXT PRIMARY KEY,
            email TEXT UNIQUE NOT NULL,
            password_hash TEXT NOT NULL,
            otp_secret TEXT,
            otp_verified BOOLEAN DEFAULT FALSE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,

        `CREATE TABLE IF NOT EXISTS ai_models (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL DEFAULT 'default',
            name TEXT NOT NULL,
            provider TEXT NOT NULL,
            enabled BOOLEAN DEFAULT FALSE,
            api_key TEXT DEFAULT '',
            custom_api_url TEXT DEFAULT '',
            custom_model_name TEXT DEFAULT '',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
        )`,

        `CREATE TABLE IF NOT EXISTS exchanges (
            id TEXT NOT NULL,
            user_id TEXT NOT NULL DEFAULT 'default',
            name TEXT NOT NULL,
            type TEXT NOT NULL,
            enabled BOOLEAN DEFAULT FALSE,
            api_key TEXT DEFAULT '',
            secret_key TEXT DEFAULT '',
            testnet BOOLEAN DEFAULT FALSE,
            hyperliquid_wallet_addr TEXT DEFAULT '',
            aster_user TEXT DEFAULT '',
            aster_signer TEXT DEFAULT '',
            aster_private_key TEXT DEFAULT '',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (id, user_id),
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
        )`,

        `CREATE TABLE IF NOT EXISTS user_signal_sources (
            id SERIAL PRIMARY KEY,
            user_id TEXT NOT NULL,
            coin_pool_url TEXT DEFAULT '',
            oi_top_url TEXT DEFAULT '',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
            UNIQUE(user_id)
        )`,

        `CREATE TABLE IF NOT EXISTS traders (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL DEFAULT 'default',
            name TEXT NOT NULL,
            ai_model_id TEXT NOT NULL,
            exchange_id TEXT NOT NULL,
            initial_balance DOUBLE PRECISION NOT NULL,
            scan_interval_minutes INTEGER DEFAULT 3,
            is_running BOOLEAN DEFAULT FALSE,
            btc_eth_leverage INTEGER DEFAULT 5,
            altcoin_leverage INTEGER DEFAULT 5,
            trading_symbols TEXT DEFAULT '',
            use_coin_pool BOOLEAN DEFAULT FALSE,
            use_oi_top BOOLEAN DEFAULT FALSE,
            custom_prompt TEXT DEFAULT '',
            override_base_prompt BOOLEAN DEFAULT FALSE,
            system_prompt_template TEXT DEFAULT 'default',
            is_cross_margin BOOLEAN DEFAULT TRUE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
        )`,

        `CREATE TABLE IF NOT EXISTS system_config (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,

        `CREATE TABLE IF NOT EXISTS beta_codes (
            code TEXT PRIMARY KEY,
            used BOOLEAN DEFAULT FALSE,
            used_by TEXT DEFAULT '',
            used_at TIMESTAMP DEFAULT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,

        `CREATE OR REPLACE FUNCTION set_updated_at()
         RETURNS TRIGGER AS $$
         BEGIN
           NEW.updated_at := CURRENT_TIMESTAMP;
           RETURN NEW;
         END;
         $$ LANGUAGE plpgsql`,

        `DROP TRIGGER IF EXISTS update_users_updated_at ON users`,
        `CREATE TRIGGER update_users_updated_at
           BEFORE UPDATE ON users
           FOR EACH ROW EXECUTE FUNCTION set_updated_at()`,

        `DROP TRIGGER IF EXISTS update_ai_models_updated_at ON ai_models`,
        `CREATE TRIGGER update_ai_models_updated_at
           BEFORE UPDATE ON ai_models
           FOR EACH ROW EXECUTE FUNCTION set_updated_at()`,

        `DROP TRIGGER IF EXISTS update_exchanges_updated_at ON exchanges`,
        `CREATE TRIGGER update_exchanges_updated_at
           BEFORE UPDATE ON exchanges
           FOR EACH ROW EXECUTE FUNCTION set_updated_at()`,

        `DROP TRIGGER IF EXISTS update_traders_updated_at ON traders`,
        `CREATE TRIGGER update_traders_updated_at
           BEFORE UPDATE ON traders
           FOR EACH ROW EXECUTE FUNCTION set_updated_at()`,

        `DROP TRIGGER IF EXISTS update_user_signal_sources_updated_at ON user_signal_sources`,
        `CREATE TRIGGER update_user_signal_sources_updated_at
           BEFORE UPDATE ON user_signal_sources
           FOR EACH ROW EXECUTE FUNCTION set_updated_at()`,

        `DROP TRIGGER IF EXISTS update_system_config_updated_at ON system_config`,
        `CREATE TRIGGER update_system_config_updated_at
           BEFORE UPDATE ON system_config
           FOR EACH ROW EXECUTE FUNCTION set_updated_at()`,
    }

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败 [%s]: %w", query, err)
		}
	}

    // 为现有数据库添加新字段（向后兼容）
    alterQueries := []string{
        `ALTER TABLE IF EXISTS exchanges ADD COLUMN IF NOT EXISTS hyperliquid_wallet_addr TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS exchanges ADD COLUMN IF NOT EXISTS aster_user TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS exchanges ADD COLUMN IF NOT EXISTS aster_signer TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS exchanges ADD COLUMN IF NOT EXISTS aster_private_key TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS custom_prompt TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS override_base_prompt BOOLEAN DEFAULT FALSE`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS is_cross_margin BOOLEAN DEFAULT TRUE`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS custom_coins TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS btc_eth_leverage INTEGER DEFAULT 5`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS altcoin_leverage INTEGER DEFAULT 5`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS trading_symbols TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS use_coin_pool BOOLEAN DEFAULT FALSE`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS use_oi_top BOOLEAN DEFAULT FALSE`,
        `ALTER TABLE IF EXISTS traders ADD COLUMN IF NOT EXISTS system_prompt_template TEXT DEFAULT 'default'`,
        `ALTER TABLE IF EXISTS ai_models ADD COLUMN IF NOT EXISTS custom_api_url TEXT DEFAULT ''`,
        `ALTER TABLE IF EXISTS ai_models ADD COLUMN IF NOT EXISTS custom_model_name TEXT DEFAULT ''`,
    }

	for _, query := range alterQueries {
		// 忽略已存在字段的错误
		d.db.Exec(query)
	}

    return nil
}

// initDefaultData 初始化默认数据
func (d *Database) initDefaultData() error {
    _, err := d.db.Exec(`
        INSERT INTO users (id, email, password_hash, otp_secret, otp_verified)
        VALUES ('default', 'default@localhost', '', '', TRUE)
        ON CONFLICT (id) DO NOTHING
    `)
    if err != nil {
        return fmt.Errorf("初始化用户失败: %w", err)
    }
	// 初始化AI模型（使用default用户）
	aiModels := []struct {
		id, name, provider string
	}{
		{"deepseek", "DeepSeek", "deepseek"},
		{"qwen", "Qwen", "qwen"},
	}

    for _, model := range aiModels {
        _, err := d.db.Exec(`
            INSERT INTO ai_models (id, user_id, name, provider, enabled)
            VALUES ($1, 'default', $2, $3, FALSE)
            ON CONFLICT (id) DO NOTHING
        `, model.id, model.name, model.provider)
        if err != nil {
            return fmt.Errorf("初始化AI模型失败: %w", err)
        }
    }

	// 初始化交易所（使用default用户）
	exchanges := []struct {
		id, name, typ string
	}{
		{"binance", "Binance Futures", "binance"},
		{"hyperliquid", "Hyperliquid", "hyperliquid"},
		{"aster", "Aster DEX", "aster"},
	}

    for _, exchange := range exchanges {
        _, err := d.db.Exec(`
            INSERT INTO exchanges (id, user_id, name, type, enabled)
            VALUES ($1, 'default', $2, $3, FALSE)
            ON CONFLICT (id, user_id) DO NOTHING
        `, exchange.id, exchange.name, exchange.typ)
        if err != nil {
            return fmt.Errorf("初始化交易所失败: %w", err)
        }
    }

	// 初始化系统配置 - 创建所有字段，设置默认值，后续由config.json同步更新
	systemConfigs := map[string]string{
		"beta_mode":            "false",                                                                               // 默认关闭内测模式
		"api_server_port":      "8080",                                                                                // 默认API端口
		"use_default_coins":    "true",                                                                                // 默认使用内置币种列表
		"default_coins":        `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`, // 默认币种列表（JSON格式）
		"max_daily_loss":       "10.0",                                                                                // 最大日损失百分比
		"max_drawdown":         "20.0",                                                                                // 最大回撤百分比
		"stop_trading_minutes": "60",                                                                                  // 停止交易时间（分钟）
		"btc_eth_leverage":     "5",                                                                                   // BTC/ETH杠杆倍数
		"altcoin_leverage":     "5",                                                                                   // 山寨币杠杆倍数
		"jwt_secret":           "",                                                                                    // JWT密钥，默认为空，由config.json或系统生成
	}

    for key, value := range systemConfigs {
        _, err := d.db.Exec(`
            INSERT INTO system_config (key, value)
            VALUES ($1, $2)
            ON CONFLICT (key) DO NOTHING
        `, key, value)
        if err != nil {
            return fmt.Errorf("初始化系统配置失败: %w", err)
        }
    }

	return nil
}

// migrateExchangesTable 迁移exchanges表支持多用户
func (d *Database) migrateExchangesTable() error {
	// 检查是否已经迁移过
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='exchanges_new'
	`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已经迁移过，直接返回
	if count > 0 {
		return nil
	}

	log.Printf("🔄 开始迁移exchanges表...")

	// 创建新的exchanges表，使用复合主键
	_, err = d.db.Exec(`
		CREATE TABLE exchanges_new (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("创建新exchanges表失败: %w", err)
	}

	// 复制数据到新表
	_, err = d.db.Exec(`
		INSERT INTO exchanges_new 
		SELECT * FROM exchanges
	`)
	if err != nil {
		return fmt.Errorf("复制数据失败: %w", err)
	}

	// 删除旧表
	_, err = d.db.Exec(`DROP TABLE exchanges`)
	if err != nil {
		return fmt.Errorf("删除旧表失败: %w", err)
	}

	// 重命名新表
	_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
	if err != nil {
		return fmt.Errorf("重命名表失败: %w", err)
	}

	// 重新创建触发器
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP 
				WHERE id = NEW.id AND user_id = NEW.user_id;
			END
	`)
	if err != nil {
		return fmt.Errorf("创建触发器失败: %w", err)
	}

	log.Printf("✅ exchanges表迁移完成")
	return nil
}

// User 用户配置
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // 不返回到前端
	OTPSecret    string    `json:"-"` // 不返回到前端
	OTPVerified  bool      `json:"otp_verified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AIModelConfig AI模型配置
type AIModelConfig struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	Enabled         bool      `json:"enabled"`
	APIKey          string    `json:"apiKey"`
	CustomAPIURL    string    `json:"customApiUrl"`
	CustomModelName string    `json:"customModelName"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ExchangeConfig 交易所配置
type ExchangeConfig struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey"`    // For Binance: API Key; For Hyperliquid: Agent Private Key (should have ~0 balance)
	SecretKey string `json:"secretKey"` // For Binance: Secret Key; Not used for Hyperliquid
	Testnet   bool   `json:"testnet"`
	// Hyperliquid Agent Wallet configuration (following official best practices)
	// Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Main Wallet Address (holds funds, never expose private key)
	// Aster 特定字段
	AsterUser       string    `json:"asterUser"`
	AsterSigner     string    `json:"asterSigner"`
	AsterPrivateKey string    `json:"asterPrivateKey"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TraderRecord 交易员配置（数据库实体）
type TraderRecord struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Name                 string    `json:"name"`
	AIModelID            string    `json:"ai_model_id"`
	ExchangeID           string    `json:"exchange_id"`
	InitialBalance       float64   `json:"initial_balance"`
	ScanIntervalMinutes  int       `json:"scan_interval_minutes"`
	IsRunning            bool      `json:"is_running"`
	BTCETHLeverage       int       `json:"btc_eth_leverage"`       // BTC/ETH杠杆倍数
	AltcoinLeverage      int       `json:"altcoin_leverage"`       // 山寨币杠杆倍数
	TradingSymbols       string    `json:"trading_symbols"`        // 交易币种，逗号分隔
	UseCoinPool          bool      `json:"use_coin_pool"`          // 是否使用COIN POOL信号源
	UseOITop             bool      `json:"use_oi_top"`             // 是否使用OI TOP信号源
	CustomPrompt         string    `json:"custom_prompt"`          // 自定义交易策略prompt
	OverrideBasePrompt   bool      `json:"override_base_prompt"`   // 是否覆盖基础prompt
	SystemPromptTemplate string    `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        bool      `json:"is_cross_margin"`        // 是否为全仓模式（true=全仓，false=逐仓）
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// UserSignalSource 用户信号源配置
type UserSignalSource struct {
	ID          int       `json:"id"`
	UserID      string    `json:"user_id"`
	CoinPoolURL string    `json:"coin_pool_url"`
	OITopURL    string    `json:"oi_top_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

// CreateUser 创建用户
func (d *Database) CreateUser(user *User) error {
    _, err := d.db.Exec(`
        INSERT INTO users (id, email, password_hash, otp_secret, otp_verified)
        VALUES ($1, $2, $3, $4, $5)
    `, user.ID, user.Email, user.PasswordHash, user.OTPSecret, user.OTPVerified)
    return err
}

// EnsureAdminUser 确保admin用户存在（用于管理员模式）
func (d *Database) EnsureAdminUser() error {
	// 检查admin用户是否已存在
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已存在，直接返回
	if count > 0 {
		return nil
	}

	// 创建admin用户（密码为空，因为管理员模式下不需要密码）
	adminUser := &User{
		ID:           "admin",
		Email:        "admin@localhost",
		PasswordHash: "", // 管理员模式下不使用密码
		OTPSecret:    "",
		OTPVerified:  true,
	}

	return d.CreateUser(adminUser)
}

// GetUserByEmail 通过邮箱获取用户
func (d *Database) GetUserByEmail(email string) (*User, error) {
	var user User
    err := d.db.QueryRow(`
        SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
        FROM users WHERE email = $1
    `, email).Scan(
        &user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
        &user.OTPVerified, &user.CreatedAt, &user.UpdatedAt,
    )
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID 通过ID获取用户
func (d *Database) GetUserByID(userID string) (*User, error) {
	var user User
    err := d.db.QueryRow(`
        SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
        FROM users WHERE id = $1
    `, userID).Scan(
        &user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
        &user.OTPVerified, &user.CreatedAt, &user.UpdatedAt,
    )
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAllUsers 获取所有用户ID列表
func (d *Database) GetAllUsers() ([]string, error) {
	rows, err := d.db.Query(`SELECT id FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

// UpdateUserOTPVerified 更新用户OTP验证状态
func (d *Database) UpdateUserOTPVerified(userID string, verified bool) error {
    _, err := d.db.Exec(`UPDATE users SET otp_verified = $1 WHERE id = $2`, verified, userID)
    return err
}

// UpdateUserPassword 更新用户密码
func (d *Database) UpdateUserPassword(userID, passwordHash string) error {
    _, err := d.db.Exec(`
        UPDATE users
        SET password_hash = $1, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2
    `, passwordHash, userID)
    return err
}

// GetAIModels 获取用户的AI模型配置
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
    rows, err := d.db.Query(`
        SELECT id, user_id, name, provider, enabled, api_key,
               COALESCE(custom_api_url, '') as custom_api_url,
               COALESCE(custom_model_name, '') as custom_model_name,
               created_at, updated_at
        FROM ai_models WHERE user_id = $1 ORDER BY id
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	models := make([]*AIModelConfig, 0)
	for rows.Next() {
		var model AIModelConfig
		err := rows.Scan(
			&model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&model.CreatedAt, &model.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		// 解密API Key
		model.APIKey = d.decryptSensitiveData(model.APIKey)
		models = append(models, &model)
	}

	return models, nil
}

// UpdateAIModel 更新AI模型配置，如果不存在则创建用户特定配置
func (d *Database) UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	// 先尝试精确匹配 ID（新版逻辑，支持多个相同 provider 的模型）
	var existingID string
    err := d.db.QueryRow(`
        SELECT id FROM ai_models WHERE user_id = $1 AND id = $2 LIMIT 1
    `, userID, id).Scan(&existingID)

	if err == nil {
		// 找到了现有配置（精确匹配 ID），更新它
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
        _, err = d.db.Exec(`
            UPDATE ai_models SET enabled = $1, api_key = $2, custom_api_url = $3, custom_model_name = $4, updated_at = CURRENT_TIMESTAMP
            WHERE id = $5 AND user_id = $6
        `, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
        return err
    }

	// ID 不存在，尝试兼容旧逻辑：将 id 作为 provider 查找
	provider := id
    err = d.db.QueryRow(`
        SELECT id FROM ai_models WHERE user_id = $1 AND provider = $2 LIMIT 1
    `, userID, provider).Scan(&existingID)

	if err == nil {
		// 找到了现有配置（通过 provider 匹配，兼容旧版），更新它
		log.Printf("⚠️  使用旧版 provider 匹配更新模型: %s -> %s", provider, existingID)
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
        _, err = d.db.Exec(`
            UPDATE ai_models SET enabled = $1, api_key = $2, custom_api_url = $3, custom_model_name = $4, updated_at = CURRENT_TIMESTAMP
            WHERE id = $5 AND user_id = $6
        `, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
        return err
    }

	// 没有找到任何现有配置，创建新的
	// 推断 provider（从 id 中提取，或者直接使用 id）
	if provider == id && (provider == "deepseek" || provider == "qwen") {
		// id 本身就是 provider
		provider = id
	} else {
		// 从 id 中提取 provider（假设格式是 userID_provider 或 timestamp_userID_provider）
		parts := strings.Split(id, "_")
		if len(parts) >= 2 {
			provider = parts[len(parts)-1] // 取最后一部分作为 provider
		} else {
			provider = id
		}
	}

	// 获取模型的基本信息
	var name string
    err = d.db.QueryRow(`
        SELECT name FROM ai_models WHERE provider = $1 LIMIT 1
    `, provider).Scan(&name)
	if err != nil {
		// 如果找不到基本信息，使用默认值
		if provider == "deepseek" {
			name = "DeepSeek AI"
		} else if provider == "qwen" {
			name = "Qwen AI"
		} else {
			name = provider + " AI"
		}
	}

	// 如果传入的 ID 已经是完整格式（如 "admin_deepseek_custom1"），直接使用
	// 否则生成新的 ID
	newModelID := id
	if id == provider {
		// id 就是 provider，生成新的用户特定 ID
		newModelID = fmt.Sprintf("%s_%s", userID, provider)
	}

	log.Printf("✓ 创建新的 AI 模型配置: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
    _, err = d.db.Exec(`
        INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
        ON CONFLICT (id) DO UPDATE SET
          user_id = EXCLUDED.user_id,
          name = EXCLUDED.name,
          provider = EXCLUDED.provider,
          enabled = EXCLUDED.enabled,
          api_key = EXCLUDED.api_key,
          custom_api_url = EXCLUDED.custom_api_url,
          custom_model_name = EXCLUDED.custom_model_name,
          updated_at = CURRENT_TIMESTAMP
    `, newModelID, userID, name, provider, enabled, encryptedAPIKey, customAPIURL, customModelName)

	return err
}

// GetExchanges 获取用户的交易所配置
func (d *Database) GetExchanges(userID string) ([]*ExchangeConfig, error) {
    rows, err := d.db.Query(`
        SELECT id, user_id, name, type, enabled, api_key, secret_key, testnet, 
               COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
               COALESCE(aster_user, '') as aster_user,
               COALESCE(aster_signer, '') as aster_signer,
               COALESCE(aster_private_key, '') as aster_private_key,
               created_at, updated_at 
        FROM exchanges WHERE user_id = $1 ORDER BY id
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	exchanges := make([]*ExchangeConfig, 0)
	for rows.Next() {
		var exchange ExchangeConfig
		err := rows.Scan(
			&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type,
			&exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
			&exchange.HyperliquidWalletAddr, &exchange.AsterUser,
			&exchange.AsterSigner, &exchange.AsterPrivateKey,
			&exchange.CreatedAt, &exchange.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// 解密敏感字段
		exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
		exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
		exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)

		exchanges = append(exchanges, &exchange)
	}

	return exchanges, nil
}

// UpdateExchange 更新交易所配置，如果不存在则创建用户特定配置
// 🔒 安全特性：空值不会覆盖现有的敏感字段（api_key, secret_key, aster_private_key）
func (d *Database) UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	log.Printf("🔧 UpdateExchange: userID=%s, id=%s, enabled=%v", userID, id, enabled)

    // 构建动态 UPDATE 语句（PostgreSQL 占位符）
    idx := 1
    setClauses := []string{}
    args := []interface{}{}

    setClauses = append(setClauses, fmt.Sprintf("enabled = $%d", idx))
    args = append(args, enabled)
    idx++
    setClauses = append(setClauses, fmt.Sprintf("testnet = $%d", idx))
    args = append(args, testnet)
    idx++
    setClauses = append(setClauses, fmt.Sprintf("hyperliquid_wallet_addr = $%d", idx))
    args = append(args, hyperliquidWalletAddr)
    idx++
    setClauses = append(setClauses, fmt.Sprintf("aster_user = $%d", idx))
    args = append(args, asterUser)
    idx++
    setClauses = append(setClauses, fmt.Sprintf("aster_signer = $%d", idx))
    args = append(args, asterSigner)
    idx++
    setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")

    if apiKey != "" {
        encryptedAPIKey := d.encryptSensitiveData(apiKey)
        setClauses = append(setClauses, fmt.Sprintf("api_key = $%d", idx))
        args = append(args, encryptedAPIKey)
        idx++
    }
    if secretKey != "" {
        encryptedSecretKey := d.encryptSensitiveData(secretKey)
        setClauses = append(setClauses, fmt.Sprintf("secret_key = $%d", idx))
        args = append(args, encryptedSecretKey)
        idx++
    }
    if asterPrivateKey != "" {
        encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
        setClauses = append(setClauses, fmt.Sprintf("aster_private_key = $%d", idx))
        args = append(args, encryptedAsterPrivateKey)
        idx++
    }

    // WHERE 条件占位符
    whereID := idx
    whereUser := idx + 1
    args = append(args, id, userID)

    query := fmt.Sprintf(`
        UPDATE exchanges SET %s
        WHERE id = $%d AND user_id = $%d
    `, strings.Join(setClauses, ", "), whereID, whereUser)

    result, err := d.db.Exec(query, args...)
	if err != nil {
		log.Printf("❌ UpdateExchange: 更新失败: %v", err)
		return err
	}

	// 检查是否有行被更新
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("❌ UpdateExchange: 获取影响行数失败: %v", err)
		return err
	}

	log.Printf("📊 UpdateExchange: 影响行数 = %d", rowsAffected)

	// 如果没有行被更新，说明用户没有这个交易所的配置，需要创建
	if rowsAffected == 0 {
		log.Printf("💡 UpdateExchange: 没有现有记录，创建新记录")

		// 根据交易所ID确定基本信息
		var name, typ string
		if id == "binance" {
			name = "Binance Futures"
			typ = "cex"
		} else if id == "hyperliquid" {
			name = "Hyperliquid"
			typ = "dex"
		} else if id == "aster" {
			name = "Aster DEX"
			typ = "dex"
		} else {
			name = id + " Exchange"
			typ = "cex"
		}

		log.Printf("🆕 UpdateExchange: 创建新记录 ID=%s, name=%s, type=%s", id, name, typ)

		// 创建用户特定的配置，使用原始的交易所ID
        _, err = d.db.Exec(`
            INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
                                   hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
            ON CONFLICT (id, user_id) DO UPDATE SET
              name = EXCLUDED.name,
              type = EXCLUDED.type,
              enabled = EXCLUDED.enabled,
              api_key = EXCLUDED.api_key,
              secret_key = EXCLUDED.secret_key,
              testnet = EXCLUDED.testnet,
              hyperliquid_wallet_addr = EXCLUDED.hyperliquid_wallet_addr,
              aster_user = EXCLUDED.aster_user,
              aster_signer = EXCLUDED.aster_signer,
              aster_private_key = EXCLUDED.aster_private_key,
              updated_at = CURRENT_TIMESTAMP
        `, id, userID, name, typ, enabled, apiKey, secretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey)

		if err != nil {
			log.Printf("❌ UpdateExchange: 创建记录失败: %v", err)
		} else {
			log.Printf("✅ UpdateExchange: 创建记录成功")
		}
		return err
	}

	log.Printf("✅ UpdateExchange: 更新现有记录成功")
	return nil
}

// CreateAIModel 创建AI模型配置
func (d *Database) CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
    _, err := d.db.Exec(`
        INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (id) DO NOTHING
    `, id, userID, name, provider, enabled, apiKey, customAPIURL)
	return err
}

// CreateExchange 创建交易所配置
func (d *Database) CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	// 加密敏感字段
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	encryptedSecretKey := d.encryptSensitiveData(secretKey)
	encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)

    _, err := d.db.Exec(`
        INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        ON CONFLICT (id, user_id) DO NOTHING
    `, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey)
	return err
}

// CreateTrader 创建交易员
func (d *Database) CreateTrader(trader *TraderRecord) error {
    _, err := d.db.Exec(`
        INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
    `, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate, trader.IsCrossMargin)
	return err
}

// GetTraders 获取用户的交易员
func (d *Database) GetTraders(userID string) ([]*TraderRecord, error) {
    rows, err := d.db.Query(`
        SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
               COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
               COALESCE(trading_symbols, '') as trading_symbols,
               COALESCE(use_coin_pool, FALSE) as use_coin_pool, COALESCE(use_oi_top, FALSE) as use_oi_top,
               COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, FALSE) as override_base_prompt,
               COALESCE(system_prompt_template, 'default') as system_prompt_template,
               COALESCE(is_cross_margin, TRUE) as is_cross_margin, created_at, updated_at
        FROM traders WHERE user_id = $1 ORDER BY created_at DESC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// UpdateTraderStatus 更新交易员状态
func (d *Database) UpdateTraderStatus(userID, id string, isRunning bool) error {
    _, err := d.db.Exec(`UPDATE traders SET is_running = $1 WHERE id = $2 AND user_id = $3`, isRunning, id, userID)
    return err
}

// UpdateTrader 更新交易员配置
func (d *Database) UpdateTrader(trader *TraderRecord) error {
    _, err := d.db.Exec(`
        UPDATE traders SET
            name = $1, ai_model_id = $2, exchange_id = $3, initial_balance = $4,
            scan_interval_minutes = $5, btc_eth_leverage = $6, altcoin_leverage = $7,
            trading_symbols = $8, custom_prompt = $9, override_base_prompt = $10,
            system_prompt_template = $11, is_cross_margin = $12, updated_at = CURRENT_TIMESTAMP
        WHERE id = $13 AND user_id = $14
    `, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance,
        trader.ScanIntervalMinutes, trader.BTCETHLeverage, trader.AltcoinLeverage,
        trader.TradingSymbols, trader.CustomPrompt, trader.OverrideBasePrompt,
        trader.SystemPromptTemplate, trader.IsCrossMargin, trader.ID, trader.UserID)
    return err
}

// UpdateTraderCustomPrompt 更新交易员自定义Prompt
func (d *Database) UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
    _, err := d.db.Exec(`UPDATE traders SET custom_prompt = $1, override_base_prompt = $2 WHERE id = $3 AND user_id = $4`, customPrompt, overrideBase, id, userID)
    return err
}

// UpdateTraderInitialBalance 更新交易员初始余额（用于自动同步交易所实际余额）
func (d *Database) UpdateTraderInitialBalance(userID, id string, newBalance float64) error {
    _, err := d.db.Exec(`UPDATE traders SET initial_balance = $1 WHERE id = $2 AND user_id = $3`, newBalance, id, userID)
    return err
}

// DeleteTrader 删除交易员
func (d *Database) DeleteTrader(userID, id string) error {
    _, err := d.db.Exec(`DELETE FROM traders WHERE id = $1 AND user_id = $2`, id, userID)
    return err
}

// GetTraderConfig 获取交易员完整配置（包含AI模型和交易所信息）
func (d *Database) GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error) {
	var trader TraderRecord
	var aiModel AIModelConfig
	var exchange ExchangeConfig

    err := d.db.QueryRow(`
        SELECT
			t.id, t.user_id, t.name, t.ai_model_id, t.exchange_id, t.initial_balance, t.scan_interval_minutes, t.is_running,
			COALESCE(t.btc_eth_leverage, 5) as btc_eth_leverage,
			COALESCE(t.altcoin_leverage, 5) as altcoin_leverage,
			COALESCE(t.trading_symbols, '') as trading_symbols,
            COALESCE(t.use_coin_pool, FALSE) as use_coin_pool,
            COALESCE(t.use_oi_top, FALSE) as use_oi_top,
			COALESCE(t.custom_prompt, '') as custom_prompt,
            COALESCE(t.override_base_prompt, FALSE) as override_base_prompt,
			COALESCE(t.system_prompt_template, 'default') as system_prompt_template,
            COALESCE(t.is_cross_margin, TRUE) as is_cross_margin,
			t.created_at, t.updated_at,
			a.id, a.user_id, a.name, a.provider, a.enabled, a.api_key,
			COALESCE(a.custom_api_url, '') as custom_api_url,
			COALESCE(a.custom_model_name, '') as custom_model_name,
			a.created_at, a.updated_at,
			e.id, e.user_id, e.name, e.type, e.enabled, e.api_key, e.secret_key, e.testnet,
			COALESCE(e.hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
			COALESCE(e.aster_user, '') as aster_user,
			COALESCE(e.aster_signer, '') as aster_signer,
			COALESCE(e.aster_private_key, '') as aster_private_key,
			e.created_at, e.updated_at
		FROM traders t
		JOIN ai_models a ON t.ai_model_id = a.id AND t.user_id = a.user_id
		JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id
        WHERE t.id = $1 AND t.user_id = $2
    `, traderID, userID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop,
		&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
		&trader.IsCrossMargin,
		&trader.CreatedAt, &trader.UpdatedAt,
		&aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CustomAPIURL, &aiModel.CustomModelName,
		&aiModel.CreatedAt, &aiModel.UpdatedAt,
		&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.HyperliquidWalletAddr, &exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchange.CreatedAt, &exchange.UpdatedAt,
	)

	if err != nil {
		return nil, nil, nil, err
	}

	// 解密敏感数据
	aiModel.APIKey = d.decryptSensitiveData(aiModel.APIKey)
	exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
	exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
	exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)

	return &trader, &aiModel, &exchange, nil
}

// GetSystemConfig 获取系统配置
func (d *Database) GetSystemConfig(key string) (string, error) {
    var value string
    err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
    return value, err
}

// SetSystemConfig 设置系统配置
func (d *Database) SetSystemConfig(key, value string) error {
    _, err := d.db.Exec(`
        INSERT INTO system_config (key, value)
        VALUES ($1, $2)
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
    `, key, value)
    return err
}

// CreateUserSignalSource 创建用户信号源配置
func (d *Database) CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
    _, err := d.db.Exec(`
        INSERT INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
        VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
        ON CONFLICT (user_id) DO UPDATE SET
          coin_pool_url = EXCLUDED.coin_pool_url,
          oi_top_url = EXCLUDED.oi_top_url,
          updated_at = CURRENT_TIMESTAMP
    `, userID, coinPoolURL, oiTopURL)
    return err
}

// GetUserSignalSource 获取用户信号源配置
func (d *Database) GetUserSignalSource(userID string) (*UserSignalSource, error) {
	var source UserSignalSource
    err := d.db.QueryRow(`
        SELECT id, user_id, coin_pool_url, oi_top_url, created_at, updated_at
        FROM user_signal_sources WHERE user_id = $1
    `, userID).Scan(
        &source.ID, &source.UserID, &source.CoinPoolURL, &source.OITopURL,
        &source.CreatedAt, &source.UpdatedAt,
    )
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// UpdateUserSignalSource 更新用户信号源配置
func (d *Database) UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
    _, err := d.db.Exec(`
        UPDATE user_signal_sources SET coin_pool_url = $1, oi_top_url = $2, updated_at = CURRENT_TIMESTAMP
        WHERE user_id = $3
    `, coinPoolURL, oiTopURL, userID)
    return err
}

// GetCustomCoins 获取所有交易员自定义币种 / Get all trader-customized currencies
func (d *Database) GetCustomCoins() []string {
	var symbol string
	var symbols []string
    _ = d.db.QueryRow(`
        SELECT STRING_AGG(custom_coins, ',') AS symbol
        FROM traders WHERE custom_coins != ''
    `).Scan(&symbol)
	// 检测用户是否未配置币种 - 兼容性
	if symbol == "" {
		symbolJSON, _ := d.GetSystemConfig("default_coins")
		if err := json.Unmarshal([]byte(symbolJSON), &symbols); err != nil {
			log.Printf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
			symbols = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
		}
	}
	// filter Symbol
	for _, s := range strings.Split(symbol, ",") {
		if s == "" {
			continue
		}
		coin := market.Normalize(s)
		if !slices.Contains(symbols, coin) {
			symbols = append(symbols, coin)
		}
	}
	return symbols
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	return d.db.Close()
}

// LoadBetaCodesFromFile 从文件加载内测码到数据库
func (d *Database) LoadBetaCodesFromFile(filePath string) error {
	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取内测码文件失败: %w", err)
	}

	// 按行分割内测码
	lines := strings.Split(string(content), "\n")
	var codes []string
	for _, line := range lines {
		code := strings.TrimSpace(line)
		if code != "" && !strings.HasPrefix(code, "#") {
			codes = append(codes, code)
		}
	}

	// 批量插入内测码
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

    stmt, err := tx.Prepare(`INSERT INTO beta_codes (code) VALUES ($1) ON CONFLICT (code) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, code := range codes {
		result, err := stmt.Exec(code)
		if err != nil {
			log.Printf("插入内测码 %s 失败: %v", code, err)
			continue
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("✅ 成功加载 %d 个内测码到数据库 (总计 %d 个)", insertedCount, len(codes))
	return nil
}

// ValidateBetaCode 验证内测码是否有效且未使用
func (d *Database) ValidateBetaCode(code string) (bool, error) {
	var used bool
    err := d.db.QueryRow(`SELECT used FROM beta_codes WHERE code = $1`, code).Scan(&used)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // 内测码不存在
		}
		return false, err
	}
	return !used, nil // 内测码存在且未使用
}

// UseBetaCode 使用内测码（标记为已使用）
func (d *Database) UseBetaCode(code, userEmail string) error {
    result, err := d.db.Exec(`
        UPDATE beta_codes SET used = TRUE, used_by = $1, used_at = CURRENT_TIMESTAMP 
        WHERE code = $2 AND used = FALSE
    `, userEmail, code)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("内测码无效或已被使用")
	}

	return nil
}

// GetBetaCodeStats 获取内测码统计信息
func (d *Database) GetBetaCodeStats() (total, used int, err error) {
	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes`).Scan(&total)
	if err != nil {
		return 0, 0, err
	}

	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes WHERE used = 1`).Scan(&used)
	if err != nil {
		return 0, 0, err
	}

	return total, used, nil
}

// SetCryptoService 设置加密服务
func (d *Database) SetCryptoService(cs *crypto.CryptoService) {
	d.cryptoService = cs
}

// encryptSensitiveData 加密敏感数据用于存储
func (d *Database) encryptSensitiveData(plaintext string) string {
	if d.cryptoService == nil || plaintext == "" {
		return plaintext
	}

	encrypted, err := d.cryptoService.EncryptForStorage(plaintext)
	if err != nil {
		log.Printf("⚠️ 加密失败: %v", err)
		return plaintext // 返回明文作为降级处理
	}

	return encrypted
}

// decryptSensitiveData 解密敏感数据
func (d *Database) decryptSensitiveData(encrypted string) string {
	if d.cryptoService == nil || encrypted == "" {
		return encrypted
	}

	// 如果不是加密格式，直接返回
	if !d.cryptoService.IsEncryptedStorageValue(encrypted) {
		return encrypted
	}

	decrypted, err := d.cryptoService.DecryptFromStorage(encrypted)
	if err != nil {
		log.Printf("⚠️ 解密失败: %v", err)
		return encrypted // 返回加密文本作为降级处理
	}

	return decrypted
}
