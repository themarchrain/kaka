package memory

import (
	"errors"
	"strings"
)

// ErrInvalidKey 表示限流 key 为空或只包含空白字符。
var ErrInvalidKey = errors.New("memory: invalid key")

func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrInvalidKey
	}
	return nil
}
