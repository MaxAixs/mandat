package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// Limits of a contact, mirrored by the contacts table checks.
const (
	ContactLabelMaxLen   = 40
	contactEmailMaxLen   = 254
	contactWebsiteMaxLen = 200
	contactPhoneMinDigit = 7
	contactPhoneMaxDigit = 15
)

// Handle shapes of the social contact types.
var (
	telegramHandle  = regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)
	instagramHandle = regexp.MustCompile(`^[A-Za-z0-9._]{1,30}$`)
	facebookHandle  = regexp.MustCompile(`^[A-Za-z0-9.]{5,50}$`)
	phoneChars      = regexp.MustCompile(`^\+?[0-9 ()\-]+$`)
)

// socialProfile describes a social contact type: the handle shape, the only host
// accepted in pasted URLs, and how the profile link and display text are built.
type socialProfile struct {
	handle  *regexp.Regexp
	host    string
	link    string
	showsAt bool
}

// socialProfiles lists the contact types whose value is a profile handle.
var socialProfiles = map[models.ContactType]socialProfile{
	models.ContactTelegram:  {handle: telegramHandle, host: "t.me", link: "https://t.me/", showsAt: true},
	models.ContactInstagram: {handle: instagramHandle, host: "instagram.com", link: "https://www.instagram.com/", showsAt: true},
	models.ContactThreads:   {handle: instagramHandle, host: "threads.net", link: "https://www.threads.net/@", showsAt: true},
	models.ContactFacebook:  {handle: facebookHandle, host: "facebook.com", link: "https://www.facebook.com/"},
}

// ContactRepository is the persistence contract of contacts.
type ContactRepository interface {
	List(ctx context.Context) ([]*models.Contact, error)
	// Create appends the contact after the last one and sets its Position.
	Create(ctx context.Context, contact *models.Contact) error
	// Update saves type, value, label and updated_at, and reads back Position and CreatedAt.
	Update(ctx context.Context, contact *models.Contact) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Move swaps the contact with its neighbour above (up) or below; no neighbour is a no-op.
	Move(ctx context.Context, id uuid.UUID, up bool) error
}

// ContactInput is the editable payload of a contact.
type ContactInput struct {
	Type  string
	Value string
	Label string
}

// ContactService implements the business rules of contacts: per-type validation,
// normalization of the value, and the derived display text and link.
type ContactService struct {
	repo   ContactRepository
	logger *slog.Logger
}

// NewContactService creates a ContactService.
func NewContactService(repo ContactRepository, logger *slog.Logger) *ContactService {
	return &ContactService{repo: repo, logger: logger}
}

// List returns all contacts ordered by position, with Display and Href filled in.
func (s *ContactService) List(ctx context.Context) ([]*models.Contact, error) {
	const fc = "contactService_List"

	contacts, err := s.repo.List(ctx)
	if err != nil {
		s.logFailure(ctx, "failed to list contacts", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	for _, contact := range contacts {
		describeContact(contact)
	}

	return contacts, nil
}

// Create validates, normalizes and stores a new contact at the end of the list.
func (s *ContactService) Create(ctx context.Context, input ContactInput) (*models.Contact, error) {
	const fc = "contactService_Create"

	contact, err := buildContact(input)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	now := time.Now().UTC()
	contact.ID = uuid.New()
	contact.CreatedAt = now
	contact.UpdatedAt = now
	if err := s.repo.Create(ctx, contact); err != nil {
		s.logFailure(ctx, "failed to create contact", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return contact, nil
}

// Update validates, normalizes and saves the contact.
func (s *ContactService) Update(ctx context.Context, id uuid.UUID, input ContactInput) (*models.Contact, error) {
	const fc = "contactService_Update"

	contact, err := buildContact(input)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	contact.ID = id
	contact.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, contact); err != nil {
		s.logFailure(ctx, "failed to update contact", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return contact, nil
}

// Delete removes the contact.
func (s *ContactService) Delete(ctx context.Context, id uuid.UUID) error {
	const fc = "contactService_Delete"

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logFailure(ctx, "failed to delete contact", fc, err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// Move shifts the contact one place up or down.
func (s *ContactService) Move(ctx context.Context, id uuid.UUID, direction string) error {
	const fc = "contactService_Move"

	if direction != MoveUp && direction != MoveDown {
		return fmt.Errorf("%s: %w", fc, errs.NewValidationError("direction", "must be up or down"))
	}

	if err := s.repo.Move(ctx, id, direction == MoveUp); err != nil {
		s.logFailure(ctx, "failed to move contact", fc, err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// logFailure logs unexpected failures with fc; client errors stay quiet.
func (s *ContactService) logFailure(ctx context.Context, msg string, fc string, err error) {
	if errors.Is(err, errs.ErrContactNotFound) || errors.Is(err, errs.ErrInvalidInput) || errors.Is(err, errs.ErrConflict) {
		return
	}

	s.logger.ErrorContext(ctx, msg, "fc", fc, "error", err)
}

// describeContact fills the display text and the link derived from the stored value.
func describeContact(contact *models.Contact) {
	contact.Display = contact.Value
	contact.Href = contact.Value

	if profile, ok := socialProfiles[contact.Type]; ok {
		contact.Href = profile.link + contact.Value
		if profile.showsAt {
			contact.Display = "@" + contact.Value
		}

		return
	}

	switch contact.Type {
	case models.ContactEmail:
		contact.Href = "mailto:" + contact.Value
	case models.ContactPhone:
		prefix := ""
		if strings.HasPrefix(contact.Value, "+") {
			prefix = "+"
		}
		contact.Href = "tel:" + prefix + phoneDigits(contact.Value)
	case models.ContactWebsite:
		if parsed, err := url.Parse(contact.Value); err == nil {
			contact.Display = parsed.Hostname()
		}
	}
}

// buildContact validates the input and returns the normalized contact with Display and Href.
func buildContact(input ContactInput) (*models.Contact, error) {
	contactType := models.ContactType(strings.TrimSpace(input.Type))
	label := strings.TrimSpace(input.Label)
	if utf8.RuneCountInString(label) > ContactLabelMaxLen {
		return nil, errs.NewValidationError("label", fmt.Sprintf("label must be at most %d characters", ContactLabelMaxLen))
	}

	value, err := normalizeContactValue(contactType, strings.TrimSpace(input.Value))
	if err != nil {
		return nil, err
	}

	contact := &models.Contact{Type: contactType, Value: value, Label: label}
	describeContact(contact)

	return contact, nil
}

// normalizeContactValue checks the value against its type and returns the form to store.
func normalizeContactValue(contactType models.ContactType, value string) (string, error) {
	if profile, ok := socialProfiles[contactType]; ok {
		return normalizeHandle(profile, value)
	}

	switch contactType {
	case models.ContactEmail:
		address, err := mail.ParseAddress(value)
		if err != nil || address.Address != value || utf8.RuneCountInString(value) > contactEmailMaxLen {
			return "", errs.NewValidationError("value", "must be an email address like name@example.com")
		}

		return value, nil
	case models.ContactPhone:
		digits := len(phoneDigits(value))
		if !phoneChars.MatchString(value) || digits < contactPhoneMinDigit || digits > contactPhoneMaxDigit {
			return "", errs.NewValidationError(
				"value",
				fmt.Sprintf("must be a phone number with %d–%d digits", contactPhoneMinDigit, contactPhoneMaxDigit),
			)
		}

		return value, nil
	case models.ContactWebsite:
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || utf8.RuneCountInString(value) > contactWebsiteMaxLen {
			return "", errs.NewValidationError("value", "must be an https:// link")
		}

		return value, nil
	default:
		return "", errs.NewValidationError("type", "unknown contact type")
	}
}

// normalizeHandle accepts a handle, @handle or a profile URL on the type's own host
// and returns the bare handle.
func normalizeHandle(profile socialProfile, value string) (string, error) {
	handle := strings.TrimPrefix(value, "@")
	if strings.ContainsAny(value, "/:") {
		path, ok := profilePath(profile.host, value)
		if !ok {
			return "", errs.NewValidationError("value", "must be a handle or a link on "+profile.host)
		}
		handle = strings.TrimPrefix(path, "@")
	}

	if !profile.handle.MatchString(handle) {
		return "", errs.NewValidationError("value", "is not a valid handle for "+profile.host)
	}

	return handle, nil
}

// profilePath returns the single path segment of a profile URL on host (with or
// without www. and the scheme); other hosts, schemes and nested paths are rejected.
func profilePath(host string, value string) (string, bool) {
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", false
	}
	if strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.") != host {
		return "", false
	}

	path := strings.Trim(parsed.Path, "/")
	if path == "" || strings.Contains(path, "/") {
		return "", false
	}

	return path, true
}

// phoneDigits keeps only the digits of a phone number.
func phoneDigits(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, value)
}
