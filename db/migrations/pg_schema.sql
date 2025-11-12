-- PostgreSQL schema for NOFX-Lite (converted from SQLite)
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  otp_secret TEXT,
  otp_verified BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_models (
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
);

CREATE TABLE IF NOT EXISTS exchanges (
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
);

CREATE TABLE IF NOT EXISTS user_signal_sources (
  id SERIAL PRIMARY KEY,
  user_id TEXT NOT NULL,
  coin_pool_url TEXT DEFAULT '',
  oi_top_url TEXT DEFAULT '',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE(user_id)
);

CREATE TABLE IF NOT EXISTS traders (
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
);

CREATE TABLE IF NOT EXISTS system_config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS beta_codes (
  code TEXT PRIMARY KEY,
  used BOOLEAN DEFAULT FALSE,
  used_by TEXT DEFAULT '',
  used_at TIMESTAMP DEFAULT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at := CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_ai_models_updated_at
  BEFORE UPDATE ON ai_models
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_exchanges_updated_at
  BEFORE UPDATE ON exchanges
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_traders_updated_at
  BEFORE UPDATE ON traders
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_user_signal_sources_updated_at
  BEFORE UPDATE ON user_signal_sources
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER update_system_config_updated_at
  BEFORE UPDATE ON system_config
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

