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
	defaultListenAddress       = ":7000"
	defaultServerT3            = 10 * time.Second
	defaultEventInterval       = 2 * time.Second
	defaultEventInitialDelay   = 1 * time.Second
	defaultSelectedPoll        = 500 * time.Millisecond
	defaultServerItemAEncoding = "GBK"
)

type serverOptions struct {
	Address              string
	DeviceID             uint16
	T3                   time.Duration
	EventInterval        time.Duration
	EventInitialDelay    time.Duration
	SelectedPollInterval time.Duration
	EnablePublisher      bool
	ItemAEncoding        string
	LogLevel             string
}

func defaultServerOptions() serverOptions {
	return serverOptions{
		Address:              defaultListenAddress,
		DeviceID:             0,
		T3:                   defaultServerT3,
		EventInterval:        defaultEventInterval,
		EventInitialDelay:    defaultEventInitialDelay,
		SelectedPollInterval: defaultSelectedPoll,
		EnablePublisher:      true,
		ItemAEncoding:        defaultServerItemAEncoding,
		LogLevel:             "info",
	}
}

func parseServerOptions(args []string) (serverOptions, error) {
	fs := flag.NewFlagSet("secs4go-example-server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := defaultServerOptions()
	var (
		deviceID uint
		err      error
	)

	fs.StringVar(&opts.Address, "addr", opts.Address, "listen address")
	fs.UintVar(&deviceID, "device-id", uint(opts.DeviceID), "server device id")
	fs.DurationVar(&opts.T3, "t3", opts.T3, "reply timeout")
	fs.DurationVar(&opts.EventInterval, "event-interval", opts.EventInterval, "interval between demo events")
	fs.DurationVar(&opts.EventInitialDelay, "event-initial-delay", opts.EventInitialDelay, "delay before first demo event after selection")
	fs.DurationVar(&opts.SelectedPollInterval, "selected-poll-interval", opts.SelectedPollInterval, "poll interval while waiting for selected state")
	fs.BoolVar(&opts.EnablePublisher, "enable-publisher", opts.EnablePublisher, "enable demo event publisher")
	fs.StringVar(&opts.ItemAEncoding, "item-a-encoding", opts.ItemAEncoding, "Item A encoding")
	fs.StringVar(&opts.LogLevel, "log-level", opts.LogLevel, "log level (debug/info/warn/error)")

	if err := fs.Parse(args); err != nil {
		return serverOptions{}, err
	}

	if deviceID > uint(^uint16(0)) {
		return serverOptions{}, fmt.Errorf("device-id must be <= %d", uint(^uint16(0)))
	}
	opts.DeviceID = uint16(deviceID)
	opts.Address, err = sharedcfg.NormalizeAddress(opts.Address)
	if err != nil {
		return serverOptions{}, err
	}
	opts.ItemAEncoding = sharedcfg.NormalizeItemAEncoding(opts.ItemAEncoding, defaultServerItemAEncoding)
	if opts.T3 <= 0 {
		return serverOptions{}, fmt.Errorf("t3 must be positive")
	}
	if opts.EventInterval <= 0 {
		return serverOptions{}, fmt.Errorf("event-interval must be positive")
	}
	if opts.EventInitialDelay < 0 {
		return serverOptions{}, fmt.Errorf("event-initial-delay cannot be negative")
	}
	if opts.SelectedPollInterval <= 0 {
		return serverOptions{}, fmt.Errorf("selected-poll-interval must be positive")
	}
	opts.LogLevel, err = sharedcfg.NormalizeLogLevel(opts.LogLevel)
	if err != nil {
		return serverOptions{}, err
	}

	return opts, nil
}

func buildServerConfig(opts serverOptions) *secs4go.Config {
	config := secs4go.DefaultConfig(opts.Address)
	config.T3 = opts.T3
	config.DeviceID = opts.DeviceID
	config.IsActive = false
	config.EnableHeartbeat = false
	config.ItemAEncoding = opts.ItemAEncoding
	return config
}
