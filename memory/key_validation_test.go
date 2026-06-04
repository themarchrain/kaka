package memory

import (
	"errors"
	"testing"
)

func TestValidateKeyRejectsBlankKeys(t *testing.T) {
	cases := []string{
		"",
		" ",
		"\t",
		"\n",
		" \t\n ",
	}

	for _, key := range cases {
		t.Run("key="+key, func(t *testing.T) {
			if err := validateKey(key); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey, got %v", err)
			}
		})
	}
}

func TestValidateKeyAcceptsNonBlankKeys(t *testing.T) {
	cases := []string{
		"global",
		"user:1",
		" user:1 ",
	}

	for _, key := range cases {
		t.Run("key="+key, func(t *testing.T) {
			if err := validateKey(key); err != nil {
				t.Fatalf("expected valid key, got %v", err)
			}
		})
	}
}
