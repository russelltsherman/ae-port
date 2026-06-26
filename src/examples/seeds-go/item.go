package seeds

import (
	"errors"
	"strings"
	"time"
)

// MaxNameLen is the longest permitted item name, in characters.
const MaxNameLen = 100

// Item is a single todo entry.
type Item struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
}

// Validation errors returned by ValidateName.
var (
	ErrNameRequired = errors.New("name is required")
	ErrNameTooLong  = errors.New("name must be at most 100 characters")
)

// ValidateName checks a proposed item name. The name is trimmed of surrounding
// whitespace before validation; an all-whitespace name is treated as empty.
func ValidateName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrNameRequired
	}
	if len([]rune(trimmed)) > MaxNameLen {
		return ErrNameTooLong
	}
	return nil
}
