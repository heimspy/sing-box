package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"time"
)

// helperContext closes the service when the owning Heimspy process disappears.
// An unowned sing-box invocation retains the upstream lifecycle.
func helperContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(parent)
	if owner := os.Getenv("HEIMSPY_HELPER_PARENT"); owner != "" {
		pid, err := strconv.Atoi(owner)
		if err != nil || pid <= 1 || os.Getppid() != pid {
			cancel()
			return nil, nil, errors.New("helper parent is no longer alive")
		}
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if os.Getppid() != pid {
						cancel()
						return
					}
				}
			}
		}()
	}
	return ctx, cancel, nil
}

// Only the owner of stdin may read it. Inspector services consume IPC themselves.
func watchHelperInput(cancel context.CancelFunc) {
	if os.Getenv("HEIMSPY_HELPER_STDIN") == "1" {
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	}
}
