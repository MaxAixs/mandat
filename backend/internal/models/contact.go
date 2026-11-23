package models

import (
	"time"

	"github.com/google/uuid"
)

// ContactType selects how a contact value is validated, shown and linked.
type ContactType string

// Supported contact types; the contacts table check mirrors this list.
const (
	ContactEmail     ContactType = "email"
	ContactTelegram  ContactType = "telegram"
	ContactInstagram ContactType = "instagram"
	ContactThreads   ContactType = "threads"
	ContactFacebook  ContactType = "facebook"
	ContactPhone     ContactType = "phone"
	ContactWebsite   ContactType = "website"
)

// Contact is one entry of the contacts page, ordered by Position ascending.
// Label is optional and replaces the type name on the page. Display and Href are
// derived from Type and Value by the service and are not stored.
type Contact struct {
	ID        uuid.UUID
	Type      ContactType
	Value     string
	Label     string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
	Display   string
	Href      string
}
