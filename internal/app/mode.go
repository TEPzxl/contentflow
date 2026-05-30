package app

import (
	"fmt"
	"strings"
)

type runtimePlan struct {
	HTTP      bool
	Scheduler bool
	Worker    bool
}

func (p runtimePlan) runsOutboxDispatcher() bool {
	return p.Scheduler
}

func runtimePlanForMode(mode string) (runtimePlan, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "all":
		return runtimePlan{HTTP: true, Scheduler: true, Worker: true}, nil
	case "api":
		return runtimePlan{HTTP: true}, nil
	case "scheduler":
		return runtimePlan{Scheduler: true}, nil
	case "worker":
		return runtimePlan{Worker: true}, nil
	default:
		return runtimePlan{}, fmt.Errorf("unsupported app mode %q", mode)
	}
}
