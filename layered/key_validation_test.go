package layered

import (
	"errors"
	"testing"
)

func TestValidateKey(t *testing.T) {
	for _, key := range []string{"user:1", "a", "  a  ", "\tuser\n"} {
		if err := validateKey(key); err != nil {
			t.Errorf("validateKey(%q) unexpected error: %v", key, err)
		}
	}
	for _, key := range []string{"", "   ", "\t\n"} {
		if err := validateKey(key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("validateKey(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
}
