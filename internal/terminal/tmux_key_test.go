package terminal

import (
	"errors"
	"testing"
)

func TestValidateKeyName(t *testing.T) {
	valid := []string{"Up", "Down", "Left", "Right", "Enter", "Escape", "Tab",
		"BTab", "Space", "PPage", "NPage", "Home", "End", "1", "2", "3", "4", "q", "?"}
	for _, name := range valid {
		if err := ValidateKeyName(name); err != nil {
			t.Errorf("ValidateKeyName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"", "C-b", "C-c", "M-x", "up", "Enter Enter", "\n", "\x00", "ab"}
	for _, name := range invalid {
		err := ValidateKeyName(name)
		if err == nil {
			t.Errorf("ValidateKeyName(%q) = nil, want error", name)
			continue
		}
		if !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("ValidateKeyName(%q) = %v, want ErrInvalidCommand", name, err)
		}
	}
}
