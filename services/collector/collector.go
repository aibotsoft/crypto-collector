package collector

import (
	"context"
	"fmt"
	"github.com/RobinUS2/golang-moving-average"
	"github.com/aibotsoft/crypto-collector/pkg/binance_ws"
	"github.com/aibotsoft/crypto-collector/pkg/config"
	"github.com/aibotsoft/crypto-collector/pkg/ftx_ws"
	"github.com/cheynewallace/tabby"
	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

const cryptoSubject = "crypto-surebet"
const binanceExchange = "binance"
const ftxExchange = "ftx"
const usdt = "USDT"
const usd = "USD"
const usdUsdtMarket = "USDT/USD"

var d100 = decimal.RequireFromString("100")

type Collector struct {
	cfg                *config.Config
	log                *zap.Logger
	ctx                context.Context
	binMap             sync.Map
	fxtMap             sync.Map
	errCh              chan error
	ftxClient          *ftx_ws.WsClient
	binClient          *binance_ws.WsClient
	ec                 *nats.EncodedConn
	binFtxSymbolMap    map[string]string
	binFtxUsdSymbolMap map[string]string
	maPriceMap         map[string]*movingaverage.MovingAverage
	ftxCountMap        map[string]*atomic.Int64
	binCountMap        map[string]*atomic.Int64
	lastBinTime        int64

	idLock    sync.Mutex
	lastID    int64
	usdtPrice decimal.Decimal
}

func NewCollector(cfg *config.Config, log *zap.Logger, ctx context.Context) *Collector {
	return &Collector{
		cfg:                cfg,
		log:                log,
		ctx:                ctx,
		errCh:              make(chan error),
		ftxClient:          ftx_ws.NewWsClient(cfg, log, ctx),
		binClient:          binance_ws.NewWsClient(cfg, log, ctx),
		binFtxSymbolMap:    make(map[string]string),
		binFtxUsdSymbolMap: make(map[string]string),
		maPriceMap:         make(map[string]*movingaverage.MovingAverage),
		ftxCountMap:        make(map[string]*atomic.Int64),
		binCountMap:        make(map[string]*atomic.Int64),
		usdtPrice:          decimal.RequireFromString("1"),
	}
}
func (c *Collector) Run() error {
	for _, m := range c.cfg.Markets {
		c.binFtxSymbolMap[fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)] = fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)
		c.binFtxUsdSymbolMap[fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)] = fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)

		c.binFtxSymbolMap[fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)] = fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)
		c.binFtxSymbolMap[fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)] = fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)
	}
	for _, m := range c.cfg.Markets {
		symbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)
		t := TickerData{Symbol: symbol}
		c.fxtMap.Store(symbol, &t)
		c.maPriceMap[symbol] = movingaverage.New(c.cfg.Service.AverageWindow)
		c.ftxCountMap[symbol] = &atomic.Int64{}

		if c.cfg.Service.EnableUSD {
			usdSymbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)
			c.log.Debug("usd_symbol", zap.String("symbol", usdSymbol))
			t2 := TickerData{Symbol: usdSymbol}
			c.fxtMap.Store(usdSymbol, &t2)
			c.maPriceMap[usdSymbol] = movingaverage.New(c.cfg.Service.AverageWindow)
			c.ftxCountMap[usdSymbol] = &atomic.Int64{}

		}
	}

	for _, m := range c.cfg.Markets {
		symbol := fmt.Sprintf("%s%s", strings.ToUpper(m), usdt)
		b := TickerData{Symbol: symbol}
		c.binMap.Store(symbol, &b)
		c.binCountMap[symbol] = &atomic.Int64{}
	}
	err := c.connectNats()
	if err != nil {
		return fmt.Errorf("connect_nats_error: %w", err)
	}
	statTick := time.Tick(c.cfg.Service.LogStatPeriod)
	//statTick := time.Tick(time.Second * 20)
	go func() {
		c.errCh <- c.binClient.Serve()
	}()
	go func() {
		c.errCh <- c.ftxClient.Serve()
	}()
	for {
		select {
		case err := <-c.binClient.ReadErr:
			c.log.Warn("binance_read_error", zap.Error(err))
		case event := <-c.binClient.EventCh:
			c.lastBinTime = event.ReceiveTime
			c.binanceHandler(&event)
		case event := <-c.ftxClient.EventCh:
			c.fxtHandler(&event)
		case <-statTick:
			c.printStat()
		case err := <-c.errCh:
			return err
		case <-c.ctx.Done():
			return c.ctx.Err()
		}
	}
}

func (c *Collector) printStat() {
	start := time.Now()
	t := tabby.New()
	t.AddHeader("symbol", "ma", "count", "min", "max", "bin_all")
	for _, m := range c.cfg.Markets {
		symbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usdt)
		ma := c.maPriceMap[symbol]
		min, _ := ma.Min()
		max, _ := ma.Max()
		t.AddLine(symbol,
			fmt.Sprintf("%.5f", ma.Avg()),
			ma.Count(),
			//ma.SlotsFilled(),
			fmt.Sprintf("%.5f", min),
			fmt.Sprintf("%.5f", max),
			c.ftxCountMap[symbol].Load(),
		)
		c.ftxCountMap[symbol].Store(0)
	}
	if c.cfg.Service.EnableUSD {
		for _, m := range c.cfg.Markets {
			symbol := fmt.Sprintf("%s/%s", strings.ToUpper(m), usd)
			ma := c.maPriceMap[symbol]
			min, _ := ma.Min()
			max, _ := ma.Max()
			t.AddLine(symbol,
				fmt.Sprintf("%.5f", ma.Avg()),
				ma.Count(),
				//ma.SlotsFilled(),
				fmt.Sprintf("%.5f", min),
				fmt.Sprintf("%.5f", max),
				c.ftxCountMap[symbol].Load(),
			)
			c.ftxCountMap[symbol].Store(0)
		}
	}

	//for symbol, ma := range c.maPriceMap {
	//	min, _ := ma.Min()
	//	max, _ := ma.Max()
	//	t.AddLine(symbol,
	//		ma.Avg(),
	//		ma.Count(),
	//		ma.SlotsFilled(),
	//		min,
	//		max,
	//	)
	//}
	t.Print()
	c.log.Info("stat",
		zap.Int64("bin_all", binAll.Load()),
		zap.Int64("bin_send", binSend.Load()),
		zap.Int64("ftx_all", ftxAll.Load()),
		zap.Int64("ftx_send", ftxSend.Load()),
		zap.Duration("avg_ftx_delay", avgDelay(ftxDelayList)),
		zap.Duration("stat_elapsed", time.Since(start)),
		zap.Any("usdt_price", c.usdtPrice),
	)
	//c.log.Info("elapsed", zap.Duration("", time.Since(start)))

	binAll.Store(0)
	binSend.Store(0)
	ftxAll.Store(0)
	ftxSend.Store(0)
	ftxDelayList = nil
}
func (c *Collector) Close() error {
	c.log.Info("close_collector")
	c.ec.Close()
	return nil
}

func (c *Collector) connectNats() error {
	url := fmt.Sprintf("nats://%s:%s", c.cfg.Nats.Host, c.cfg.Nats.Port)
	nc, err := nats.Connect(url)
	if err != nil {
		return fmt.Errorf("connect_nats_error: %w", err)
	}
	ec, err := nats.NewEncodedConn(nc, nats.GOB_ENCODER)
	if err != nil {
		return fmt.Errorf("encoded_connection_error: %w", err)
	}
	c.ec = ec
	return nil
}
func (c *Collector) errHandler(err error) {
	c.log.Error("binance_ws_error", zap.Error(err))
}
func avgDelay(list []int64) time.Duration {
	if len(list) == 0 {
		return 0
	}
	var total int64
	for i := 0; i < len(list); i++ {
		total = total + list[i]
	}
	return time.Duration(total / int64(len(list)))
}

func (c *Collector) send(t *TickerData, e string, symbol string) {
	var sb Surebet
	if e == binanceExchange {
		sb.FtxTicker = c.loadTicker(symbol)
		sb.BinTicker = t
	} else {
		sb.FtxTicker = t
		sb.BinTicker = c.loadBinance(symbol)
	}
	if sb.FtxTicker.AskPrice.IsZero() || sb.BinTicker.AskPrice.IsZero() {
		return
	}
	sb.UsdtPrice = c.usdtPrice

	avgFtx := decimal.Avg(sb.FtxTicker.AskPrice, sb.FtxTicker.BidPrice)
	if strings.Index(sb.FtxTicker.Symbol, usdt) == -1 {
		avgFtx = avgFtx.Div(sb.UsdtPrice)
	}

	diff := decimal.Avg(sb.BinTicker.AskPrice, sb.BinTicker.BidPrice).Sub(avgFtx).Div(avgFtx).Mul(d100)
	ma := c.maPriceMap[sb.FtxTicker.Symbol]
	ma.Add(diff.InexactFloat64())

	if !ma.SlotsFilled() {
		return
	}
	sb.LastBinTime = c.lastBinTime
	sb.ID = c.createID(t.ReceiveTime)
	sb.AvgPriceDiff = decimal.NewFromFloat(ma.Avg()).Round(4)
	max, _ := ma.Max()
	min, _ := ma.Min()
	sb.MaxPriceDiff = decimal.NewFromFloat(max).Round(4)
	sb.MinPriceDiff = decimal.NewFromFloat(min).Round(4)
	err := c.ec.Publish(cryptoSubject, &sb)
	if err != nil {
		c.log.Warn("send_error", zap.Error(err))
	}
	//if e == binanceExchange {
	//	c.log.Info("sb",
	//		zap.Any("e", e),
	//		zap.Any("symbol", symbol),
	//		zap.Any("id", sb.ID),
	//		zap.Any("ftx", sb.FtxTicker),
	//	)
	//}

}

func (c *Collector) createID(id int64) int64 {
	c.idLock.Lock()
	defer c.idLock.Unlock()
	if id <= c.lastID {
		id = c.lastID + 1
	}
	c.lastID = id
	return id
}
