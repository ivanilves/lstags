package cache

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ivanilves/lstags/api/v1/registry/client/auth"
)

// WaitBetween defines how much we will wait between batches of requests
var WaitBetween time.Duration

// Token is a structure to hold already obtained tokens
// Prevents excess HTTP requests to be made (error 429)
var Token = token{items: make(map[string]auth.Token)}

type token struct {
	items map[string]auth.Token
	mux   sync.Mutex
}

// Exists tells if passed key is already present in cache
func (t *token) Exists(key string) bool {
	t.mux.Lock()
	defer t.mux.Unlock()

	_, defined := t.items[key]

	if !defined && WaitBetween != 0 {
		log.Debugf("[EXISTS] Locking token operations for %v (key: %s)", WaitBetween, key)
		time.Sleep(WaitBetween)
	}

	return defined
}

// Get gets token for a passed key
func (t *token) Get(key string) auth.Token {
	t.mux.Lock()
	defer t.mux.Unlock()

	if WaitBetween != 0 {
		log.Debugf("[GET] Locking token operations for %v (key: %s)", WaitBetween, key)
		time.Sleep(WaitBetween)
	}

	return t.items[key]
}

// Get sets token for a passed key
func (t *token) Set(key string, value auth.Token) {
	t.mux.Lock()

	t.items[key] = value

	t.mux.Unlock()
}
