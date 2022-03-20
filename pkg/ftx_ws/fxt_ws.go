package ftx_ws

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aibotsoft/crypto-collector/pkg/config"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"time"
)

var (
	pingMsg        *websocket.PreparedMessage
	pingStart      time.Time
	subscribeStart time.Time
)

type WsClient struct {
	cfg     *config.Config
	log     *zap.Logger
	ctx     context.Context
	errCh   chan error
	Conn    *websocket.Conn
	EventCh chan Response
}

func NewWsClient(cfg *config.Config, log *zap.Logger, ctx context.Context) *WsClient {
	jsonByte, _ := json.Marshal(Request{Op: "ping"})
	pingMsg, _ = websocket.NewPreparedMessage(websocket.TextMessage, jsonByte)
	ws := &WsClient{
		cfg:     cfg,
		log:     log,
		ctx:     ctx,
		errCh:   make(chan error),
		EventCh: make(chan Response, 10000),
	}
	return ws
}

func (w *WsClient) ConnectAndSubscribe() error {
	start := time.Now()
	dialCtx, cancel := context.WithTimeout(w.ctx, w.cfg.Ws.ConnTimeout)
	defer cancel()
	c, _, err := websocket.DefaultDialer.DialContext(dialCtx, w.cfg.Ftx.WsHost, nil)
	if err != nil {
		return fmt.Errorf("dial_error: %w", err)
	}
	c.SetReadLimit(655350)
	w.Conn = c
	err = w.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe_error: %w", err)
	}
	w.log.Info("ws_connect_done", zap.String("service", w.cfg.Ftx.Name), zap.Duration("elapsed", time.Since(start)))
	return nil
}
func (w *WsClient) Subscribe() error {
	subscribeStart = time.Now()
	if w.cfg.Service.EnableUSD {
		err := w.Conn.WriteJSON(Request{Op: "subscribe", Channel: "ticker", Market: "usdt/usd"})
		if err != nil {
			return fmt.Errorf("subscribe_error: %w, market: %s", err, "usdt/usd")
		}
	}

	for _, c := range w.cfg.Markets {
		err := w.Conn.WriteJSON(Request{Op: "subscribe", Channel: "ticker", Market: fmt.Sprintf("%s/usdt", c)})
		if err != nil {
			return fmt.Errorf("subscribe_error: %w, market: %s", err, c)
		}
		if !w.cfg.Service.EnableUSD {
			continue
		}
		err = w.Conn.WriteJSON(Request{Op: "subscribe", Channel: "ticker", Market: fmt.Sprintf("%s/usd", c)})
		if err != nil {
			return fmt.Errorf("subscribe_error: %w, market: %s", err, fmt.Sprintf("%s/usd", c))
		}
	}
	w.log.Debug("subscribe_done", zap.Duration("elapsed", time.Since(subscribeStart)), zap.Int("markets_count", len(w.cfg.Markets)), zap.Strings("markets", w.cfg.Markets))
	return nil
}
func (w *WsClient) Ping() {
	pingStart = time.Now()
	err := w.Conn.WritePreparedMessage(pingMsg)
	if err != nil {
		w.log.Warn("write_ping_error", zap.String("service", w.cfg.Ftx.Name), zap.Error(err))
	}
}

func (w *WsClient) ReadLoop() {
	for {
		if w.Conn == nil {
			w.log.Debug("conn_nil", zap.String("service", w.cfg.Ftx.Name))
			time.Sleep(time.Millisecond * 50)
			continue
		}
		var event Response
		err := w.Conn.ReadJSON(&event)
		if err != nil {
			//w.log.Warn("ReadLoop_error", zap.Error(err))
			w.errCh <- fmt.Errorf("read_loop_error: %w", err)
			time.Sleep(time.Second * 2)
			continue
		}
		switch event.Type {
		case "update":
			//w.log.Debug("event_ch", zap.Int("len", len(w.EventCh)))
			event.ReceiveTime = time.Now().UnixNano()
			w.EventCh <- event
		case "pong":
			w.log.Debug("pong", zap.Duration("elapsed", time.Since(pingStart)))
		case "subscribed":
			w.log.Debug("subscribed", zap.Duration("elapsed", time.Since(subscribeStart)), zap.Any("event", event))
		default:
			w.log.Debug("unknown_event_type", zap.String("type", event.Type), zap.Any("event", event))
		}
	}
}

func (w *WsClient) Serve() error {
	go w.ReadLoop()
	pingTimer := time.Tick(time.Second * 15)
	//failTimer := time.Tick(time.Second * 20)
	err := w.ConnectAndSubscribe()
	if err != nil {
		return err
	}
	for {
		select {
		case <-w.ctx.Done():
			w.log.Info("close_fxt_ws_conn", zap.String("service", w.cfg.Ftx.Name))
			w.Conn.Close()
			return nil
		case err := <-w.errCh:
			w.log.Info("serve_error", zap.String("service", w.cfg.Ftx.Name), zap.Error(err))
			err = w.ConnectAndSubscribe()
			if err != nil {
				w.log.Warn("ConnectAndSubscribe_error", zap.String("service", w.cfg.Ftx.Name), zap.Error(err))
				//time.Sleep(time.Second)
			}
		//case <-failTimer:
		//	w.Conn.Close()
		//	w.log.Debug("close_done")
		case <-pingTimer:
			w.Ping()
		}
	}
}
