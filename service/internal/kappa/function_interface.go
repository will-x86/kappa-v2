package kappa

import (
	"context"
	"time"
)

type Function interface {
	Start(ctx context.Context) error
	Stop() error
	Invoke(ctx context.Context, event KappaEvent) (*KappaResponse, error)
	GetLogs() []string
	IsRunning() bool
	SetIdleTimeout(duration time.Duration)
	//resetIdleTimer()
	//cancelIdleTimer()
	// Any other methods from KappaFunction that KappaService needs
}
