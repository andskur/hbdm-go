package ws

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
)

var mu sync.Mutex

// HBDM websocket API URL's
const (
	wsMarketData = "wss://www.hbdm.com/ws"
)

// responseMarketChannels handles all incoming data from the hbdm connection.
type responseMarketChannels struct {
	MarketDepth map[string]chan WsDepthMarketResponse

	ErrorFeed chan error
}

// WSMarketClient represents a JSON RPC v2 Connection over Websocket,
type WSMarketClient struct {
	conn    *websocket.Conn
	Updates *responseMarketChannels
}

// NewWSMarketClient creates a new hbm Websocket API client
func NewWSMarketClient() (*WSMarketClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsMarketData, nil)
	if err != nil {
		return nil, err
	}

	handler := responseMarketChannels{
		MarketDepth: make(map[string]chan WsDepthMarketResponse),

		ErrorFeed: make(chan error),
	}

	client := &WSMarketClient{
		conn:    conn,
		Updates: &handler,
	}

	go client.handle()

	return client, nil
}

// wsHbdmMarketRequest is top-level hbdm request to Websocket API
type wsHbdmMarketRequest struct {
	Sub string `json:"sub"`
	Id  string `json:"id"`
}

// wsHbdmMarketResponse is top-level response from hbdm Websocket API
type wsHbdmMarketResponse struct {
	Ch string `json:"ch"`
	Ts int    `json:"ts"`
}

// handle message from websocket
func (c *WSMarketClient) handle() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			c.Updates.ErrorFeed <- err
			break
		}

		msg, err := gzipCompress(message)
		if err != nil {
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
		case "depth":
			var resp WsDepthMarketResponse
			if err := json.Unmarshal(msg, &resp); err != nil {
				c.Updates.ErrorFeed <- err
				break
			}
			mu.Lock()
			c.Updates.MarketDepth[symbol] <- resp
			mu.Unlock()
		default:
			continue
		}

	}
}

// PingMarket represent "PingMarket" request from Websocket server
type PingMarket struct {
	Ping int `json:"Ping"`
}

// PongMarket represent "PongMarket" request to Websocket server
type PongMarket struct {
	Pong int `json:"Pong"`
}

// checkPing check if message is "ping" if yes - send "pong"
func (c *WSMarketClient) checkPing(msg []byte) (bool, error) {
	var res map[string]interface{}

	if err := json.Unmarshal(msg, &res); err != nil {
		return false, err
	}

	for k := range res {
		if strings.Contains(k, `ping`) {
			goto Pong
		}
		return false, nil
	}

Pong:
	var ping PingMarket

	err := json.Unmarshal(msg, &ping)
	if err != nil {
		return true, err
	}

	pong := PongMarket{Pong: ping.Ping}

	jsonPong, err := json.Marshal(pong)
	if err != nil {
		return true, err
	}

	mu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, jsonPong)
	mu.Unlock()
	if err != nil {
		return true, err
	}

	return true, nil
}

// parseMethod parse API method from Websocket response message
func (c WSMarketClient) parseMethod(msg []byte) (method, symbol string, err error) {
	var resp wsHbdmMarketResponse

	if err := json.Unmarshal(msg, &resp); err != nil {
		log.Printf("%s", err)
		return "", "", err
	}

	slice := strings.Split(resp.Ch, ".")
	length := len(slice)

	if length > 1 {
		symbol = slice[1]
	}

	if length > 2 {
		method = slice[2]
	}

	return
}

// Close closes the Websocket connected to the hbdm api.
func (c *WSMarketClient) Close() {
	// Cleanly close the connection by sending a close message and then
	// waiting (with timeout) for the server to close the connection.
	mu.Lock()
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	mu.Unlock()
	if err != nil {
		log.Println("write close:", err)
		return
	}
	c.conn.Close()

	for _, channel := range c.Updates.MarketDepth {
		close(channel)
	}

	close(c.Updates.ErrorFeed)

	c.Updates.MarketDepth = make(map[string]chan WsDepthMarketResponse)
	c.Updates.ErrorFeed = make(chan error)

}

// WsDepthMarketResponse is Market Depth method top-level response
type WsDepthMarketResponse struct {
	Ch   string          `json:"ch"`
	Ts   int             `json:"ts"`
	Tick MarketDepthTick `json:"tick"`
}

// MarketDepthTick is Depth Offer main data
type MarketDepthTick struct {
	Ch      string  `json:"ch"`
	Mrid    int     `json:"mrid"`
	Id      int     `json:"id"`
	Ts      int     `json:"ts"`
	Version int     `json:"version"`
	Bids    []Offer `json:"bids"`
	Asks    []Offer `json:"asks"`
}

// Offer is Offer with Contract Price and Amount
type Offer struct {
	Price  float64 `json:"price"`
	Amount float64 `json:"amount"`
}

// UnmarshalJSON make correct Json Unmarshaling fro Offer structure
func (o *Offer) UnmarshalJSON(b []byte) error {
	var offer []float64

	if err := json.Unmarshal(b, &offer); err != nil {
		return fmt.Errorf("unmarshalling: %v", err)
	}

	o.Price = offer[0]
	o.Amount = offer[1]
	return nil
}

// SubscribeMarketDepth subscribe to websocket Market Depth data
func (c *WSMarketClient) SubscribeMarketDepth(symbol string) (<-chan WsDepthMarketResponse, error) {
	if c.conn == nil {
		return nil, errors.New("connection is unitialized")
	}

	sub := fmt.Sprintf("market.%s.depth.step0", symbol)

	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var request = wsHbdmMarketRequest{Sub: sub, Id: id.String()}

	msg, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
	}

	mu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	mu.Unlock()
	if err != nil {
		log.Println("write", err)
	}

	mu.Lock()
	_, ok := c.Updates.MarketDepth[symbol]
	if !ok {
		c.Updates.MarketDepth[symbol] = make(chan WsDepthMarketResponse)
	}

	depthChan := c.Updates.MarketDepth[symbol]
	mu.Unlock()

	return depthChan, nil
}

// gzipCompress compress Gzip response
func gzipCompress(in []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	msg, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
