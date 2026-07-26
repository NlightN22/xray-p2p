package service

import (
	"context"
	"time"
)

const DefaultShutdownTimeout = 5 * time.Second

func StopWithTimeout(stop StopFunc) error {
	if stop == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()
	return stop(ctx)
}
