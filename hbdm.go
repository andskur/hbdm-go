package hbdm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// hbdm API base url
const API_BASE = "https://api.hbdm.com/api/v1"

// New returns an instantiated hbdm struct
func New(apiKey, apiSecret string) *Hbdm {
	client := NewHttpClient(apiKey, apiSecret)
	return &Hbdm{client}
}

// NewWithCustomHttpClient returns an instantiated hbdm struct with custom http client
func NewWithCustomHttpClient(apiKey, apiSecret string, httpClient *http.Client) *Hbdm {
	client := NewHttpClientWithCustomHttpConfig(apiKey, apiSecret, httpClient)
	return &Hbdm{client}
}

// NewWithCustomTimeout returns an instantiated hbdm struct with custom timeout
func NewWithCustomTimeout(apiKey, apiSecret string, timeout time.Duration) *Hbdm {
	client := NewHttpClientWithCustomTimeout(apiKey, apiSecret, timeout)
	return &Hbdm{client}
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
}

// SetDebug sets enable/disable http request/response dump
func (b *Hbdm) SetDebug(enable bool) {
	b.client.debug = enable
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
func (b *Hbdm) ContractIndex(symbol string) (index *ContractIndexResponse, err error) {
	payload := make(map[string]string, 1)
	payload["symbol"] = symbol

	r, err := b.client.do("GET", "contract_index", payload, false)
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
func (b *Hbdm) AccountInfo(symbol string) (info *AccountInfoResponse, err error) {
	payload := make(map[string]string, 1)
	if symbol != "" {
		payload["symbol"] = symbol
	}

	r, err := b.client.do("POST", "contract_account_info", payload, true)
	if err != nil {
		fmt.Println("request")
		return
	}

	var response interface{}
	if err = json.Unmarshal(r, &response); err != nil {
		fmt.Println("interface unmarshaling")
		return
	}

	if err = handleErr(response); err != nil {
		fmt.Println("err handler")
		return
	}

	err = json.Unmarshal(r, &info)
	return
}
