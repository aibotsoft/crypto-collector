package config

import (
	"github.com/cristalhq/aconfig"
	"github.com/cristalhq/aconfig/aconfigyaml"
	"time"
)

type Config struct {
	Service struct {
		Name          string        `json:"name" default:"crypto-collector"`
		LogStatPeriod time.Duration `json:"log_stat_period" default:"1m"`
		EnableUSD     bool          `json:"enable_usd"`
		AverageWindow int           `json:"average_window"`
	} `json:"service"`
	Zap struct {
		Level         string `json:"level" default:"info"`
		Encoding      string `json:"encoding" default:"console"`
		DisableCaller bool   `json:"disable_caller" default:"true"`
	} `json:"zap"`
	Binance struct {
		Name   string `json:"name" default:"binance"`
		WsHost string `json:"ws_host" default:"wss://stream.binance.com:9443/stream?streams="`
		Debug  bool   `json:"debug" default:"false"`
	} `json:"binance"`
	Ftx struct {
		Name   string `json:"name" default:"ftx"`
		WsHost string `json:"ws_host" default:"wss://ftx.com/ws/"`
	} `json:"fxt"`

	Ws struct {
		ConnTimeout time.Duration `json:"conn_timeout" default:"5s"`
	} `json:"ws"`
	Nats struct {
		Host string `json:"host"`
		Port string `json:"port"`
	} `json:"nats"`
	Markets []string `json:"markets"`
}

func NewConfig() *Config {
	var cfg Config
	loader := aconfig.LoaderFor(&cfg, aconfig.Config{
		//SkipFlags:          true,
		AllFieldRequired:   true,
		AllowUnknownFlags:  true,
		AllowUnknownEnvs:   true,
		AllowUnknownFields: true,
		AllowDuplicates:    true,
		SkipEnv:            false,
		FileFlag:           "config",
		FailOnFileNotFound: false,
		MergeFiles:         true,
		Files:              []string{"config.yaml", "crypto-collector.yaml"},
		FileDecoders: map[string]aconfig.FileDecoder{
			".yaml": aconfigyaml.New(),
		},
	})
	err := loader.Load()
	if err != nil {
		panic(err)
	}

	return &cfg
}
