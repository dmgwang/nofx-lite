package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// getBaseURL 根据是否使用测试网返回对应的基础URL
func getBaseURL(testnet bool) string {
	if testnet {
		return "https://testnet.binancefuture.com"
	}
	return "https://fapi.binance.com"
}

// FundingRateCache 资金费率缓存结构
// Binance Funding Rate 每 8 小时才更新一次，使用 1 小时缓存可显著减少 API 调用
type FundingRateCache struct {
	Rate      float64
	UpdatedAt time.Time
}

var (
	fundingRateMap sync.Map // map[string]*FundingRateCache
	frCacheTTL     = 1 * time.Hour
)

// Get 获取指定代币的市场数据
func Get(symbol string, testnet ...bool) (*Data, error) {
	// 检查是否使用测试网，默认为false
	useTestnet := false
	if len(testnet) > 0 {
		useTestnet = testnet[0]
	}
	var klines3m, klines4h []Kline
	var err error
	// 标准化symbol
	symbol = Normalize(symbol)
	// 获取3分钟K线数据 (最近10个)
	klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m") // 多获取一些用于计算
	if err != nil {
		return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
	}

	// 获取4小时K线数据 (最近10个)
	klines4h, err = WSMonitorCli.GetCurrentKlines(symbol, "4h") // 多获取用于计算指标
	if err != nil {
		return nil, fmt.Errorf("获取4小时K线失败: %v", err)
	}

	// 检查数据是否为空
	if len(klines3m) == 0 {
		return nil, fmt.Errorf("3分钟K线数据为空")
	}
	if len(klines4h) == 0 {
		return nil, fmt.Errorf("4小时K线数据为空")
	}

	// 计算当前指标 (基于3分钟最新数据)
	currentPrice := klines3m[len(klines3m)-1].Close
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD := calculateMACD(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// 计算价格变化百分比
	// 1小时价格变化 = 20个3分钟K线前的价格
	priceChange1h := 0.0
	if len(klines3m) >= 21 { // 至少需要21根K线 (当前 + 20根前)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4小时价格变化 = 1个4小时K线前的价格
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 获取OI数据
	oiData, err := GetOpenInterestData(symbol, useTestnet)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: 0, Average: 0}
	}

	// 获取Funding Rate
	fundingRate, _ := GetFundingRate(symbol, useTestnet)

	// 获取深度数据 (获取10档深度数据)
	depthData, err := GetDepthData(symbol, useTestnet)
	if err != nil {
		// 深度数据获取失败不影响整体，记录错误并继续
		log.Printf("获取深度数据失败: %v", err)
		depthData = nil
	}

	// 计算日内系列数据
	intradayData := calculateIntradaySeries(klines3m)

	// 计算长期数据
	longerTermData := calculateLongerTermData(klines4h)

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		DepthData:         depthData,
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// 计算12期和26期EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateIntradaySeries 计算日内系列数据
func calculateIntradaySeries(klines []Kline) *IntradayData {
	data := &IntradayData{
		MidPrices:   make([]float64, 0, 10),
		EMA20Values: make([]float64, 0, 10),
		MACDValues:  make([]float64, 0, 10),
		RSI7Values:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
	}

	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	return data
}

// calculateLongerTermData 计算长期数据
func calculateLongerTermData(klines []Kline) *LongerTermData {
	data := &LongerTermData{
		MACDValues:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	data.ATR3 = calculateATR(klines, 3)
	data.ATR14 = calculateATR(klines, 14)

	// 计算成交量
	if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		// 计算平均成交量
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(klines))
	}

	// 计算MACD和RSI序列
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	return data
}

// GetOpenInterestData 获取持仓数据（导出供测试使用）
func GetOpenInterestData(symbol string, testnet bool) (*OIData, error) {
	// 使用统一的APIClient
	apiClient := NewAPIClient()
	
	// 获取基础URL（复用api_client.go中的常量）
	baseURL := getBaseURL(testnet)
	
	// 构建请求URL
	url := fmt.Sprintf("%s/fapi/v1/openInterest?symbol=%s", baseURL, symbol)
	
	// 使用统一的HTTP客户端进行请求
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get open interest data: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	oi, err := strconv.ParseFloat(result.OpenInterest, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse open interest: %w", err)
	}

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // 近似平均值
	}, nil
}

// GetFundingRate 获取资金费率（导出供测试使用）
func GetFundingRate(symbol string, testnet bool) (float64, error) {
	// 检查缓存（有效期 1 小时）
	// Funding Rate 每 8 小时才更新，1 小时缓存非常合理
	if cached, ok := fundingRateMap.Load(symbol); ok {
		cache := cached.(*FundingRateCache)
		if time.Since(cache.UpdatedAt) < frCacheTTL {
			// 缓存命中，直接返回
			return cache.Rate, nil
		}
	}

	// 使用统一的APIClient
	apiClient := NewAPIClient()
	
	// 获取基础URL（复用api_client.go中的常量）
	baseURL := getBaseURL(testnet)
	
	// 构建请求URL
	url := fmt.Sprintf("%s/fapi/v1/premiumIndex?symbol=%s", baseURL, symbol)
	
	// 使用统一的HTTP客户端进行请求
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to get funding rate: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	rate, err := strconv.ParseFloat(result.LastFundingRate, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse funding rate: %w", err)
	}

	// 更新缓存
	fundingRateMap.Store(symbol, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})

	return rate, nil
}

// Format 格式化输出市场数据
func Format(data *Data) string {
    var sb strings.Builder

    // Compact core metrics only: trend (EMA20), momentum (MACD), volatility (ATR14), RSI14
    priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)
    rsi14 := 0.0
    if data.IntradaySeries != nil && len(data.IntradaySeries.RSI14Values) > 0 {
        rsi14 = data.IntradaySeries.RSI14Values[len(data.IntradaySeries.RSI14Values)-1]
    }
    atr14 := 0.0
    if data.LongerTermContext != nil {
        atr14 = data.LongerTermContext.ATR14
    }

    sb.WriteString(fmt.Sprintf(
        "price=%s | ema20=%.3f | macd=%.3f | rsi14=%.3f | atr14=%.3f\n",
        priceStr, data.CurrentEMA20, data.CurrentMACD, rsi14, atr14,
    ))

    // Minimal OI and funding summary only if available
    if data.OpenInterest != nil || data.FundingRate != 0 {
        sb.WriteString("metrics: ")
        if data.OpenInterest != nil {
            oiLatestStr := formatPriceWithDynamicPrecision(data.OpenInterest.Latest)
            oiAverageStr := formatPriceWithDynamicPrecision(data.OpenInterest.Average)
            sb.WriteString(fmt.Sprintf("oi_latest=%s oi_avg=%s ", oiLatestStr, oiAverageStr))
        }
        sb.WriteString(fmt.Sprintf("funding=%.2e\n", data.FundingRate))
    }

    return sb.String()
}

// formatPriceWithDynamicPrecision 根据价格区间动态选择精度
// 这样可以完美支持从超低价 meme coin (< 0.0001) 到 BTC/ETH 的所有币种
func formatPriceWithDynamicPrecision(price float64) string {
	switch {
	case price < 0.0001:
		// 超低价 meme coin: 1000SATS, 1000WHY, DOGS
		// 0.00002070 → "0.00002070" (8位小数)
		return fmt.Sprintf("%.8f", price)
	case price < 0.001:
		// 低价 meme coin: NEIRO, HMSTR, HOT, NOT
		// 0.00015060 → "0.000151" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 0.01:
		// 中低价币: PEPE, SHIB, MEME
		// 0.00556800 → "0.005568" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 1.0:
		// 低价币: ASTER, DOGE, ADA, TRX
		// 0.9954 → "0.9954" (4位小数)
		return fmt.Sprintf("%.4f", price)
	case price < 100:
		// 中价币: SOL, AVAX, LINK, MATIC
		// 23.4567 → "23.4567" (4位小数)
		return fmt.Sprintf("%.4f", price)
	default:
		// 高价币: BTC, ETH (节省 Token)
		// 45678.9123 → "45678.91" (2位小数)
		return fmt.Sprintf("%.2f", price)
	}
}

// formatFloatSlice 格式化float64切片为字符串（使用动态精度）
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = formatPriceWithDynamicPrecision(v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// GetDepthData 获取深度数据
func GetDepthData(symbol string, testnet bool) (*DepthData, error) {
	// 使用统一的APIClient
	apiClient := NewAPIClient()
	
	// 获取10档深度数据 (平衡数据完整性和API调用效率)
	depthData, err := apiClient.GetOrderBookData(symbol, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get depth data: %w", err)
	}
	
	return depthData, nil
}

// AnalyzeDepthData 分析深度数据，返回分析结果
func AnalyzeDepthData(depthData *DepthData) *DepthAnalysis {
	if depthData == nil || len(depthData.Bids) == 0 || len(depthData.Asks) == 0 {
		return nil
	}

	analysis := &DepthAnalysis{
		Symbol:           depthData.Symbol,
		Timestamp:        time.Now(),
		BidDepth:         0,
		AskDepth:         0,
		BidAskRatio:      0,
		LargeBidOrders:   0,
		LargeAskOrders:   0,
		SupportLevels:    []float64{},
		ResistanceLevels: []float64{},
		LiquidityScore:   0,
		MarketSentiment:  "neutral",
	}

	// 计算买盘和卖盘总深度
	var bidQuantities []float64
	var askQuantities []float64

	for _, bid := range depthData.Bids {
		analysis.BidDepth += bid.Price * bid.Quantity
		bidQuantities = append(bidQuantities, bid.Quantity)
	}

	for _, ask := range depthData.Asks {
		analysis.AskDepth += ask.Price * ask.Quantity
		askQuantities = append(askQuantities, ask.Quantity)
	}

	// 计算买卖盘比例
	if analysis.AskDepth > 0 {
		analysis.BidAskRatio = analysis.BidDepth / analysis.AskDepth
	}

	// 计算平均数量，用于识别大单
	avgBidQuantity := 0.0
	if len(bidQuantities) > 0 {
		sum := 0.0
		for _, q := range bidQuantities {
			sum += q
		}
		avgBidQuantity = sum / float64(len(bidQuantities))
	}

	avgAskQuantity := 0.0
	if len(askQuantities) > 0 {
		sum := 0.0
		for _, q := range askQuantities {
			sum += q
		}
		avgAskQuantity = sum / float64(len(askQuantities))
	}

	// 识别大订单 (数量 > 平均值 * 2)
	largeBidThreshold := avgBidQuantity * 2
	largeAskThreshold := avgAskQuantity * 2

	for _, bid := range depthData.Bids {
		if bid.Quantity > largeBidThreshold {
			analysis.LargeBidOrders++
		}
	}

	for _, ask := range depthData.Asks {
		if ask.Quantity > largeAskThreshold {
			analysis.LargeAskOrders++
		}
	}

	// 识别支撑和阻力位
	// 支撑位：买盘密集区域 (连续3个档位数量递增)
	for i := 0; i < len(depthData.Bids)-2; i++ {
		if depthData.Bids[i].Quantity < depthData.Bids[i+1].Quantity &&
			depthData.Bids[i+1].Quantity < depthData.Bids[i+2].Quantity {
			analysis.SupportLevels = append(analysis.SupportLevels, depthData.Bids[i].Price)
		}
	}

	// 阻力位：卖盘密集区域 (连续3个档位数量递增)
	for i := 0; i < len(depthData.Asks)-2; i++ {
		if depthData.Asks[i].Quantity < depthData.Asks[i+1].Quantity &&
			depthData.Asks[i+1].Quantity < depthData.Asks[i+2].Quantity {
			analysis.ResistanceLevels = append(analysis.ResistanceLevels, depthData.Asks[i].Price)
		}
	}

	// 如果没有找到支撑/阻力位，使用简单的阈值判断
	if len(analysis.SupportLevels) == 0 {
		for _, bid := range depthData.Bids {
			if bid.Quantity > avgBidQuantity * 1.5 { // 数量超过平均值1.5倍
				analysis.SupportLevels = append(analysis.SupportLevels, bid.Price)
			}
		}
	}

	if len(analysis.ResistanceLevels) == 0 {
		for _, ask := range depthData.Asks {
			if ask.Quantity > avgAskQuantity * 1.5 { // 数量超过平均值1.5倍
				analysis.ResistanceLevels = append(analysis.ResistanceLevels, ask.Price)
			}
		}
	}

	// 计算流动性评分 (0-100)
	// 基于：1. 总深度 2. 买卖盘平衡性 3. 价差大小
	liquidityScore := 0.0

	// 1. 总深度评分 (40分)
	totalDepth := analysis.BidDepth + analysis.AskDepth
	if totalDepth > 1000000 { // 深度很好
		liquidityScore += 40
	} else if totalDepth > 100000 { // 深度中等
		liquidityScore += 25
	} else if totalDepth > 10000 { // 深度一般
		liquidityScore += 15
	} else if totalDepth > 1000 { // 深度较差
		liquidityScore += 5
	} else { // 深度很差
		liquidityScore += 0
	}

	// 2. 买卖盘平衡性评分 (30分)
	if analysis.BidAskRatio >= 0.8 && analysis.BidAskRatio <= 1.2 {
		liquidityScore += 30 // 非常平衡
	} else if analysis.BidAskRatio >= 0.6 && analysis.BidAskRatio <= 1.4 {
		liquidityScore += 20 // 比较平衡
	} else if analysis.BidAskRatio >= 0.4 && analysis.BidAskRatio <= 2.5 {
		liquidityScore += 10 // 不太平衡
	} else {
		liquidityScore += 5 // 很不平衡
	}

	// 3. 价差评分 (30分)
	if depthData.Spread > 0 {
		spreadPercentage := (depthData.Spread / depthData.MidPrice) * 100
		if spreadPercentage < 0.1 { // 价差极小
			liquidityScore += 30
		} else if spreadPercentage < 0.5 { // 价差小
			liquidityScore += 20
		} else if spreadPercentage < 1.0 { // 价差中等
			liquidityScore += 10
		} else if spreadPercentage < 5.0 { // 价差较大
			liquidityScore += 5
		} else { // 价差很大
			liquidityScore += 0
		}
	}

	analysis.LiquidityScore = math.Min(100, liquidityScore)

	// 判断市场情绪 - 放宽条件以提高准确性
	if analysis.BidAskRatio > 1.2 && analysis.LargeBidOrders >= analysis.LargeAskOrders {
		analysis.MarketSentiment = "bullish"
	} else if analysis.BidAskRatio < 0.8 && analysis.LargeAskOrders >= analysis.LargeBidOrders {
		analysis.MarketSentiment = "bearish"
	} else {
		analysis.MarketSentiment = "neutral"
	}

	return analysis
}
