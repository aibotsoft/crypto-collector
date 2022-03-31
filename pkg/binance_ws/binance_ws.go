package binance_ws

import (
	"context"
	"fmt"
	"github.com/aibotsoft/crypto-collector/pkg/config"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"strings"
	"time"
)

type WsClient struct {
	cfg     *config.Config
	log     *zap.Logger
	ctx     context.Context
	errCh   chan error
	Conn    *websocket.Conn
	handler func(event *WsBookTickerEvent)
}

func NewWsClient(cfg *config.Config, log *zap.Logger, ctx context.Context, handler func(event *WsBookTickerEvent)) *WsClient {
	ws := &WsClient{
		cfg:     cfg,
		log:     log,
		ctx:     ctx,
		errCh:   make(chan error),
		handler: handler,
	}
	return ws
}
func (w *WsClient) ConnectAndSubscribe() error {
	start := time.Now()
	dialCtx, cancel := context.WithTimeout(w.ctx, w.cfg.Ws.ConnTimeout)
	defer cancel()
	//endpoint := fmt.Sprintf("%s/!bookTicker", w.cfg.Binance.WsHost)
	endpoint := w.cfg.Binance.WsHost
	for _, m := range w.cfg.Markets {
		endpoint += fmt.Sprintf("%s@bookTicker", strings.ToLower(m)+"usdt") + "/"
	}
	endpoint = endpoint[:len(endpoint)-1]
	c, _, err := websocket.DefaultDialer.DialContext(dialCtx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("dial_error: %w", err)
	}
	c.SetReadLimit(655350)
	w.Conn = c
	w.log.Info("ws_connect_done", zap.String("service", w.cfg.Binance.Name), zap.Duration("elapsed", time.Since(start)))
	return nil
}

func (w *WsClient) ReadLoop() {
	for {
		if w.Conn == nil {
			w.log.Debug("conn_nil", zap.String("service", w.cfg.Binance.Name))
			time.Sleep(time.Millisecond * 100)
			continue
		}
		var event Event
		err := w.Conn.ReadJSON(&event)
		if err != nil {
			w.log.Warn("ReadLoop_error", zap.Error(err))
			w.errCh <- fmt.Errorf("read_loop_error: %w", err)
			time.Sleep(time.Second * 2)
			continue
		}
		event.Data.ReceiveTime = time.Now().UnixNano()
		w.handler(&event.Data)
	}
}
func (w *WsClient) Serve() error {
	go w.ReadLoop()
	err := w.ConnectAndSubscribe()
	if err != nil {
		return err
	}
	for {
		select {
		case err := <-w.errCh:
			w.log.Error("serve_error", zap.String("service", w.cfg.Binance.Name), zap.Error(err))
			err = w.ConnectAndSubscribe()
			if err != nil {
				w.log.Warn("ConnectAndSubscribe_error", zap.Error(err))
				//time.Sleep(time.Second)
			}
		case <-w.ctx.Done():
			w.log.Info("close_binance_ws_conn")
			w.Conn.Close()
			return nil
		}
	}
}
