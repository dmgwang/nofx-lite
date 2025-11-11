package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nofx-lite/hook"
	"strconv"
	"time"
)

const (
	baseURL = "https://fapi.binance.com"
)

type APIClient struct {
	client *http.Client
}

func NewAPIClient() *APIClient {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	hookRes := hook.HookExec[hook.SetHttpClientResult](hook.SET_HTTP_CLIENT, client)
	if hookRes != nil && hookRes.Error() == nil {
		log.Printf("使用Hook设置的HTTP客户端")
		client = hookRes.GetResult()
	}

	return &APIClient{
		client: client,
	}
}

func (c *APIClient) GetExchangeInfo() (*ExchangeInfo, error) {
	url := fmt.Sprintf("%s/fapi/v1/exchangeInfo", baseURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var exchangeInfo ExchangeInfo
	err = json.Unmarshal(body, &exchangeInfo)
	if err != nil {
		return nil, err
	}

	return &exchangeInfo, nil
}

func (c *APIClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("%s/fapi/v1/klines", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("interval", interval)
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var klineResponses []KlineResponse
	err = json.Unmarshal(body, &klineResponses)
	if err != nil {
		log.Printf("获取K线数据失败,响应内容: %s", string(body))
		return nil, err
	}

	var klines []Kline
	for _, kr := range klineResponses {
		kline, err := parseKline(kr)
		if err != nil {
			log.Printf("解析K线数据失败: %v", err)
			continue
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

func parseKline(kr KlineResponse) (Kline, error) {
	var kline Kline

	if len(kr) < 11 {
		return kline, fmt.Errorf("invalid kline data")
	}

	// 解析各个字段
	kline.OpenTime = int64(kr[0].(float64))
	kline.Open, _ = strconv.ParseFloat(kr[1].(string), 64)
	kline.High, _ = strconv.ParseFloat(kr[2].(string), 64)
	kline.Low, _ = strconv.ParseFloat(kr[3].(string), 64)
	kline.Close, _ = strconv.ParseFloat(kr[4].(string), 64)
	kline.Volume, _ = strconv.ParseFloat(kr[5].(string), 64)
	kline.CloseTime = int64(kr[6].(float64))
	kline.QuoteVolume, _ = strconv.ParseFloat(kr[7].(string), 64)
	kline.Trades = int(kr[8].(float64))
	kline.TakerBuyBaseVolume, _ = strconv.ParseFloat(kr[9].(string), 64)
	kline.TakerBuyQuoteVolume, _ = strconv.ParseFloat(kr[10].(string), 64)

	return kline, nil
}

func (c *APIClient) GetCurrentPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/fapi/v1/ticker/price", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var ticker PriceTicker
	err = json.Unmarshal(body, &ticker)
	if err != nil {
		return 0, err
	}

	price, err := strconv.ParseFloat(ticker.Price, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// GetOrderBookData 获取订单簿深度数据
// limit参数控制返回的档位数量，建议值：5, 10, 20
// 返回的数据包含买卖盘各limit个档位的数据
func (c *APIClient) GetOrderBookData(symbol string, limit int) (*DepthData, error) {
	url := fmt.Sprintf("%s/fapi/v1/depth", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Binance API响应结构
	var apiResponse struct {
		LastUpdateID int64      `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"` // [价格, 数量]
		Asks         [][]string `json:"asks"` // [价格, 数量]
	}

	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Printf("解析订单簿数据失败,响应内容: %s", string(body))
		return nil, err
	}

	// 转换为内部数据结构
	depthData := &DepthData{
		Symbol:     symbol,
		Timestamp:  time.Now(),
		LastUpdate: time.Now(),
		Bids:       make([]DepthLevel, 0, len(apiResponse.Bids)),
		Asks:       make([]DepthLevel, 0, len(apiResponse.Asks)),
	}

	// 解析买盘数据 (按价格降序排列)
	for _, bid := range apiResponse.Bids {
		if len(bid) >= 2 {
			price, err1 := strconv.ParseFloat(bid[0], 64)
			quantity, err2 := strconv.ParseFloat(bid[1], 64)
			if err1 == nil && err2 == nil && quantity > 0 {
				depthData.Bids = append(depthData.Bids, DepthLevel{
					Price:    price,
					Quantity: quantity,
				})
			}
		}
	}

	// 解析卖盘数据 (按价格升序排列)
	for _, ask := range apiResponse.Asks {
		if len(ask) >= 2 {
			price, err1 := strconv.ParseFloat(ask[0], 64)
			quantity, err2 := strconv.ParseFloat(ask[1], 64)
			if err1 == nil && err2 == nil && quantity > 0 {
				depthData.Asks = append(depthData.Asks, DepthLevel{
					Price:    price,
					Quantity: quantity,
				})
			}
		}
	}

	// 计算买卖价差和中间价
	if len(depthData.Bids) > 0 && len(depthData.Asks) > 0 {
		bestBid := depthData.Bids[0].Price
		bestAsk := depthData.Asks[0].Price
		depthData.Spread = bestAsk - bestBid
		depthData.MidPrice = (bestBid + bestAsk) / 2
	}

	return depthData, nil
}
