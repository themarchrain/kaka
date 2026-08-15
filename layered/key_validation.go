package layered

import (
	"errors"
	"strings"
)

// ErrInvalidKey is returned when the rate limit key is empty or blank.
var ErrInvalidKey = errors.New("layered: invalid key")

func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrInvalidKey
	}
	return nil
}
