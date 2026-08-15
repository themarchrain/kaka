package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Script wraps a Lua script and its EVALSHA cache.
// Usage: call Load once to cache the SHA, then Run executes via EVALSHA; NOSCRIPT
// errors fall back to reloading automatically.
// It is safe for concurrent use; the SHA is guarded by an RWMutex.
type Script struct {
	mu   sync.RWMutex
	name string
	body string
	sha  string
}

// NewScript creates a Script for the given name and Lua body.
func NewScript(name, body string) *Script {
	return &Script{name: name, body: body}
}

func (s *Script) getSHA() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sha
}

// Load runs SCRIPT LOAD and caches the resulting SHA.
func (s *Script) Load(ctx context.Context, client *redis.Client) error {
	sha, err := client.ScriptLoad(ctx, s.body).Result()
	if err != nil {
		return fmt.Errorf("%w: load %s: %v", ErrRedisUnavailable, s.name, err)
	}
	s.mu.Lock()
	s.sha = sha
	s.mu.Unlock()
	return nil
}

// Run executes the script, loading it first when the SHA is not cached and reloading on NOSCRIPT errors.
func (s *Script) Run(ctx context.Context, client *redis.Client, keys []string, args ...interface{}) (interface{}, error) {
	if s.getSHA() == "" {
		if err := s.Load(ctx, client); err != nil {
			return nil, err
		}
	}
	res, err := client.EvalSha(ctx, s.getSHA(), keys, args...).Result()
	if err == nil {
		return res, nil
	}
	if isNoScriptErr(err) {
		if loadErr := s.Load(ctx, client); loadErr != nil {
			return nil, loadErr
		}
		res, err = client.EvalSha(ctx, s.getSHA(), keys, args...).Result()
	}
	if err != nil {
		return nil, fmt.Errorf("%w: run %s: %v", ErrScript, s.name, err)
	}
	return res, nil
}

func isNoScriptErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "NOSCRIPT")
}
