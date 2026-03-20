package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/jianwushu/Secs4go/example/sharedcfg"
	"github.com/jianwushu/Secs4go/secs4go"
)

const (
	defaultServerAddress       = "127.0.0.1:7000"
	defaultClientT3            = 10 * time.Second
	defaultClientItemAEncoding = "GBK"
)

type clientOptions struct {
	Address         string
	DeviceID        uint16
	T3              time.Duration
	ItemAEncoding   string
	EnableHeartbeat bool
	LogLevel        string
}

func defaultClientOptions() clientOptions {
	return clientOptions{
		Address:         defaultServerAddress,
		DeviceID:        0,
		T3:              defaultClientT3,
		ItemAEncoding:   defaultClientItemAEncoding,
		EnableHeartbeat: true,
		LogLevel:        "info",
	}
}

func parseClientOptions(args []string) (clientOptions, error) {
	fs := flag.NewFlagSet("secs4go-example-client", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := defaultClientOptions()
	var (
		deviceID uint
		err      error
	)

	fs.StringVar(&opts.Address, "addr", opts.Address, "server address")
	fs.UintVar(&deviceID, "device-id", uint(opts.DeviceID), "client device id")
	fs.DurationVar(&opts.T3, "t3", opts.T3, "reply timeout")
	fs.StringVar(&opts.ItemAEncoding, "item-a-encoding", opts.ItemAEncoding, "Item A encoding")
	fs.BoolVar(&opts.EnableHeartbeat, "heartbeat", opts.EnableHeartbeat, "enable heartbeat")
	fs.StringVar(&opts.LogLevel, "log-level", opts.LogLevel, "log level (debug/info/warn/error)")

	if err := fs.Parse(args); err != nil {
		return clientOptions{}, err
	}

	if deviceID > uint(^uint16(0)) {
		return clientOptions{}, fmt.Errorf("device-id must be <= %d", uint(^uint16(0)))
	}
	opts.DeviceID = uint16(deviceID)
	opts.Address, err = sharedcfg.NormalizeAddress(opts.Address)
	if err != nil {
		return clientOptions{}, err
	}
	opts.ItemAEncoding = sharedcfg.NormalizeItemAEncoding(opts.ItemAEncoding, defaultClientItemAEncoding)
	if opts.T3 <= 0 {
		return clientOptions{}, fmt.Errorf("t3 must be positive")
	}
	opts.LogLevel, err = sharedcfg.NormalizeLogLevel(opts.LogLevel)
	if err != nil {
		return clientOptions{}, err
	}

	return opts, nil
}

func buildClientConfig(opts clientOptions) *secs4go.Config {
	config := secs4go.DefaultConfig(opts.Address)
	config.DeviceID = opts.DeviceID
	config.T3 = opts.T3
	config.IsActive = true
	config.EnableHeartbeat = opts.EnableHeartbeat
	config.ItemAEncoding = opts.ItemAEncoding
	return config
}
