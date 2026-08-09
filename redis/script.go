package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Script 封装一段 Lua 脚本及其 EVALSHA 缓存。
// 用法：Load 一次缓存 SHA，之后 Run 走 EVALSHA；NOSCRIPT 自动回退重载。
// 并发安全：sha 读写由 RWMutex 保护。
type Script struct {
	mu   sync.RWMutex
	name string
	body string
	sha  string
}

// NewScript 创建脚本对象。
func NewScript(name, body string) *Script {
	return &Script{name: name, body: body}
}

func (s *Script) getSHA() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sha
}

// Load 用 SCRIPT LOAD 加载脚本并缓存 SHA。
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

// Run 执行脚本；sha 未缓存时先 Load，NOSCRIPT 错误时回退重载再执行。
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
