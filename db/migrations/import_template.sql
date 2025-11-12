-- Import data from CSV files exported from SQLite
-- Adjust file paths as needed and ensure CSV headers match column order

-- Users
COPY users (id, email, password_hash, otp_secret, otp_verified, created_at, updated_at)
FROM '/path/to/csv/users.csv' DELIMITER ',' CSV HEADER;

-- AI Models
COPY ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
FROM '/path/to/csv/ai_models.csv' DELIMITER ',' CSV HEADER;

-- Exchanges
COPY exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at)
FROM '/path/to/csv/exchanges.csv' DELIMITER ',' CSV HEADER;

-- Traders
COPY traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin, created_at, updated_at)
FROM '/path/to/csv/traders.csv' DELIMITER ',' CSV HEADER;

-- User Signal Sources
COPY user_signal_sources (id, user_id, coin_pool_url, oi_top_url, created_at, updated_at)
FROM '/path/to/csv/user_signal_sources.csv' DELIMITER ',' CSV HEADER;

-- System Config
COPY system_config (key, value, updated_at)
FROM '/path/to/csv/system_config.csv' DELIMITER ',' CSV HEADER;

-- Beta Codes
COPY beta_codes (code, used, used_by, used_at, created_at)
FROM '/path/to/csv/beta_codes.csv' DELIMITER ',' CSV HEADER;

