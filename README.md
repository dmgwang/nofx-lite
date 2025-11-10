# 🤖 NOFX-Lite - Agentic Trading OS

> Fork of [NOFX](https://github.com/NoFxAiOS/nofx)

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

> ⚠️ **Risk Warning**: This system is experimental. AI auto-trading carries significant risks. Strongly recommended for learning/research purposes or testing with small amounts only!

## 🚀 Multi-Exchange Support

NOFX-Lite supports **three major exchanges**: Binance, Hyperliquid, and Aster DEX!

---

## 📸 Screenshots

### 🏆 Competition Mode - Real-time AI Battle
![Competition Page](screenshots/competition-page.png)
*Multi-AI leaderboard with real-time performance comparison*

### 📊 Trader Details - Complete Trading Dashboard
![Details Page](screenshots/details-page.png)
*Professional trading interface with equity curves and AI decision logs*

---

## 🏗️ Technical Architecture

- **Backend:** Go + Gin framework + SQLite
- **Frontend:** React 18 + TypeScript + Vite + TailwindCSS  
- **Multi-Exchange:** Binance, Hyperliquid, Aster DEX
- **AI Models:** DeepSeek, Qwen, OpenAI-compatible APIs
- **Real-time:** WebSocket + SWR polling

---

## 💰 Exchange Setup

### Binance (Fee Discount)
**[Register Binance - Get 30% Fee Discount](https://www.maxweb.red/referral/earn-together/refer2earn-usdc/claim?hl=en&ref=GRO_28502_F9I5J)**

**Steps:**
1. Register via link above
2. Complete KYC verification  
3. Enable Futures trading
4. Create API key with Futures permission
5. Whitelist your IP for security

---

## 🚀 Quick Start

### 🐳 Docker Deployment (Recommended)

**One-click deployment with Docker - handles all dependencies automatically**

```bash
# 1. Prepare config
cp config.json.example config.json
# Edit config.json with your settings

# 2. Deploy
chmod +x quick-deploy.sh && ./quick-deploy.sh

# 3. Access
# Open http://localhost:3000
```

**Setup via Web Interface:**
1. Configure AI Models (DeepSeek/Qwen API keys)
2. Configure Exchanges (Binance/Hyperliquid credentials)  
3. Create Traders (combine AI + exchange)
4. Start Trading

### 📦 Manual Installation (Developers)

**Prerequisites:** Go 1.21+, Node.js 18+, TA-Lib

**Install TA-Lib:**
```bash
# macOS
brew install ta-lib

# Ubuntu/Debian
sudo apt-get install -y build-essential wget
wget http://prdownloads.sourceforge.net/ta-lib/ta-lib-0.4.0-src.tar.gz
tar -xzf ta-lib-0.4.0-src.tar.gz && cd ta-lib
./configure --prefix=/usr/local && make && sudo make install
cd .. && rm -rf ta-lib ta-lib-0.4.0-src.tar.gz
```

**Build & Run:**
```bash
git clone https://github.com/dmgwang/nofx-lite.git && cd nofx-lite
cp config.json.example config.json  # Edit with your keys

cd backend && go mod download && go build -o nofx-backend && ./nofx-backend &
cd ../web && npm install && npm run dev

# Access: http://localhost:3000
```

### 4. Get AI API Keys

#### DeepSeek (Recommended)
1. Visit [DeepSeek Platform](https://platform.deepseek.com)
2. Create API key in API Keys section
3. Add funds (free credits available for new users)

#### Qwen (Alternative)
1. Visit [Alibaba Cloud DashScope](https://dashscope.console.aliyun.com)
2. Enable DashScope service
3. Create API key

#### Custom OpenAI API
Configure any OpenAI-compatible endpoint in the web interface

---

### 5. Start the System

#### **Step 1: Start the Backend**

```bash
# Build the program (first time only, or after code changes)
go build -o nofx

# Start the backend
./nofx
```

**What you should see:**

```
╔════════════════════════════════════════════════════════════╗
║    🤖 AI多模型交易系统 - 支持 DeepSeek & Qwen                  ║
╚════════════════════════════════════════════════════════════╝

🤖 数据库中的AI交易员配置:
  • 暂无配置的交易员，请通过Web界面创建

🌐 API服务器启动在 http://localhost:8081
```

#### **Step 2: Start the Frontend**

Open a **NEW terminal window**, then:

```bash
cd web
npm run dev
```

#### **Step 3: Access the Web Interface**

Open your browser and visit: **🌐 http://localhost:3000**

### 6. Configure Through Web Interface

**Now configure everything through the web interface - no more JSON editing!**

## 🎯 AI Model Configuration

Access web interface at http://localhost:3000 → Settings → Add AI Model

### DeepSeek
- API Key: `sk-xxxxxxxxxxxxx`
- Model: `deepseek-chat`
- Temperature: `0.1`
- Max Tokens: `4096`

### Qwen  
- API Key: `sk-xxxxxxxxxxxxx`
- Model: `qwen-turbo`
- Temperature: `0.1`
- Max Tokens: `4096`

#### **Step 2: Configure Exchanges**

1. Click "交易所配置" button
2. Enable Binance or Hyperliquid (or both)
3. Enter your API credentials
4. Save configuration

## 🎮 Create Your First Trader

1. Access web interface at http://localhost:3000
2. Navigate to Traders → Create Trader
3. Configure:
   - Name: "My DeepSeek Binance Trader"
   - AI Model: Select configured model
   - Exchange: Select configured exchange  
   - Initial Balance: 1000 USDT
   - Scan Interval: 3 minutes
   - Trading Pairs: Select crypto pairs
4. Click Create → Start to begin trading

**Monitor:** Dashboard (performance), Positions (P&L), History (trades), Logs (AI decisions)

#### **Step 4: Start Trading**

- Your traders will appear in the main interface
- Use Start/Stop buttons to control them
- Monitor performance in real-time

**✅ No more JSON file editing - everything is done through the web interface!**

---

#### 🔷 Hyperliquid Exchange

**NOFX supports Hyperliquid** - a decentralized perpetual futures exchange.

**⚙️ Configuration via Web Interface:**
1. Open http://localhost:3000 → Settings → Add Exchange
2. Select Hyperliquid
3. Enter:
   - Wallet Address
   - Private Key (⚠️ use dedicated wallet, remove 0x prefix)
   - Testnet toggle
4. Save

**⚠️ Security Warning**: Private key required – never share it!

---

#### 🔶 Aster DEX Exchange

**NOFX supports Aster DEX** - Binance-compatible decentralized perps.

**⚙️ Configuration via Web Interface:**
1. Open http://localhost:3000 → Settings → Add Exchange  
2. Select Aster DEX
3. Enter:
   - Wallet Address (User)
   - API Wallet Address (Signer)
   - Private Key (⚠️ shown once, remove 0x prefix)
   - Testnet toggle
4. Save

**⚠️ Security Warning**: API-wallet layer – revoke anytime via [asterdex.com](https://www.asterdex.com/en/api-wallet)

---

#### 🚀 Starting the System (2 steps)

The system has **2 parts** that run separately:

1. **Backend** (AI trading brain + API)
2. **Frontend** (Web dashboard for monitoring)

---

#### **Step 1: Start the Backend**

Open a terminal and run:

```bash
# Build the program (first time only, or after code changes)
go build -o nofx

# Start the backend
./nofx
```

**What you should see:**

```
🚀 启动自动交易系统...
✓ Trader [my_trader] 已初始化
✓ API服务器启动在端口 8080
📊 开始交易监控...
```

**⚠️ If you see errors:**

| Error Message                | Solution                                                                            |
| ---------------------------- | ----------------------------------------------------------------------------------- |
| `invalid API key`          | Check your Binance API key in config.json                                           |
| `TA-Lib not found`         | Run `brew install ta-lib` (macOS)                                                 |
| `port 8080 already in use` | ~~Change `api_server_port` in config.json~~ *Change `API_PORT` in .env file* |
| `DeepSeek API error`       | Verify your DeepSeek API key and balance                                            |

**✅ Backend is running correctly when you see:**

- No error messages
- "开始交易监控..." appears
- System shows account balance
- Keep this terminal window open!

---

#### **Step 2: Start the Frontend**

Open a **NEW terminal window** (keep the first one running!), then:

```bash
cd web
npm run dev
```

**What you should see:**

```
VITE v5.x.x  ready in xxx ms

➜  Local:   http://localhost:3000/
➜  Network: use --host to expose
```

**✅ Frontend is running when you see:**

- "Local: http://localhost:3000/" message
- No error messages
- Keep this terminal window open too!

---

#### **Step 3: Access the Dashboard**

Open your web browser and visit:

**🌐 http://localhost:3000**

**What you'll see:**

- 📊 Real-time account balance
- 📈 Open positions (if any)
- 🤖 AI decision logs
- 📉 Equity curve chart

**First-time tips:**

- It may take 3-5 minutes for the first AI decision
- Initial decisions might say "观望" (wait) - this is normal
- AI needs to analyze market conditions first

---

## 🔧 Troubleshooting

| Error | Solution |
|-------|----------|
| TA-Lib Not Found | Windows: Install Visual Studio Build Tools<br>macOS: `brew install ta-lib`<br>Linux: Install dev packages |
| Port 8080 in Use | `sudo lsof -i :8080 && kill -9 <PID>` |
| Database Locked | `pkill -f nofx-backend && rm data/trading.db` |
| Invalid API Key | Check format (sk-), permissions, credits |
| Exchange Timeout | Check internet, credentials, exchange status |

**Support:** Check logs → [GitHub Issues](https://github.com/dmgwang/nofx-lite/issues)

---

## 📄 License

The NOFX-Lite project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)** - See [LICENSE](LICENSE) file for details.

**What this means:**

- ✅ You can use, modify, and distribute this software
- ✅ You must disclose source code of your modifications
- ✅ If you run a modified version on a server, you must make the source code available to users
- ✅ All derivatives must also be licensed under AGPL-3.0

For commercial licensing or questions, please contact the maintainers.

---

## ⭐ Star History

[![Star History Chart](https://api.star-history.com/svg?repos=dmgwang/nofx-lite&type=Date)](https://star-history.com/#dmgwang/nofx-lite&Date)
