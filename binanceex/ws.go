package binanceex

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type AggTrade struct {
	Symbol    string
	Price     float64
	Quantity  float64
	IsBuyer   bool
	EventTime time.Time
}

type BookTicker struct {
	Symbol   string
	BidPrice float64
	AskPrice float64
	BidQty   float64
	AskQty   float64
}

type Kline struct {
	Symbol    string
	Interval  string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Closed    bool
	CloseTime time.Time
}

type WSCallbacks struct {
	OnAggTrade   func(AggTrade)
	OnBookTicker func(BookTicker)
	OnKline      func(Kline)
}

type WSManager struct {
	net       Network
	callbacks WSCallbacks
	mu        sync.Mutex
	conns     []*websocket.Conn
}

func NewWSManager(net Network, cb WSCallbacks) *WSManager {
	return &WSManager{net: net, callbacks: cb}
}

func (m *WSManager) Start(ctx context.Context, symbols []string) {
	if len(symbols) == 0 {
		return
	}
	streams := make([]string, 0, len(symbols)*3)
	for _, s := range symbols {
		low := strings.ToLower(s)
		streams = append(streams,
			low+"@aggTrade",
			low+"@bookTicker",
			low+"@kline_5m",
		)
	}
	// Binance combined stream limit — batch by 30 streams (~10 symbols)
	const batch = 30
	for i := 0; i < len(streams); i += batch {
		end := i + batch
		if end > len(streams) {
			end = len(streams)
		}
		batchStreams := streams[i:end]
		go m.runCombined(ctx, batchStreams)
	}
}

func (m *WSManager) runCombined(ctx context.Context, streams []string) {
	url := m.net.WSBase + "/stream?streams=" + strings.Join(streams, "/")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("binance ws dial: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		m.mu.Lock()
		m.conns = append(m.conns, conn)
		m.mu.Unlock()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				conn.Close()
				break
			}
			m.dispatch(msg)
		}
		time.Sleep(2 * time.Second)
	}
}

func (m *WSManager) dispatch(msg []byte) {
	var envelope struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg, &envelope); err != nil {
		return
	}
	stream := envelope.Stream
	raw := envelope.Data
	if strings.Contains(stream, "@aggTrade") {
		var e struct {
			S string `json:"s"`
			P string `json:"p"`
			Q string `json:"q"`
			M bool   `json:"m"`
			T int64  `json:"T"`
		}
		if json.Unmarshal(raw, &e) == nil && m.callbacks.OnAggTrade != nil {
			m.callbacks.OnAggTrade(AggTrade{
				Symbol: e.S, Price: ParseFloat(e.P), Quantity: ParseFloat(e.Q),
				IsBuyer: !e.M, EventTime: time.UnixMilli(e.T),
			})
		}
		return
	}
	if strings.Contains(stream, "@bookTicker") {
		var e struct {
			S string `json:"s"`
			B string `json:"b"`
			A string `json:"a"`
			Bq string `json:"B"`
			Aq string `json:"A"`
		}
		if json.Unmarshal(raw, &e) == nil && m.callbacks.OnBookTicker != nil {
			m.callbacks.OnBookTicker(BookTicker{
				Symbol: e.S, BidPrice: ParseFloat(e.B), AskPrice: ParseFloat(e.A),
				BidQty: ParseFloat(e.Bq), AskQty: ParseFloat(e.Aq),
			})
		}
		return
	}
	if strings.Contains(stream, "@kline_5m") {
		var e struct {
			S string `json:"s"`
			K struct {
				O string `json:"o"`
				H string `json:"h"`
				L string `json:"l"`
				C string `json:"c"`
				V string `json:"v"`
				X bool   `json:"x"`
				T int64  `json:"T"`
			} `json:"k"`
		}
		if json.Unmarshal(raw, &e) == nil && m.callbacks.OnKline != nil {
			m.callbacks.OnKline(Kline{
				Symbol: e.S, Interval: "5m",
				Open: ParseFloat(e.K.O), High: ParseFloat(e.K.H),
				Low: ParseFloat(e.K.L), Close: ParseFloat(e.K.C),
				Volume: ParseFloat(e.K.V), Closed: e.K.X,
				CloseTime: time.UnixMilli(e.K.T),
			})
		}
	}
}

func (m *WSManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.conns {
		_ = c.Close()
	}
	m.conns = nil
}
