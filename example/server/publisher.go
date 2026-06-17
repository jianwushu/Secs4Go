package main

import (
	"context"
	"fmt"
	"log"
	"time"

	secs4go "github.com/jianwushu/secs4go/core"
)

type eventPublisher struct {
	interval             time.Duration
	initialDelay         time.Duration
	selectedPollInterval time.Duration
	counter              int
	isSelected           func() bool
	updateDV             func(string, interface{}) error
	trigger              func() (*secs4go.Message, error)
	send                 func(*secs4go.Message) error
	logf                 func(string, ...interface{})
}

func newEventPublisher(opts serverOptions, server *secs4go.SecsGem) *eventPublisher {
	return &eventPublisher{
		interval:     opts.EventInterval,
		initialDelay: opts.EventInitialDelay,
		// selectedPollInterval: opts.SelectedPollInterval,
		counter:    10,
		isSelected: server.IsSelected,
		updateDV:   UpdateDv,
		trigger: func() (*secs4go.Message, error) {
			return TriggerEvent("10020")
		},
		send: func(msg *secs4go.Message) error {
			_, err := server.Send(msg)
			return err
		},
		logf: log.Printf,
	}
}

func (p *eventPublisher) run(ctx context.Context) {
	if p.logf == nil {
		p.logf = log.Printf
	}

	delay := p.initialDelay
	if delay <= 0 {
		delay = p.interval
	}
	if p.selectedPollInterval <= 0 {
		p.selectedPollInterval = defaultSelectedPoll
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if !p.isSelected() {
				timer.Reset(p.selectedPollInterval)
				continue
			}

			if err := p.publishOnce(); err != nil {
				p.logf("事件发送失败: %v", err)
			}
			timer.Reset(p.interval)
		}
	}
}

func (p *eventPublisher) publishOnce() error {
	p.counter++
	if err := p.updateDV("1001", fmt.Sprintf("CRR_好_%d", p.counter)); err != nil {
		return fmt.Errorf("更新DV 1001失败: %w", err)
	}

	msg, err := p.trigger()
	if err != nil {
		return fmt.Errorf("触发10020失败: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("触发10020未生成消息")
	}
	if err := p.send(msg); err != nil {
		return fmt.Errorf("发送S6F11失败: %w", err)
	}
	return nil
}
