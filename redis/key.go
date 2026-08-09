package redis

import (
	"fmt"
	"strings"
)

// validateKey 校验业务 key：空/空白 key 返回错误（与 memory 语义一致）。
func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("kaka/redis: key must not be blank")
	}
	return nil
}
