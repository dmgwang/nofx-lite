package decision

import (
    "encoding/json"
    "fmt"
    "log"
    "nofx-lite/market"
    "nofx-lite/mcp"
    "nofx-lite/pool"
    "regexp"
    "sort"
    "strings"
    "time"
)

// 预编译正则表达式（性能优化：避免每次调用时重新编译）
var (
    // ✅ 安全的正則：精確匹配 ```json 代碼塊
    // 使用反引號 + 拼接避免轉義問題
    reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
    reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
    reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
    reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
    reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

    // More tolerant captures and comment stripping for JSON-like outputs
    reJSONFenceGeneric = regexp.MustCompile(`(?is)` + "```json\\s*(.*?)\\s*```")
    reLineComment      = regexp.MustCompile(`(?m)^\s*(//|#).*?$`)
    reBlockComment     = regexp.MustCompile(`(?s)/\*.*?\*/`)

	// 新增：XML标签提取（支持思维链中包含任何字符）
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // 历史最高收益率（百分比）
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance     interface{}             `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	UseTestnet      bool                    `json:"-"` // 是否使用测试网（从交易所配置读取）
}

// Decision AI的交易决策
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// 开仓参数
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// 调整参数（新增）
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`    // 用于 update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`  // 用于 update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"` // 用于 partial_close (0-100)

	// 通用参数
	Confidence int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning  string  `json:"reasoning"`
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // 系统提示词（发送给AI的系统prompt）
	UserPrompt   string     `json:"user_prompt"`   // 发送给AI的输入prompt
	CoTTrace     string     `json:"cot_trace"`     // 思维链分析（AI输出）
	Decisions    []Decision `json:"decisions"`     // 具体决策列表
	Timestamp    time.Time  `json:"timestamp"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt和模板选择）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, customPrompt, overrideBase, templateName)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
    aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
    if err != nil {
        return nil, fmt.Errorf("AI call failed: %w", err)
    }

	// 4. 解析AI响应
    decision, err := parseFullDecisionResponse(aiResponse, ctx)
    if err != nil {
        return decision, fmt.Errorf("AI response parse failed: %w", err)
    }

	decision.Timestamp = time.Now()
	decision.SystemPrompt = systemPrompt // 保存系统prompt
	decision.UserPrompt = userPrompt     // 保存输入prompt
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
    ctx.MarketDataMap = make(map[string]*market.Data)
    ctx.OITopDataMap = make(map[string]*OITopData)

    // 收集所有需要获取数据的币种
    symbolSet := make(map[string]bool)

    // 1. 优先获取持仓币种的数据（这是必须的）
    for _, pos := range ctx.Positions {
        symbolSet[pos.Symbol] = true
    }

    // 2. 候选币种数量根据账户状态动态调整
    maxCandidates := calculateMaxCandidates(ctx)
    for i, coin := range ctx.CandidateCoins {
        if i >= maxCandidates {
            break
        }
        symbolSet[coin.Symbol] = true
    }

    // 持仓币种集合（用于判断是否跳过OI检查）
    positionSymbols := make(map[string]bool)
    for _, pos := range ctx.Positions {
        positionSymbols[pos.Symbol] = true
    }

    // 先收集数据，再统一做 OI 过滤（避免逐个币种阈值不一致）
    preFilterData := make(map[string]*market.Data)
    for symbol := range symbolSet {
        data, err := market.Get(symbol, ctx.UseTestnet)
        if err != nil {
            // 单个币种失败不影响整体，只记录错误
            continue
        }
        preFilterData[symbol] = data
    }

    // 计算候选币种（非持仓）在百万美元单位下的 OI 价值分布
    oiValuesM := make([]float64, 0, len(preFilterData))
    for symbol, data := range preFilterData {
        if positionSymbols[symbol] {
            continue // 现有持仓不参与阈值计算
        }
        if data.OpenInterest != nil && data.CurrentPrice > 0 {
            oiValue := data.OpenInterest.Latest * data.CurrentPrice
            oiValuesM = append(oiValuesM, oiValue/1_000_000)
        }
    }
    sort.Float64s(oiValuesM)

    // 动态阈值：绝对下限 + 顶部四分位（保留高 OI 币种）
    absFloorM := 8.0 // 绝对下限：8M（更平衡）
    quartileEnabled := len(oiValuesM) >= 8
    quartileThresholdM := absFloorM
    if quartileEnabled {
        idx := int(float64(len(oiValuesM)) * 0.75)
        if idx >= len(oiValuesM) {
            idx = len(oiValuesM) - 1
        }
        if idx < 0 {
            idx = 0
        }
        quartileThresholdM = oiValuesM[idx]
        // 避免阈值过低，至少不低于绝对下限
        if quartileThresholdM < absFloorM {
            quartileThresholdM = absFloorM
        }
    }

    // 应用过滤：保留现有持仓；其余需满足（OI >= 绝对下限）或（OI 属于顶四分位）
    for symbol, data := range preFilterData {
        if positionSymbols[symbol] {
            ctx.MarketDataMap[symbol] = data
            continue
        }
        // 无法获取 OI 的币种保留（避免误杀），由后续策略再考虑
        if data.OpenInterest == nil || data.CurrentPrice <= 0 {
            ctx.MarketDataMap[symbol] = data
            continue
        }

        oiValueM := (data.OpenInterest.Latest * data.CurrentPrice) / 1_000_000
        passAbs := oiValueM >= absFloorM
        passQuartile := quartileEnabled && oiValueM >= quartileThresholdM
        if !(passAbs || passQuartile) {
            log.Printf("⚠️  %s skipped: low OI liquidity (%.2fM < floor %.1fM, below top quartile %.1fM)",
                symbol, oiValueM, absFloorM, quartileThresholdM)
            continue
        }
        ctx.MarketDataMap[symbol] = data
    }

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ 重要：限制候选币种数量，避免 Prompt 过大
	// 根据持仓数量动态调整：持仓越少，可以分析更多候选币
	const (
		maxCandidatesWhenEmpty    = 30 // 无持仓时最多分析30个候选币
		maxCandidatesWhenHolding1 = 25 // 持仓1个时最多分析25个候选币
		maxCandidatesWhenHolding2 = 20 // 持仓2个时最多分析20个候选币
		maxCandidatesWhenHolding3 = 15 // 持仓3个时最多分析15个候选币（避免 Prompt 过大）
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ 持仓
		maxCandidates = maxCandidatesWhenHolding3
	}

	// 返回实际候选币数量和上限中的较小值
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom 构建包含自定义内容的 System Prompt
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string) string {
    log.Printf("📝 buildSystemPromptWithCustom start [template='%s', override=%t, custom_len=%d]",
        templateName, overrideBase, len(customPrompt))

	// 如果覆盖基础prompt且有自定义prompt，只使用自定义prompt
    if overrideBase && customPrompt != "" {
        log.Printf("🎯 Override mode enabled: returning custom prompt only (len=%d)", len(customPrompt))
        return customPrompt
    }

	// 获取基础prompt（使用指定的模板）
    log.Printf("🏗️  Building base system prompt [template='%s']", templateName)
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName)

	// 如果没有自定义prompt，直接返回基础prompt
	if customPrompt == "" {
		log.Printf("✅ 无自定义prompt，直接返回基础提示词（长度：%d字符）", len(basePrompt))
		return basePrompt
	}

	// 添加自定义prompt部分到基础prompt
	log.Printf("🔗 合并基础提示词与自定义提示词")
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 个性化交易策略\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("注意: 以上个性化策略是对基础规则的补充，不能违背基础风险控制原则。\n")

	finalPrompt := sb.String()
	log.Printf("✅ 合并完成，最终提示词长度：%d字符", len(finalPrompt))
	return finalPrompt
}

// buildSystemPrompt 构建 System Prompt（使用模板+动态部分）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string) string {
	var sb strings.Builder

	// 1. 加载提示词模板（核心交易策略部分）
    log.Printf("🔍 Loading system prompt template [requested='%s']", templateName)

	if templateName == "" {
        templateName = "default" // default template
        log.Printf("ℹ️  Empty template name, fallback to 'default'")
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
        // Template not found, fallback to default
        log.Printf("⚠️  Prompt template '%s' not found: %v", templateName, err)
        log.Printf("🔄 Fallback to default template 'default'")

		template, err = GetPromptTemplate("default")
		if err != nil {
            // If default also missing, use a minimal built-in fallback
            log.Printf("❌ Failed to load default template 'default': %v", err)
            log.Printf("🏠 Using built-in minimal fallback")

            // Minimal built-in strategy guidance
            sb.WriteString("You are a professional crypto trading AI. Make trading decisions based on provided market data.\n")
            sb.WriteString("Core principles:\n")
            sb.WriteString("- Strict risk control: per-trade risk ≤ 2%% of equity\n")
            sb.WriteString("- Trade with trend, avoid counter-trend entries\n")
            sb.WriteString("- Set reasonable stop-loss and take-profit\n\n")
		} else {
            log.Printf("✅ Loaded default template 'default'")
            sb.WriteString(template.Content)
            sb.WriteString("\n\n")
        }
    } else {
        log.Printf("✅ Loaded user-specified template '%s'", templateName)
        sb.WriteString(template.Content)
        sb.WriteString("\n\n")
    }

    // 2. Hard constraints (risk control) - concise, structured
    sb.WriteString("# Hard Constraints (Risk Control)\n\n")
    sb.WriteString("1) Risk-Reward: target ≥ 3.0:1.\n")
    sb.WriteString("2) Max positions: 3 symbols.\n")
    sb.WriteString(fmt.Sprintf("3) Position size caps: Alt %.0f–%.0f USDT | BTC/ETH %.0f–%.0f USDT\n",
        accountEquity*0.8, accountEquity*1.5, accountEquity*5, accountEquity*10))
    sb.WriteString(fmt.Sprintf("4) Leverage caps: Alt ≤ %dx | BTC/ETH ≤ %dx (hard limit).\n", altcoinLeverage, btcEthLeverage))
    sb.WriteString("5) Margin usage: total ≤ 90%.\n")
    sb.WriteString("6) Min notional: ≥ 12 USDT (exchange min + safety).\n")
    sb.WriteString("7) Volatility-aware stops: use ATR14-based distances (≥ 1×ATR14).\n")
    sb.WriteString("8) CRITICAL: Stop-loss and take-profit placement:\n")
    sb.WriteString("   - For LONG positions: stop_loss < entry_price < take_profit\n")
    sb.WriteString("   - For SHORT positions: take_profit < entry_price < stop_loss\n")
    sb.WriteString("   - Violating this will cause validation failure!\n\n")

    // 3. Output format (JSON-only, strict)
    sb.WriteString("# Output Format (strict)\n\n")
    sb.WriteString("Return ONLY a single JSON object with key 'decisions'. No extra text.\n")
    sb.WriteString("Example (schema only, not a suggestion):\n")
    sb.WriteString("{\n  \"decisions\": [\n    {\n      \"symbol\": \"BTCUSDT\",\n      \"action\": \"open_long|open_short|close_long|close_short|update_stop_loss|update_take_profit|partial_close|hold|wait\",\n      \"leverage\": <int>,\n      \"position_size_usd\": <number>,\n      \"stop_loss\": <number>,\n      \"take_profit\": <number>,\n      \"new_stop_loss\": <number>,\n      \"new_take_profit\": <number>,\n      \"close_percentage\": <number>,\n      \"confidence\": <int>,\n      \"risk_usd\": <number>,\n      \"reasoning\": \"short rationale in English\"\n    }\n  ]\n}\n\n")
    sb.WriteString("Required fields for opens: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning.\n")

    return sb.String()
}

// PreviewSystemPrompt provides a simple exported helper to build the system prompt
// for a given template name using representative parameters. This is intended for
// tests and preview tooling where we only need the composed prompt content without
// invoking the full decision-making flow.
func PreviewSystemPrompt(templateName string) string {
    // Use fixed sample values; core content comes from the template and fixed sections.
    return buildSystemPrompt(1000 /*account equity*/, 5 /*BTC/ETH leverage*/, 10 /*altcoin leverage*/, templateName)
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("时间: %s | 周期: #%d | 运行: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// 账户
	sb.WriteString(fmt.Sprintf("账户: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓（完整市场数据）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
				}
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f 当前价%.4f | 盈亏%+.2f%% | 盈亏金额%+.2f USDT | 最高收益率%.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// 使用FormatMarketData输出完整市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("当前持仓: 无\n\n")
	}

	// 候选币种（完整市场数据）
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top双重信号)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top持仓增长)"
		}

		// 使用FormatMarketData输出完整市场数据
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// 夏普比率（直接传值，不要复杂格式化）
	if ctx.Performance != nil {
		// 直接从interface{}中提取SharpeRatio
		type PerformanceData struct {
			SharpeRatio float64 `json:"sharpe_ratio"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("## 📊 夏普比率: %.2f\n\n", perfData.SharpeRatio))
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, ctx *Context) (*FullDecision, error) {
    // Parse decisions JSON
    decisions, err := extractDecisionsStrict(aiResponse)
    // Extract chain-of-thought reasoning (non-JSON) for display/debug
    cot := strings.TrimSpace(extractCoTTrace(aiResponse))
    if err != nil {
        return &FullDecision{CoTTrace: cot, Decisions: []Decision{}}, fmt.Errorf("strict JSON parsing failed: %w", err)
    }

    // Validate decisions against risk constraints
    if err := validateDecisions(decisions, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx); err != nil {
        return &FullDecision{CoTTrace: cot, Decisions: decisions}, fmt.Errorf("decision validation failed: %w", err)
    }

    return &FullDecision{CoTTrace: cot, Decisions: decisions}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
    // First: explicit reasoning tag
    s := strings.TrimSpace(removeInvisibleRunes(response))
    if m := reReasoningTag.FindStringSubmatch(s); len(m) >= 2 {
        return strings.TrimSpace(m[1])
    }

    // Try to parse JSON wrapper and extract CoT as JSON (object/array) or string
    coTKeys := []string{"cot_trace", "chain_of_thought", "cot", "reasoning", "analysis"}

    // Prefer fenced JSON content
    if m := reJSONFenceGeneric.FindStringSubmatch(s); len(m) >= 2 {
        inner := strings.TrimSpace(stripJSONComments(m[1]))
        if cot := extractCoTFromJSONObject(inner, coTKeys); cot != "" {
            return cot
        }
    }

    // Next: sanitized JSON (may be object containing decisions)
    inner := sanitizeModelResponse(response)
    if cot := extractCoTFromJSONObject(inner, coTKeys); cot != "" {
        return cot
    }

    // Fallback: scan raw response for an object containing any CoT keys
    if obj := findObjectWithAnyKey(s, coTKeys); obj != "" {
        if cot := extractCoTFromJSONObject(obj, coTKeys); cot != "" {
            return cot
        }
    }

    // Last resort: capture prose around JSON decision blocks
    fenceIdx := strings.Index(s, "```json")
    decTagIdx := strings.Index(s, "<decision>")
    cutoff := len(s)
    if fenceIdx >= 0 && fenceIdx < cutoff {
        cutoff = fenceIdx
    }
    if decTagIdx >= 0 && decTagIdx < cutoff {
        cutoff = decTagIdx
    }
    head := strings.TrimSpace(s[:cutoff])
    if head != "" {
        return head
    }
    if fenceIdx >= 0 {
        tail := strings.TrimSpace(s[fenceIdx+len("```json"):])
        return tail
    }
    if decTagIdx >= 0 {
        tail := strings.TrimSpace(s[decTagIdx+len("<decision>"):])
        return tail
    }
    return ""
}

// extractCoTFromJSONObject attempts to parse 's' as a JSON object and returns a pretty-printed
// JSON string or a plain string value from common CoT keys. Returns empty string if not found.
func extractCoTFromJSONObject(s string, keys []string) string {
    s = strings.TrimSpace(s)
    if s == "" || !strings.HasPrefix(s, "{") {
        return ""
    }
    var m map[string]interface{}
    if err := json.Unmarshal([]byte(s), &m); err != nil {
        return ""
    }
    // Normalize keys (lowercase compare)
    for k, v := range m {
        lk := strings.ToLower(k)
        for _, target := range keys {
            if lk == target {
                switch vv := v.(type) {
                case string:
                    return strings.TrimSpace(vv)
                default:
                    b, err := json.MarshalIndent(v, "", "  ")
                    if err == nil {
                        return string(b)
                    }
                }
            }
        }
    }
    return ""
}

// findObjectWithAnyKey locates an object substring containing any of the provided keys.
// It searches for '"key"' and slices to the nearest enclosing balanced object.
func findObjectWithAnyKey(s string, keys []string) string {
    idx := -1
    for _, k := range keys {
        i := strings.Index(s, "\""+k+"\"")
        if i >= 0 && (idx < 0 || i < idx) {
            idx = i
        }
    }
    if idx < 0 {
        return ""
    }
    // find nearest '{' before idx
    start := -1
    for i := idx; i >= 0; i-- {
        if s[i] == '{' {
            start = i
            break
        }
    }
    if start < 0 {
        return ""
    }
    end := findMatchingBrace(s, start)
    if end > start {
        return s[start : end+1]
    }
    return ""
}

// extractDecisions 提取JSON决策列表
func extractDecisionsStrict(response string) ([]Decision, error) {
    // Sanitize and try to parse object first
    s := sanitizeModelResponse(response)

    // Try strict object with 'decisions' key
    var wrapper struct {
        Decisions []Decision `json:"decisions"`
    }
    if err := json.Unmarshal([]byte(s), &wrapper); err == nil && len(wrapper.Decisions) > 0 {
        return wrapper.Decisions, nil
    }

    // If not parsed, try to find an object containing the 'decisions' key from raw response
    if obj := findDecisionsObjectSubstring(response); obj != "" {
        var w2 struct {
            Decisions []Decision `json:"decisions"`
        }
        if err := json.Unmarshal([]byte(obj), &w2); err == nil && len(w2.Decisions) > 0 {
            return w2.Decisions, nil
        }
    }

    // Fallback: parse bare array of decisions
    var arr []Decision
    if err := json.Unmarshal([]byte(s), &arr); err == nil && len(arr) > 0 {
        return arr, nil
    }

    // Last attempt: find an array substring and parse
    idx := strings.Index(s, "[")
    if idx >= 0 {
        end := findMatchingBracket(s, idx)
        if end > idx {
            sub := s[idx : end+1]
            if err := json.Unmarshal([]byte(sub), &arr); err == nil && len(arr) > 0 {
                return arr, nil
            }
        }
    }

    // Unable to parse
    preview := s
    if len(preview) > 160 {
        preview = preview[:160] + "..."
    }
    return nil, fmt.Errorf("expected JSON object with 'decisions' or bare array; got unparsable content (preview): %s", preview)
}

// fixMissingQuotes 替换中文引号和全角字符为英文引号和半角字符（避免AI输出全角JSON字符导致解析失败）
func fixMissingQuotes(jsonStr string) string {
	// 替换中文引号
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ 替换全角括号、冒号、逗号（防止AI输出全角JSON字符）
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B 全角左方括号
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D 全角右方括号
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B 全角左花括号
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D 全角右花括号
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A 全角冒号
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C 全角逗号

	// ⚠️ 替换CJK标点符号（AI在中文上下文中也可能输出这些）
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK左方头括号 U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK右方头括号 U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK左龟壳括号 U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK右龟壳括号 U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK顿号 U+3001

	// ⚠️ 替换全角空格为半角空格（JSON中不应该有全角空格）
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 全角空格

	return jsonStr
}

// validateJSONFormat 验证 JSON 格式，检测常见错误
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	if !reArrayHead.MatchString(trimmed) {
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("不是有效的决策数组（必须包含对象 {}），实际内容: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON 必须以 [{ 开头（允许空白），实际: %s", trimmed[:min(20, len(trimmed))])
	}

	// 检查是否包含范围符号 ~（LLM 常见错误）
	if strings.Contains(jsonStr, "~") {
		outsideQuotes := true
		for i, ch := range jsonStr {
			if ch == '"' && (i == 0 || jsonStr[i-1] != '\\') {
				outsideQuotes = !outsideQuotes
			} else if ch == '~' && outsideQuotes {
				return fmt.Errorf("JSON 中不可包含范围符号 ~，所有数字必须是精确的单一值")
			}
		}
	}

	// 检查是否包含千位分隔符（如 98,000）
	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON 数字不可包含千位分隔符逗号，发现: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes 去除零宽字符和 BOM，避免肉眼看不见的前缀破坏校验
func removeInvisibleRunes(s string) string {
    return reInvisibleRunes.ReplaceAllString(s, "")
}

// stripJSONComments removes common comment syntaxes that models sometimes include
// in JSON-like blocks. It strips line comments starting with '//' or '#', and
// block comments delimited by /* */. This function does not attempt to detect
// comments inside quoted strings; callers should provide JSON-like content.
func stripJSONComments(s string) string {
    s = reBlockComment.ReplaceAllString(s, "")
    s = reLineComment.ReplaceAllString(s, "")
    return s
}

// findMatchingBrace finds the closing '}' that matches the '{' at index start.
// It accounts for nested braces and ignores braces that appear inside quoted strings.
func findMatchingBrace(s string, start int) int {
    if start >= len(s) || s[start] != '{' {
        return -1
    }
    depth := 0
    inString := false
    for i := start; i < len(s); i++ {
        ch := s[i]
        if ch == '"' && (i == 0 || s[i-1] != '\\') {
            inString = !inString
            continue
        }
        if inString {
            continue
        }
        switch ch {
        case '{':
            depth++
        case '}':
            depth--
            if depth == 0 {
                return i
            }
        }
    }
    return -1
}

// sanitizeModelResponse extracts a JSON object or array from a possibly noisy
// model response. Preference order:
// 1) Content inside ```json ... ``` fences
// 2) Content inside <decision>...</decision> tags
// 3) Substring starting with the first '{' (object) or '[' (array), trimmed to a balanced ending
// It also removes invisible runes, fixes quotes, and strips comments.
func sanitizeModelResponse(response string) string {
    s := removeInvisibleRunes(strings.TrimSpace(response))
    s = fixMissingQuotes(s)

    // Prefer generic JSON fence first
    if m := reJSONFenceGeneric.FindStringSubmatch(s); len(m) >= 2 {
        inner := strings.TrimSpace(m[1])
        inner = stripJSONComments(inner)
        return inner
    }

    // Next: decision XML tag
    if m := reDecisionTag.FindStringSubmatch(s); len(m) >= 2 {
        inner := strings.TrimSpace(m[1])
        inner = stripJSONComments(inner)
        return inner
    }

    // Prefer object containing "decisions" key anywhere in the text
    if obj := findDecisionsObjectSubstring(s); obj != "" {
        return stripJSONComments(strings.TrimSpace(obj))
    }

    // Fallback: find first JSON-looking start
    idxObj := strings.Index(s, "{")
    idxArr := strings.Index(s, "[")
    start := -1
    isObj := false
    if idxObj >= 0 && (idxArr < 0 || idxObj < idxArr) {
        start = idxObj
        isObj = true
    } else if idxArr >= 0 {
        start = idxArr
        isObj = false
    }

    if start >= 0 {
        var end int
        if isObj {
            end = findMatchingBrace(s, start)
        } else {
            end = findMatchingBracket(s, start)
        }
        // If we found a balanced end, trim; else take from start to end of string
        if end > start {
            s = s[start : end+1]
        } else {
            s = s[start:]
        }
    }

    s = stripJSONComments(s)
    return strings.TrimSpace(s)
}

// findDecisionsObjectSubstring locates a JSON object containing the "decisions" key
// by searching backwards to the nearest '{' before the key and trimming to the
// matching closing '}'. Returns empty string if not found or not balanced.
func findDecisionsObjectSubstring(s string) string {
    idx := strings.Index(s, "\"decisions\"")
    if idx < 0 {
        return ""
    }
    // find nearest '{' before idx
    start := -1
    for i := idx; i >= 0; i-- {
        if s[i] == '{' {
            start = i
            break
        }
    }
    if start < 0 {
        return ""
    }
    end := findMatchingBrace(s, start)
    if end > start {
        return s[start : end+1]
    }
    return ""
}

// compactArrayOpen 规整开头的 "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, ctx *Context) error {
    for i, decision := range decisions {
        if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage, ctx); err != nil {
            return fmt.Errorf("Decision #%d failed validation: %w", i+1, err)
        }
    }
    return nil
}

// findMatchingBracket 查找匹配的右括号
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, ctx *Context) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"update_stop_loss":   true,
		"update_take_profit": true,
		"partial_close":      true,
		"hold":               true,
		"wait":               true,
	}

    if !validActions[d.Action] {
        return fmt.Errorf("invalid action: %s", d.Action)
    }

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 根据币种使用配置的杠杆上限
		maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
		maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

        if d.Leverage <= 0 || d.Leverage > maxLeverage {
            return fmt.Errorf("leverage must be within 1-%d (%s, config cap %dx): %d", maxLeverage, d.Symbol, maxLeverage, d.Leverage)
        }
        if d.PositionSizeUSD <= 0 {
            return fmt.Errorf("position_size_usd must be > 0: %.2f", d.PositionSizeUSD)
        }

		// ✅ 验证最小开仓金额（防止数量格式化为 0 的错误）
		// Binance 最小名义价值 10 USDT + 安全边际
		const minPositionSizeGeneral = 12.0 // 10 + 20% 安全边际
		const minPositionSizeBTCETH = 60.0  // BTC/ETH 因价格高和精度限制需要更大金额（更灵活）

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
            if d.PositionSizeUSD < minPositionSizeBTCETH {
                return fmt.Errorf("%s position_size_usd too small (%.2f), must be ≥ %.2f USDT", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
            }
		} else {
            if d.PositionSizeUSD < minPositionSizeGeneral {
                return fmt.Errorf("position_size_usd too small (%.2f), must be ≥ %.2f USDT (exchange min notional)", d.PositionSizeUSD, minPositionSizeGeneral)
            }
		}

		// 验证仓位价值上限（加1%容差以避免浮点数精度问题）
		tolerance := maxPositionValue * 0.01 // 1%容差
        if d.PositionSizeUSD > maxPositionValue+tolerance {
            if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
                return fmt.Errorf("BTC/ETH position notional cannot exceed %.0f USDT (10x equity), got %.0f", maxPositionValue, d.PositionSizeUSD)
            } else {
                return fmt.Errorf("Altcoin position notional cannot exceed %.0f USDT (1.5x equity), got %.0f", maxPositionValue, d.PositionSizeUSD)
            }
        }
        if d.StopLoss <= 0 || d.TakeProfit <= 0 {
            return fmt.Errorf("stop_loss and take_profit must be > 0")
        }

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
            if d.StopLoss >= d.TakeProfit {
                return fmt.Errorf("for long, stop_loss must be less than take_profit")
            }
        } else {
            if d.StopLoss <= d.TakeProfit {
                return fmt.Errorf("for short, stop_loss must be greater than take_profit")
            }
        }

        // Compute synthetic entry price between stop and take levels
        var entryPrice float64
        if d.Action == "open_long" {
            entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2
        } else {
            entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2
        }

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

        // ATR14-aware minimum distances and dynamic RRR
        var atr14, price float64
        if ctx != nil && ctx.MarketDataMap != nil {
            if md, ok := ctx.MarketDataMap[d.Symbol]; ok && md != nil {
                price = md.CurrentPrice
                if md.LongerTermContext != nil {
                    atr14 = md.LongerTermContext.ATR14
                }
            }
        }
        // Enforce minimum SL/TP distance if ATR available
        minATRMultiple := 1.0
        if atr14 > 0 {
            minDist := minATRMultiple * atr14
            if d.Action == "open_long" {
                if (entryPrice-d.StopLoss) < minDist || (d.TakeProfit-entryPrice) < minDist {
                    return fmt.Errorf("SL/TP distances must be ≥ %.2f (≥ %.1fx ATR14)", minDist, minATRMultiple)
                }
            } else {
                if (d.StopLoss-entryPrice) < minDist || (entryPrice-d.TakeProfit) < minDist {
                    return fmt.Errorf("SL/TP distances must be ≥ %.2f (≥ %.1fx ATR14)", minDist, minATRMultiple)
                }
            }
        }

        // Dynamic risk-reward ratio threshold by volatility
        rrrMin := 3.0
        if atr14 > 0 && price > 0 {
            vol := atr14 / price
            if vol < 0.01 {
                rrrMin = 2.5
            } else if vol >= 0.02 {
                rrrMin = 3.5
            }
        }
        if riskRewardRatio < rrrMin {
            return fmt.Errorf("risk-reward ratio too low (%.2f:1), required ≥ %.1f:1 [risk=%.2f%% reward=%.2f%%] [sl=%.2f tp=%.2f]",
                riskRewardRatio, rrrMin, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
        }

        // If provided, ensure RiskUSD bounds per-trade risk
        if d.RiskUSD > 0 {
            estimatedRiskUSD := (riskPercent / 100.0) * d.PositionSizeUSD
            if estimatedRiskUSD > d.RiskUSD {
                return fmt.Errorf("estimated risk (%.2f USDT) exceeds risk_usd budget (%.2f USDT)", estimatedRiskUSD, d.RiskUSD)
            }
        }
    }

	// 动态调整止损验证
	if d.Action == "update_stop_loss" {
        if d.NewStopLoss <= 0 {
            return fmt.Errorf("new_stop_loss must be > 0: %.2f", d.NewStopLoss)
        }
    }

	// 动态调整止盈验证
	if d.Action == "update_take_profit" {
        if d.NewTakeProfit <= 0 {
            return fmt.Errorf("new_take_profit must be > 0: %.2f", d.NewTakeProfit)
        }
    }

	// 部分平仓验证
	if d.Action == "partial_close" {
        if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
            return fmt.Errorf("close_percentage must be in (0,100]: %.1f", d.ClosePercentage)
        }
    }

	return nil
}
