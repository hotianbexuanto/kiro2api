package service

import (
	"context"
	"sync"
)

type semaphore struct {
	ch  chan struct{}
	cap int
}

var (
	tokenSemMu   sync.Mutex
	tokenSems    = make(map[string]*semaphore)
	groupSemMu   sync.Mutex
	groupSems    = make(map[string]*semaphore)
	tokenLimitMu sync.Mutex
	tokenLimit   = make(map[string]*RateLimiter)
)

func acquireSemaphore(ctx context.Context, semMap map[string]*semaphore, key string, cap int, mu *sync.Mutex) (release func(), ok bool) {
	if cap <= 0 || key == "" {
		return func() {}, true
	}

	mu.Lock()
	sem := semMap[key]
	if sem == nil || sem.cap != cap {
		sem = &semaphore{ch: make(chan struct{}, cap), cap: cap}
		semMap[key] = sem
	}
	mu.Unlock()

	select {
	case sem.ch <- struct{}{}:
		return func() { <-sem.ch }, true
	case <-ctx.Done():
		return func() {}, false
	}
}

func getTokenLimiter(key string, qps float64, burst int) *RateLimiter {
	if key == "" || qps <= 0 || burst <= 0 {
		return nil
	}

	tokenLimitMu.Lock()
	defer tokenLimitMu.Unlock()

	rl := tokenLimit[key]
	if rl == nil {
		rl = NewRateLimiter(qps, burst)
		tokenLimit[key] = rl
		return rl
	}
	rl.SetRate(qps, burst)
	return rl
}
