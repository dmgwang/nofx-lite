package market

import "time"

// Data 市场数据结构
type Data struct {
	Symbol            string
	CurrentPrice      float64
	PriceChange1h     float64 // 1小时价格变化百分比
	PriceChange4h     float64 // 4小时价格变化百分比
	CurrentEMA20      float64
	CurrentMACD       float64
	CurrentRSI7       float64
	OpenInterest      *OIData
	FundingRate       float64
	DepthData         *DepthData // 深度数据
	IntradaySeries    *IntradayData
	LongerTermContext *LongerTermData
}

// OIData Open Interest数据
type OIData struct {
	Latest  float64
	Average float64
}

// DepthData 深度数据
// 包含买卖盘信息，用于分析市场流动性和情绪
// Bid/Ask 数据按照价格从高到低排序
// 0索引通常是最佳买卖价格
// 注意: 价格可能为0，表示该档位没有挂单
// 数量可能为0，表示该档位没有挂单
// 深度数据通常用于分析市场流动性和支撑阻力位
type DepthData struct {
	Symbol     string    `json:"symbol"`     // 交易对
	Timestamp  time.Time `json:"timestamp"`  // 数据时间戳
	LastUpdate time.Time `json:"last_update"` // 最后更新时间
	Bids       []DepthLevel `json:"bids"`     // 买盘 [价格, 数量] 按价格降序排列
	Asks       []DepthLevel `json:"asks"`     // 卖盘 [价格, 数量] 按价格升序排列
	Spread     float64   `json:"spread"`     // 买卖价差 (ask0 - bid0)
	MidPrice   float64   `json:"mid_price"`  // 中间价 (bid0 + ask0) / 2
}

// DepthLevel 深度档位数据
type DepthLevel struct {
	Price    float64 `json:"price"`    // 价格
	Quantity float64 `json:"quantity"` // 数量
}

// DepthAnalysis 深度数据分析结果
type DepthAnalysis struct {
	Symbol            string    `json:"symbol"`
	Timestamp         time.Time `json:"timestamp"`
	BidDepth          float64   `json:"bid_depth"`           // 买盘总深度
	AskDepth          float64   `json:"ask_depth"`           // 卖盘总深度
	BidAskRatio       float64   `json:"bid_ask_ratio"`       // 买卖盘比例
	LargeBidOrders    int       `json:"large_bid_orders"`    // 大买单数量 (>平均数量*2)
	LargeAskOrders    int       `json:"large_ask_orders"`    // 大卖单数量
	SupportLevels     []float64 `json:"support_levels"`      // 支撑位 (买盘密集区域)
	ResistanceLevels  []float64 `json:"resistance_levels"`   // 阻力位 (卖盘密集区域)
	LiquidityScore    float64   `json:"liquidity_score"`     // 流动性评分 (0-100)
	MarketSentiment   string    `json:"market_sentiment"`      // 市场情绪: "bullish", "bearish", "neutral"
}

// IntradayData 日内数据(3分钟间隔)
type IntradayData struct {
	MidPrices   []float64
	EMA20Values []float64
	MACDValues  []float64
	RSI7Values  []float64
	RSI14Values []float64
}

// LongerTermData 长期数据(4小时时间框架)
type LongerTermData struct {
	EMA20         float64
	EMA50         float64
	ATR3          float64
	ATR14         float64
	CurrentVolume float64
	AverageVolume float64
	MACDValues    []float64
	RSI14Values   []float64
}

// Binance API 响应结构
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol            string `json:"symbol"`
	Status            string `json:"status"`
	BaseAsset         string `json:"baseAsset"`
	QuoteAsset        string `json:"quoteAsset"`
	ContractType      string `json:"contractType"`
	PricePrecision    int    `json:"pricePrecision"`
	QuantityPrecision int    `json:"quantityPrecision"`
}

type Kline struct {
	OpenTime            int64   `json:"openTime"`
	Open                float64 `json:"open"`
	High                float64 `json:"high"`
	Low                 float64 `json:"low"`
	Close               float64 `json:"close"`
	Volume              float64 `json:"volume"`
	CloseTime           int64   `json:"closeTime"`
	QuoteVolume         float64 `json:"quoteVolume"`
	Trades              int     `json:"trades"`
	TakerBuyBaseVolume  float64 `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume float64 `json:"takerBuyQuoteVolume"`
}

type KlineResponse []interface{}

type PriceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Ticker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
}

// 特征数据结构
type SymbolFeatures struct {
	Symbol           string    `json:"symbol"`
	Timestamp        time.Time `json:"timestamp"`
	Price            float64   `json:"price"`
	PriceChange15Min float64   `json:"price_change_15min"`
	PriceChange1H    float64   `json:"price_change_1h"`
	PriceChange4H    float64   `json:"price_change_4h"`
	Volume           float64   `json:"volume"`
	VolumeRatio5     float64   `json:"volume_ratio_5"`
	VolumeRatio20    float64   `json:"volume_ratio_20"`
	VolumeTrend      float64   `json:"volume_trend"`
	RSI14            float64   `json:"rsi_14"`
	SMA5             float64   `json:"sma_5"`
	SMA10            float64   `json:"sma_10"`
	SMA20            float64   `json:"sma_20"`
	HighLowRatio     float64   `json:"high_low_ratio"`
	Volatility20     float64   `json:"volatility_20"`
	PositionInRange  float64   `json:"position_in_range"`
}

// 警报数据结构
type Alert struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type Config struct {
	AlertThresholds AlertThresholds `json:"alert_thresholds"`
	UpdateInterval  int             `json:"update_interval"` // seconds
	CleanupConfig   CleanupConfig   `json:"cleanup_config"`
}

type AlertThresholds struct {
	VolumeSpike      float64 `json:"volume_spike"`
	PriceChange15Min float64 `json:"price_change_15min"`
	VolumeTrend      float64 `json:"volume_trend"`
	RSIOverbought    float64 `json:"rsi_overbought"`
	RSIOversold      float64 `json:"rsi_oversold"`
}
type CleanupConfig struct {
	InactiveTimeout   time.Duration `json:"inactive_timeout"`    // 不活跃超时时间
	MinScoreThreshold float64       `json:"min_score_threshold"` // 最低评分阈值
	NoAlertTimeout    time.Duration `json:"no_alert_timeout"`    // 无警报超时时间
	CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
}

var config = Config{
	AlertThresholds: AlertThresholds{
		VolumeSpike:      3.0,
		PriceChange15Min: 0.05,
		VolumeTrend:      2.0,
		RSIOverbought:    70,
		RSIOversold:      30,
	},
	CleanupConfig: CleanupConfig{
		InactiveTimeout:   30 * time.Minute,
		MinScoreThreshold: 15.0,
		NoAlertTimeout:    20 * time.Minute,
		CheckInterval:     5 * time.Minute,
	},
	UpdateInterval: 60, // 1 minute
}
