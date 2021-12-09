package server

import (
	"sync"
	"time"
)

type requestRateLimit struct {
	sync.Mutex
	lastRequest map[string]time.Time
	minInterval map[string]time.Duration
}

func (rlimit *requestRateLimit)CheckAndUpdate(id string) bool {
	rlimit.Lock()
	defer rlimit.Unlock()

	if rlimit.lastRequest == nil {
		rlimit.lastRequest = make(map[string]time.Time)
		rlimit.minInterval = make(map[string]time.Duration)
	}

	if t, ok := rlimit.lastRequest[id]; ok {
		if time.Now().Sub(t) < rlimit.minInterval[id] {
			return false
		}
	}

	rlimit.lastRequest[id] = time.Now()
	return true
}

func (rlimit *requestRateLimit)setRequestRate(id string, reqPerSecond float64) {
	rlimit.Lock()
	defer rlimit.Unlock()

	if rlimit.lastRequest == nil {
		rlimit.lastRequest = make(map[string]time.Time)
		rlimit.minInterval = make(map[string]time.Duration)
	}

	rlimit.minInterval[id] = time.Duration(float64(time.Second) * 1/reqPerSecond)
}
