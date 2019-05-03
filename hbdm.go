package hbdm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
)

// hbdm API base url
const API_BASE = "https://api.hbdm.com/api/v1"

// New returns an instantiated hbdm struct
func New(apiKey, apiSecret string) *Hbdm {
	client := NewHttpClient(apiKey, apiSecret)
	return &Hbdm{client, sync.Mutex{}}
}

// NewWithCustomHttpClient returns an instantiated hbdm struct with custom http client
func NewWithCustomHttpClient(apiKey, apiSecret string, httpClient *http.Client) *Hbdm {
	client := NewHttpClientWithCustomHttpConfig(apiKey, apiSecret, httpClient)
	return &Hbdm{client, sync.Mutex{}}
}

// NewWithCustomTimeout returns an instantiated hbdm struct with custom timeout
func NewWithCustomTimeout(apiKey, apiSecret string, timeout time.Duration) *Hbdm {
	client := NewHttpClientWithCustomTimeout(apiKey, apiSecret, timeout)
	return &Hbdm{client, sync.Mutex{}}
}

// handleErr gets JSON response from livecoin API en deal with error
func handleErr(r interface{}) error {
	switch v := r.(type) {
	case map[string]interface{}:
		code, ok := v["err_code"]
		if !ok {
			return nil
		}

		err, ok := v["err_msg"]
		if !ok {
			return nil
		}

		return fmt.Errorf("error %g: %v", code, err)
	default:
		return fmt.Errorf("I don't know about type %T!\n", v)
	}
}

// Hbdm represent a hbdm client
type Hbdm struct {
	client *client
	mu     sync.Mutex
}

// SetDebug sets enable/disable http request/response dump
func (h *Hbdm) SetDebug(enable bool) {
	h.client.debug = enable
}

// ContractIndexResponse is response for ContactIndex method
type ContractIndexResponse struct {
	Status string            `json:"status"`
	Ts     int               `json:"ts"`
	Data   ContractIndexData `json:"data"`
}

// ContractIndexData is data field in Contract Index method response
type ContractIndexData struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"index_price"`
	Ts     int     `json:"index_ts"`
}

// UnmarshalJSON process correct json Unmarrshaling for ContractIndexData struct
func (c *ContractIndexData) UnmarshalJSON(b []byte) (err error) {
	var resp []map[string]interface{}

	if err := json.Unmarshal(b, &resp); err != nil {
		return fmt.Errorf("unmarshalling: %v", err)
	}

	c.Symbol = resp[0]["symbol"].(string)
	c.Price = resp[0]["index_price"].(float64)
	c.Ts = int(resp[0]["index_ts"].(float64))

	return
}

// ContractIndex Get Contract Index Price Information
func (h *Hbdm) ContractIndex(symbol string) (index *ContractIndexResponse, err error) {
	payload := make(map[string]interface{}, 1)
	payload["symbol"] = symbol

	r, err := h.client.do("GET", "contract_index", payload, false)
	if err != nil {
		return
	}
	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}
	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &index)
	return
}

// AccountInfoResponse is response for ContactIndex method
type AccountInfoResponse struct {
	Status string            `json:"status"`
	Ts     int               `json:"ts"`
	Data   []AccountInfoData `json:"data"`
}

// AccountInfoData is data field in Account Info method response
type AccountInfoData struct {
	Symbol            string  `json:"symbol"`
	MarginBalance     float64 `json:"margin_balance"`
	MarginPosition    float64 `json:"margin_position"`
	MarginFrozen      float64 `json:"margin_frozen"`
	MarginAvailable   float64 `json:"margin_available"`
	ProfitReal        float64 `json:"profit_real"`
	ProfitUnreal      float64 `json:"profit_unreal"`
	WithdrawAvailable float64 `json:"withdraw_available"`
	RiskRate          float64 `json:"risk_rate"`
	LiquidationPrice  float64 `json:"liquidation_price"`
}

// AccountInfo return User’s Account Information
func (h *Hbdm) AccountInfo(symbol string) (info *AccountInfoResponse, err error) {
	payload := make(map[string]interface{}, 1)
	if symbol != "" {
		payload["symbol"] = symbol
	}

	r, err := h.client.do("POST", "contract_account_info", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &info)
	return
}

// ContractPositionResponse is response from PositionInfo method
type ContractPositionResponse struct {
	Status string                 `json:"status"`
	Data   []ContractPositionData `json:"data"`
	Ts     int                    `json:"ts"`
}

// ContractPositionData is Position data model
type ContractPositionData struct {
	Symbol         string  `json:"symbol"`
	ContractType   string  `json:"contract_type"`
	ContractCode   string  `json:"contract_code"`
	Volume         float64 `json:"volume"`
	Price          float64 `json:"price"`
	Available      float64 `json:"available"`
	Frozen         float64 `json:"frozen"`
	CostOpen       float64 `json:"cost_open"`
	CostHold       float64 `json:"cost_hold"`
	ProfitUnreal   float64 `json:"profit_unreal"`
	ProfitRate     float64 `json:"profit_rate"`
	Profit         float64 `json:"profit"`
	PositionMargin float64 `json:"position_margin"`
	LevelRate      int     `json:"level_rate"`
	Direction      string  `json:"direction"`
}

// PositionInfo Get Account open position
func (h *Hbdm) PositionInfo(symbol string) (positions *ContractPositionResponse, err error) {
	payload := make(map[string]interface{}, 1)
	if symbol != "" {
		payload["symbol"] = symbol
	}

	r, err := h.client.do("POST", "contract_position_info", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &positions)
	return

}

// ContractOrderResponse is response from ContractOder method
type ContractOrderResponse struct {
	Status string            `json:"status"`
	Data   ContractOrderData `json:"data"`
	Ts     int               `json:"ts"`
}

// ContractOrderData is ContractOrder method response data
type ContractOrderData struct {
	OrderId       float64 `json:"order_id"`
	ClientOrderId float64 `json:"client_order_id"`
}

// ContractOder place order for open or close contract position
func (h *Hbdm) ContractOder(symbol, contractType, contractCode, direction, offset, priceType string, price float64, volume, levelRate int) (order *ContractOrderResponse, err error) {

	orderId, err := h.GetAndIncrementNonce()
	if err != nil {
		return nil, err
	}

	payload := make(map[string]interface{}, 9)
	payload["symbol"] = symbol
	payload["contract_type"] = contractType
	payload["client_order_id"] = orderId
	payload["price"] = price
	payload["volume"] = volume
	payload["direction"] = direction
	payload["offset"] = offset
	payload["lever_rate"] = levelRate
	payload["order_price_type"] = priceType

	if contractCode != "" {
		payload["contract_code"] = contractCode
	}

	r, err := h.client.do("POST", "contract_order", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &order)
	return
}

type CancelOrderResponse struct {
	Status    string             `json:"status"`
	Errors    []CancelOrderError `json:"errors"`
	Successes []string           `json:"successes"`
	Ts        int                `json:"ts"`
}

type CancelOrderError struct {
	OrderId string `json:"order_id"`
	ErrCode int    `json:"err_code"`
	ErrMsg  string `json:"err_msg"`
}

// CancelAllOrders cancel all user orders for given symbol
func (h *Hbdm) CanceOrder(symbol, orderId, clientOrderId string) (resp *CancelOrderResponse, err error) {
	payload := make(map[string]interface{}, 3)
	payload["symbol"] = symbol

	if orderId != "" {
		payload["order_id"] = orderId
	}

	if clientOrderId != "" {
		payload["client_order_id"] = clientOrderId
	}

	r, err := h.client.do("POST", "contract_cancel", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &resp)
	return

}

// CancelAllOrdersResponse is response from CancelAllOrders method
type CancelAllOrdersResponse struct {
	Status string                `json:"status"`
	Data   []CancelAllOrdersData `json:"data"`
	Ts     int                   `json:"ts"`
}

// CancelAllOrdersData is CancelAllOrdersData method response data
type CancelAllOrdersData struct {
	OrderId string `json:"order_id"`
	ErrCode int    `json:"err_code"`
	ErrMsg  string `json:"err_msg"`
}

// CancelAllOrders cancel all user orders for given symbol
func (h *Hbdm) CancelAllOrders(symbol string) (resp *CancelAllOrdersResponse, err error) {
	payload := make(map[string]interface{}, 1)
	payload["symbol"] = symbol

	r, err := h.client.do("POST", "contract_cancelall", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &resp)
	return

}

// OrderInfoResponse is response for OrderInfo method
type OrderInfoResponse struct {
	Status string          `json:"status"`
	Ts     int             `json:"ts"`
	Data   []OrderInfoData `json:"data"`
}

// OrderInfoData id Order data model
type OrderInfoData struct {
	Symbol        string  `json:"symbol"`
	ContractType  string  `json:"contract_type"`
	ContractCode  string  `json:"contract_code"`
	Volume        int     `json:"volume"`
	Price         float64 `json:"price"`
	PriceType     string  `json:"order_price_type"`
	Direction     string  `json:"direction"`
	Offset        string  `json:"offset"`
	LevelRate     int     `json:"level_rate"`
	OrderId       int     `json:"order_id"`
	ClientOrderId int     `json:"client_order_id"`
	OrderSource   string  `json:"order_source"`
	CreatedAt     int     `json:"created_at"`
	TradeVolume   int     `json:"trade_volume"`
	TradeTurnover float64 `json:"trade_turnover"`
	Fee           float64 `json:"fee"`
	TradeAvgPrice float64 `json:"trade_avg_price"`
	MarginFrozen  float64 `json:"margin_frozen"`
	Profit        float64 `json:"profit"`
	Status        int     `json:"status"`
}

// OrderInfo get Order info by given order ID for providing Symbol
func (h *Hbdm) OrderInfo(orderId, clientOrderId, symbol string) (orders *OrderInfoResponse, err error) {
	payload := make(map[string]interface{}, 3)
	if symbol != "" {
		payload["symbol"] = symbol
	}
	if orderId != "" {
		payload["order_id"] = orderId
	}
	if clientOrderId != "" {
		payload["client_order_id"] = clientOrderId
	}

	spew.Dump(payload)

	r, err := h.client.do("POST", "contract_order_info", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &orders)
	return
}

// OrdersResponse is mutual response for Orders arrays methods - Open, History
type OrdersResponse struct {
	Status string `json:"status"`
	Ts     int    `json:"ts"`
	Data   struct {
		Orders []OrderInfoData `json:"orders"`
	} `json:"data"`
}

// OpenOrders get all open orders
func (h *Hbdm) OpenOrders(symbol string, pageIndex, pageSize *int) (orders *OrdersResponse, err error) {
	payload := make(map[string]interface{}, 3)
	if symbol != "" {
		payload["symbol"] = symbol
	}
	if pageIndex != nil {
		payload["page_index"] = *pageIndex
	}
	if pageSize != nil {
		payload["page_size"] = *pageSize
	}

	r, err := h.client.do("POST", "contract_openorders", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &orders)
	return
}

// HistoryOrders get history orders by given filters
func (h *Hbdm) HistoryOrders(symbol string, tradeType, orderType, status, create int, pageIndex, pageSize *int) (orders *OrdersResponse, err error) {
	payload := make(map[string]interface{}, 7)
	payload["symbol"] = symbol
	payload["trade_type"] = tradeType
	payload["type"] = orderType
	payload["status"] = status
	payload["create_date"] = create

	if pageIndex != nil {
		payload["page_index"] = *pageIndex
	}
	if pageSize != nil {
		payload["page_size"] = *pageSize
	}

	r, err := h.client.do("POST", "contract_hisorders", payload, true)
	if err != nil {
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		return
	}

	if err = handleErr(response); err != nil {
		return
	}

	err = json.Unmarshal(r, &orders)
	return
}
