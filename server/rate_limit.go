package server

import (
	"sync"
	"time"
)

type requestRateLimit struct {
	sync.Mutex
	lastRequest map[string]time.Time
}

func (rlimit *requestRateLimit)CheckAndUpdate(id string, reqPerSecond float64) bool {
	rlimit.Lock()
	defer rlimit.Unlock()

	if rlimit.lastRequest == nil {
		rlimit.lastRequest = make(map[string]time.Time)
	}

	minInterval := time.Duration(float64(time.Second) * 1/reqPerSecond)

	if t, ok := rlimit.lastRequest[id]; ok {
		if time.Now().Sub(t) < minInterval {
			return false
		}
	}

	rlimit.lastRequest[id] = time.Now()
	return true
}
