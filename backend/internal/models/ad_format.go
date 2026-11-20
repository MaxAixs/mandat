package models

import (
	"time"

	"github.com/google/uuid"
)

// AdFormat is one card of the «Formats we work with» block on the advertising page.
// Position orders the cards, ascending; ImageURL is empty when the card has no image.
type AdFormat struct {
	ID          uuid.UUID
	Title       string
	Description string
	ImageURL    string
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
