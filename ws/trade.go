package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"

	"github.com/andskur/hbdm-go"
)

var (
	muT sync.Mutex
	wgT sync.WaitGroup
)

// HBDM websocket API URL's
const (
	wsOrders = "wss://api.hbdm.com/notification"
)

// responseChannels handles all incoming data from the hbdm connection.
type responseTradeChannels struct {
	OrderPush map[string]chan WsOrderPushResponse
	ErrorFeed chan error
}

// WSTradeClient represents a JSON RPC v2 Connection over Websocket,
type WSTradeClient struct {
	apiKey    string
	apiSecret string
	conn      *websocket.Conn
	Updates   *responseTradeChannels
	exit      chan struct{}
}

// NewWSTradeClient creates a new hbm Websocket API client
func NewWSTradeClient(apiKey, apiSecret string) (*WSTradeClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsOrders, nil)
	if err != nil {
		return nil, err
	}

	handler := responseTradeChannels{
		OrderPush: make(map[string]chan WsOrderPushResponse),

		ErrorFeed: make(chan error),
	}

	client := &WSTradeClient{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		conn:      conn,
		Updates:   &handler,
		exit:      make(chan struct{}),
	}

	go client.handle()

	if err := client.auth(); err != nil {
		return nil, err
	}

	return client, nil
}

// TradeAuthRequest is request for websocket authentication
type TradeAuthRequest struct {
	Op               string `json:"op"`
	Type             string `json:"type"`
	AccessKeyId      string `json:"AccessKeyId"`
	SignatureMethod  string `json:"SignatureMethod"`
	SignatureVersion string `json:"SignatureVersion"`
	Timestamp        string `json:"Timestamp"`
	Signature        string `json:"Signature"`
}

// auth authenticate to Notification Websocket API
func (c *WSTradeClient) auth() error {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05")
	strRequest := "/notification"

	mapParams2Sign := make(map[string]string)
	mapParams2Sign["AccessKeyId"] = c.apiKey
	mapParams2Sign["SignatureMethod"] = "HmacSHA256"
	mapParams2Sign["SignatureVersion"] = "2"
	mapParams2Sign["Timestamp"] = timestamp
	hostName := "api.hbdm.com"

	sign := hbdm.CreateSign(mapParams2Sign, "GET", hostName, strRequest, c.apiSecret)

	request := &TradeAuthRequest{
		Op:               "auth",
		Type:             "api",
		AccessKeyId:      c.apiKey,
		SignatureMethod:  "HmacSHA256",
		SignatureVersion: "2",
		Timestamp:        timestamp,
		Signature:        sign,
	}

	msg, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
	}

	muT.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	muT.Unlock()
	if err != nil {
		log.Println("write", err)
	}

	return nil
}

// handle message from websocket
func (c *WSTradeClient) handle() {
	for {
		select {
		case <-c.exit:
			wgT.Done()
			break
		default:
			goto HandleMessages
		}

	HandleMessages:
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// c.muM.Lock()
			c.Updates.ErrorFeed <- err
			// c.muM.Unlock()
			break
		}

		msg, err := gzipCompress(message)
		if err != nil {
			c.Updates.ErrorFeed <- err
			break
		}

		var res map[string]interface{}
		if err := json.Unmarshal(msg, &res); err != nil {
			c.Updates.ErrorFeed <- err
			break
		}

		ok, err := c.checkPing(msg)
		if err != nil {
			c.Updates.ErrorFeed <- err
			break
		}
		if ok {
			continue
		}

		method, symbol, err := c.parseMethod(msg)
		if err != nil {
			c.Updates.ErrorFeed <- err
			break
		}

		switch method {
		case "orders":
			var resp WsOrderPushResponse
			if err := json.Unmarshal(msg, &resp); err != nil {
				c.Updates.ErrorFeed <- err
				break
			}
			muT.Lock()
			c.Updates.OrderPush[symbol] <- resp
			muT.Unlock()
		default:
			continue
		}
	}
}

// wsHbdmTradeResponse is top-level response from hbdm Trade Websocket API
type wsHbdmTradeResponse struct {
	Op    string `json:"op"`
	Topic string `json:"topic"`
}

// parseMethod parse API method from Websocket response message
func (c *WSTradeClient) parseMethod(msg []byte) (method, symbol string, err error) {
	var resp wsHbdmTradeResponse

	if err := json.Unmarshal(msg, &resp); err != nil {
		log.Printf("%s", err)
		return "", "", err
	}

	if resp.Op != "notify" {
		return
	}

	slice := strings.Split(resp.Topic, ".")

	method = slice[0]
	symbol = slice[1]

	return
}

// PingTrade is ping request/response
type PingTrade struct {
	Op string `json:"op"`
	Ts string `json:"ts"`
}

// checkPing check if message is "ping" if yes - send "pong"
func (c *WSTradeClient) checkPing(msg []byte) (bool, error) {
	var res map[string]interface{}

	if err := json.Unmarshal(msg, &res); err != nil {
		return false, err
	}

	for _, v := range res {
		op, ok := v.(string)
		if ok && strings.Contains(op, `ping`) {
			goto Pong
		}
	}
	return false, nil

Pong:
	var ping PingMarket

	err := json.Unmarshal(msg, &ping)
	if err != nil {
		return true, err
	}

	pong := PingTrade{
		Op: "pong",
		Ts: string(time.Now().Unix()),
	}

	jsonPong, err := json.Marshal(pong)
	if err != nil {
		return true, err
	}

	muT.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, jsonPong)
	muT.Unlock()
	if err != nil {
		return true, err
	}

	return true, nil
}

// OrderPushResponse is request for Order Push method subscribe
type wsOrderPushRequest struct {
	Op    string `json:"op"`
	Cid   string `json:"cid"`
	Topic string `json:"topic"`
}

// OrderPushResponse is response from Order Push method subscribe
type WsOrderPushResponse struct {
	Op             string       `json:"op"`
	Topic          string       `json:"topic"`
	Ts             int          `json:"ts"`
	Symbol         string       `json:"symbol"`
	ContractType   string       `json:"contract_type"`
	ContractCode   string       `json:"contract_code"`
	Volume         float64      `json:"volume"`
	Price          float64      `json:"price"`
	OrderPriceType string       `json:"order_price_type"`
	Direction      string       `json:"direction"`
	Offset         string       `json:"offset"`
	Status         int          `json:"status"`
	LevelRate      int          `json:"level_rate"`
	OrderId        int          `json:"order_id"`
	ClientOrderId  int          `json:"client_order_id"`
	OrderSource    string       `json:"order_source"`
	OrderType      int          `json:"order_type"`
	CreatedAt      int          `json:"created_at"`
	TradeVolume    float64      `json:"trade_volume"`
	TradeTurnover  float64      `json:"trade_turnover"`
	Fee            float64      `json:"fee"`
	TradeAvgPrice  float64      `json:"trade_avg_price"`
	MarginFrozen   float64      `json:"margin_frozen"`
	Profit         float64      `json:"profit"`
	Trade          []OrderTrade `json:"trade"`
}

type OrderTrade struct {
	TradeId       int     `json:"trade_id"`
	TradeVolume   float64 `json:"trade_volume"`
	TradePrice    float64 `json:"trade_price"`
	TradeFee      float64 `json:"trade_fee"`
	TradeTurnover float64 `json:"trade_turnover"`
	CreatedAt     int     `json:"created_at"`
}

// SubscribeOrderPush subscribe to websocket Order Push data
func (c *WSTradeClient) SubscribeOrderPush(symbol string) (<-chan WsOrderPushResponse, error) {
	if c.conn == nil {
		return nil, errors.New("connection is unitialized")
	}

	topik := fmt.Sprintf("orders.%s", symbol)

	cid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var request = wsOrderPushRequest{
		Op:    "sub",
		Cid:   cid.String(),
		Topic: topik,
	}

	msg, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
	}

	muT.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	muT.Unlock()
	if err != nil {
		log.Println("write", err)
	}

	muT.Lock()
	_, ok := c.Updates.OrderPush[symbol]
	if !ok {
		c.Updates.OrderPush[symbol] = make(chan WsOrderPushResponse)
	}

	depthChan := c.Updates.OrderPush[symbol]
	muT.Unlock()

	return depthChan, nil
}

// Close closes the Websocket connected to the hbdm api.
func (c *WSTradeClient) Close() {
	wgT.Add(1)
	// Cleanly close the connection by sending a close message and then
	// waiting (with timeout) for the server to close the connection.
	muT.Lock()
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	muT.Unlock()
	if err != nil {
		log.Println("write close:", err)
		return
	}

	close(c.exit)

	wgT.Wait()
	c.conn.Close()

	/*for _, channel := range c.Updates.OrderPush {
		close(channel)
	}

	close(c.Updates.ErrorFeed)
	c.Updates.ErrorFeed = make(chan error)
	c.Updates.OrderPush = make(map[string]chan WsOrderPushResponse)*/
}
