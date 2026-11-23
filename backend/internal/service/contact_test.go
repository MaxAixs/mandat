package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeContactRepo is an in-memory ContactRepository for service unit tests.
type fakeContactRepo struct {
	items       map[uuid.UUID]*models.Contact
	order       []uuid.UUID
	listErr     error
	createErr   error
	updateErr   error
	deleteErr   error
	moveErr     error
	createCalls int
	updateCalls int
	deleteCalls int
	moveCalls   int
	lastMoveID  uuid.UUID
	lastMoveUp  bool
}

func newFakeContactRepo() *fakeContactRepo {
	return &fakeContactRepo{items: map[uuid.UUID]*models.Contact{}}
}

// List returns the seeded contacts in insertion order.
func (r *fakeContactRepo) List(_ context.Context) ([]*models.Contact, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make([]*models.Contact, 0, len(r.order))
	for _, id := range r.order {
		copied := *r.items[id]
		out = append(out, &copied)
	}

	return out, nil
}

// Create appends the contact and assigns its ID and position.
func (r *fakeContactRepo) Create(_ context.Context, contact *models.Contact) error {
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	if contact.ID == uuid.Nil {
		contact.ID = uuid.New()
	}
	contact.Position = len(r.items) + 1
	stored := *contact
	r.items[contact.ID] = &stored
	r.order = append(r.order, contact.ID)

	return nil
}

// Update replaces a stored contact, reading Position and CreatedAt back
// into the passed struct like the SQL UPDATE ... RETURNING does.
func (r *fakeContactRepo) Update(_ context.Context, contact *models.Contact) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	stored, ok := r.items[contact.ID]
	if !ok {
		return errs.ErrContactNotFound
	}
	contact.Position = stored.Position
	contact.CreatedAt = stored.CreatedAt
	next := *contact
	r.items[contact.ID] = &next

	return nil
}

// Delete removes a stored contact.
func (r *fakeContactRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.deleteCalls++
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.items[id]; !ok {
		return errs.ErrContactNotFound
	}
	delete(r.items, id)
	for i, oid := range r.order {
		if oid == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}

	return nil
}

// Move records the direction for assertions.
func (r *fakeContactRepo) Move(_ context.Context, id uuid.UUID, up bool) error {
	r.moveCalls++
	r.lastMoveID = id
	r.lastMoveUp = up
	if r.moveErr != nil {
		return r.moveErr
	}
	if _, ok := r.items[id]; !ok {
		return errs.ErrContactNotFound
	}

	return nil
}

// seedContact inserts a contact directly, bypassing service validation.
func seedContact(r *fakeContactRepo, contactType models.ContactType, value, label string) *models.Contact {
	contact := &models.Contact{
		ID:       uuid.New(),
		Type:     contactType,
		Value:    value,
		Label:    label,
		Position: len(r.items) + 1,
	}
	stored := *contact
	r.items[contact.ID] = &stored
	r.order = append(r.order, contact.ID)

	return contact
}

func newContactTestService(repo ContactRepository) *ContactService {
	return NewContactService(repo, slog.New(&captureHandler{}))
}

func validContactInput() ContactInput {
	return ContactInput{Type: "email", Value: "info@mandat.ua"}
}

func TestContact_Create_NormalizesValidValues(t *testing.T) {
	tests := []struct {
		name        string
		input       ContactInput
		wantValue   string
		wantDisplay string
		wantHref    string
	}{
		{
			name:        "email trimmed",
			input:       ContactInput{Type: "email", Value: "  Name@Example.com "},
			wantValue:   "Name@Example.com",
			wantDisplay: "Name@Example.com",
			wantHref:    "mailto:Name@Example.com",
		},
		{
			name:        "telegram bare handle",
			input:       ContactInput{Type: "telegram", Value: "mandat"},
			wantValue:   "mandat",
			wantDisplay: "@mandat",
			wantHref:    "https://t.me/mandat",
		},
		{
			name:        "telegram at handle",
			input:       ContactInput{Type: "telegram", Value: "@mandat"},
			wantValue:   "mandat",
			wantDisplay: "@mandat",
			wantHref:    "https://t.me/mandat",
		},
		{
			name:        "telegram https url",
			input:       ContactInput{Type: "telegram", Value: "https://t.me/mandat"},
			wantValue:   "mandat",
			wantDisplay: "@mandat",
			wantHref:    "https://t.me/mandat",
		},
		{
			name:        "telegram bare host url",
			input:       ContactInput{Type: "telegram", Value: "t.me/mandat"},
			wantValue:   "mandat",
			wantDisplay: "@mandat",
			wantHref:    "https://t.me/mandat",
		},
		{
			name:        "instagram at handle",
			input:       ContactInput{Type: "instagram", Value: "@mandat.ua"},
			wantValue:   "mandat.ua",
			wantDisplay: "@mandat.ua",
			wantHref:    "https://www.instagram.com/mandat.ua",
		},
		{
			name:        "instagram https url with trailing slash",
			input:       ContactInput{Type: "instagram", Value: "https://www.instagram.com/mandat.ua/"},
			wantValue:   "mandat.ua",
			wantDisplay: "@mandat.ua",
			wantHref:    "https://www.instagram.com/mandat.ua",
		},
		{
			name:        "instagram bare host url",
			input:       ContactInput{Type: "instagram", Value: "instagram.com/mandat.ua"},
			wantValue:   "mandat.ua",
			wantDisplay: "@mandat.ua",
			wantHref:    "https://www.instagram.com/mandat.ua",
		},
		{
			name:        "threads at handle",
			input:       ContactInput{Type: "threads", Value: "@mandat"},
			wantValue:   "mandat",
			wantDisplay: "@mandat",
			wantHref:    "https://www.threads.net/@mandat",
		},
		{
			name:        "threads https url",
			input:       ContactInput{Type: "threads", Value: "https://www.threads.net/@mandat"},
			wantValue:   "mandat",
			wantDisplay: "@mandat",
			wantHref:    "https://www.threads.net/@mandat",
		},
		{
			name:        "threads bare host url",
			input:       ContactInput{Type: "threads", Value: "threads.net/@mandat"},
			wantValue:   "mandat",
			wantDisplay: "@mandat",
			wantHref:    "https://www.threads.net/@mandat",
		},
		{
			name:        "facebook bare handle",
			input:       ContactInput{Type: "facebook", Value: "mandat.ua"},
			wantValue:   "mandat.ua",
			wantDisplay: "mandat.ua",
			wantHref:    "https://www.facebook.com/mandat.ua",
		},
		{
			name:        "facebook https url",
			input:       ContactInput{Type: "facebook", Value: "https://www.facebook.com/mandat.ua"},
			wantValue:   "mandat.ua",
			wantDisplay: "mandat.ua",
			wantHref:    "https://www.facebook.com/mandat.ua",
		},
		{
			name:        "facebook bare host url",
			input:       ContactInput{Type: "facebook", Value: "facebook.com/mandat.ua"},
			wantValue:   "mandat.ua",
			wantDisplay: "mandat.ua",
			wantHref:    "https://www.facebook.com/mandat.ua",
		},
		{
			name:        "phone keeps formatting, href keeps digits",
			input:       ContactInput{Type: "phone", Value: "+380 44 123-45-67"},
			wantValue:   "+380 44 123-45-67",
			wantDisplay: "+380 44 123-45-67",
			wantHref:    "tel:+380441234567",
		},
		{
			name:        "phone with parentheses",
			input:       ContactInput{Type: "phone", Value: "(044) 123 45 67"},
			wantValue:   "(044) 123 45 67",
			wantDisplay: "(044) 123 45 67",
			wantHref:    "tel:0441234567",
		},
		{
			name:        "website keeps value, display is host",
			input:       ContactInput{Type: "website", Value: "https://mandat.ua/about"},
			wantValue:   "https://mandat.ua/about",
			wantDisplay: "mandat.ua",
			wantHref:    "https://mandat.ua/about",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)

			created, err := svc.Create(context.Background(), tt.input)

			require.NoError(t, err)
			require.NotNil(t, created)
			assert.Equal(t, tt.wantValue, created.Value, "the stored value must be normalized")
			assert.Equal(t, tt.wantDisplay, created.Display)
			assert.Equal(t, tt.wantHref, created.Href)
			require.Contains(t, repo.items, created.ID)
			assert.Equal(t, tt.wantValue, repo.items[created.ID].Value,
				"the normalized value must be persisted, not just returned")
		})
	}
}

func TestContact_Create_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		input ContactInput
	}{
		{"email without at", ContactInput{Type: "email", Value: "not-an-email"}},
		{"email with display name", ContactInput{Type: "email", Value: "Name <a@b.co>"}},
		{"email too long", ContactInput{Type: "email", Value: strings.Repeat("a", 250) + "@b.co"}},
		{"telegram too short", ContactInput{Type: "telegram", Value: "abc"}},
		{"telegram bad chars", ContactInput{Type: "telegram", Value: "bad-name"}},
		{"telegram foreign host", ContactInput{Type: "telegram", Value: "https://evil.com/mandat"}},
		{"telegram other type host", ContactInput{Type: "telegram", Value: "https://www.instagram.com/mandat"}},
		{"telegram javascript scheme", ContactInput{Type: "telegram", Value: "javascript:alert(1)"}},
		{"instagram foreign host", ContactInput{Type: "instagram", Value: "https://evil.com/mandat"}},
		{"instagram other type host", ContactInput{Type: "instagram", Value: "https://t.me/mandat"}},
		{"instagram spaces", ContactInput{Type: "instagram", Value: "bad name"}},
		{"instagram too long", ContactInput{Type: "instagram", Value: strings.Repeat("a", 31)}},
		{"threads foreign host", ContactInput{Type: "threads", Value: "https://evil.com/@mandat"}},
		{"threads other type host", ContactInput{Type: "threads", Value: "https://t.me/mandat"}},
		{"facebook too short", ContactInput{Type: "facebook", Value: "abc"}},
		{"facebook foreign host", ContactInput{Type: "facebook", Value: "https://evil.com/mandat.ua"}},
		{"facebook other type host", ContactInput{Type: "facebook", Value: "https://t.me/mandat"}},
		{"phone letters", ContactInput{Type: "phone", Value: "call +380441234567"}},
		{"phone too few digits", ContactInput{Type: "phone", Value: "123456"}},
		{"phone too many digits", ContactInput{Type: "phone", Value: "+1234567890123456"}},
		{"website http", ContactInput{Type: "website", Value: "http://mandat.ua"}},
		{"website ftp", ContactInput{Type: "website", Value: "ftp://x"}},
		{"website bare host", ContactInput{Type: "website", Value: "mandat.ua"}},
		{"website no host", ContactInput{Type: "website", Value: "https://"}},
		{"website too long", ContactInput{Type: "website", Value: "https://" + strings.Repeat("a", 200)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)

			created, err := svc.Create(context.Background(), tt.input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
			assert.Equal(t, "value", validationErr.Field)
			assert.Zero(t, repo.createCalls, "an invalid value must be rejected before any repository write")
			assert.Empty(t, repo.items)
		})
	}
}

func TestContact_Create_RejectsBlankValuePerType(t *testing.T) {
	tests := []struct {
		name  string
		input ContactInput
	}{
		{"email empty", ContactInput{Type: "email", Value: ""}},
		{"email whitespace only", ContactInput{Type: "email", Value: "   "}},
		{"telegram empty", ContactInput{Type: "telegram", Value: ""}},
		{"telegram whitespace only", ContactInput{Type: "telegram", Value: "   "}},
		{"instagram empty", ContactInput{Type: "instagram", Value: ""}},
		{"instagram whitespace only", ContactInput{Type: "instagram", Value: "   "}},
		{"threads empty", ContactInput{Type: "threads", Value: ""}},
		{"threads whitespace only", ContactInput{Type: "threads", Value: "   "}},
		{"facebook empty", ContactInput{Type: "facebook", Value: ""}},
		{"facebook whitespace only", ContactInput{Type: "facebook", Value: "   "}},
		{"phone empty", ContactInput{Type: "phone", Value: ""}},
		{"phone whitespace only", ContactInput{Type: "phone", Value: "   "}},
		{"website empty", ContactInput{Type: "website", Value: ""}},
		{"website whitespace only", ContactInput{Type: "website", Value: "   "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)

			created, err := svc.Create(context.Background(), tt.input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
			assert.Equal(t, "value", validationErr.Field)
			assert.Zero(t, repo.createCalls, "a blank value must be rejected before any repository write")
			assert.Empty(t, repo.items)
		})
	}
}

func TestContact_Create_RejectsUnknownType(t *testing.T) {
	for _, contactType := range []string{"skype", ""} {
		t.Run("type "+contactType, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)

			created, err := svc.Create(context.Background(), ContactInput{Type: contactType, Value: "x"})

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
			assert.Equal(t, "type", validationErr.Field)
			assert.Zero(t, repo.createCalls)
		})
	}
}

func TestContact_Create_TrimsLabel(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		wantLabel string
	}{
		{"trims spaces", "  Реклама  ", "Реклама"},
		{"boundary 40 runes ok", strings.Repeat("а", 40), strings.Repeat("а", 40)},
		{"empty stays empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)

			input := validContactInput()
			input.Label = tt.label

			created, err := svc.Create(context.Background(), input)

			require.NoError(t, err)
			require.NotNil(t, created)
			assert.Equal(t, tt.wantLabel, created.Label)
		})
	}
}

func TestContact_Create_RejectsLongLabel(t *testing.T) {
	repo := newFakeContactRepo()
	svc := newContactTestService(repo)

	input := validContactInput()
	input.Label = strings.Repeat("а", 41)

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	var validationErr *errs.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
	assert.Equal(t, "label", validationErr.Field)
	assert.Zero(t, repo.createCalls, "an invalid label must be rejected before any repository write")
}

func TestContact_List_ReturnsRepoOrderWithDisplayAndHref(t *testing.T) {
	repo := newFakeContactRepo()
	first := seedContact(repo, models.ContactEmail, "info@mandat.ua", "")
	second := seedContact(repo, models.ContactTelegram, "mandat", "")
	svc := newContactTestService(repo)

	items, err := svc.List(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, first.ID, items[0].ID)
	assert.Equal(t, second.ID, items[1].ID)
	assert.Equal(t, "info@mandat.ua", items[0].Display)
	assert.Equal(t, "mailto:info@mandat.ua", items[0].Href)
	assert.Equal(t, "@mandat", items[1].Display)
	assert.Equal(t, "https://t.me/mandat", items[1].Href)
}

func TestContact_Create_AppendsAndReturnsStored(t *testing.T) {
	repo := newFakeContactRepo()
	svc := newContactTestService(repo)

	first, err := svc.Create(context.Background(), validContactInput())
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.NotEqual(t, uuid.Nil, first.ID)
	assert.Equal(t, 1, first.Position)

	second, err := svc.Create(context.Background(), ContactInput{Type: "telegram", Value: "mandat"})
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2, second.Position)
	assert.Equal(t, 2, repo.createCalls)
}

func TestContact_Update_MissingNotFound(t *testing.T) {
	repo := newFakeContactRepo()
	svc := newContactTestService(repo)

	_, err := svc.Update(context.Background(), uuid.New(), validContactInput())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrContactNotFound), "expected ErrContactNotFound, got %v", err)
}

func TestContact_Update_SavesNormalizedAndReturnsRepoFields(t *testing.T) {
	repo := newFakeContactRepo()
	svc := newContactTestService(repo)
	existing := seedContact(repo, models.ContactEmail, "old@mandat.ua", "Стара")

	updated, err := svc.Update(context.Background(), existing.ID,
		ContactInput{Type: "telegram", Value: "@mandat", Label: "  Нова  "})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "mandat", updated.Value)
	assert.Equal(t, "@mandat", updated.Display)
	assert.Equal(t, "https://t.me/mandat", updated.Href)
	assert.Equal(t, "Нова", updated.Label)
	assert.Equal(t, existing.Position, updated.Position, "update must return the repo position")
	assert.Equal(t, existing.CreatedAt, updated.CreatedAt, "update must return the repo created_at")
	assert.Equal(t, "mandat", repo.items[existing.ID].Value,
		"the normalized value must be persisted, not just returned")
}

func TestContact_Update_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input ContactInput
		field string
	}{
		{"unknown type", ContactInput{Type: "skype", Value: "x"}, "type"},
		{"bad value", ContactInput{Type: "email", Value: "not-an-email"}, "value"},
		{"long label", ContactInput{Type: "email", Value: "a@b.co", Label: strings.Repeat("а", 41)}, "label"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)
			existing := seedContact(repo, models.ContactEmail, "old@mandat.ua", "")

			updated, err := svc.Update(context.Background(), existing.ID, tt.input)

			require.Error(t, err)
			assert.Nil(t, updated)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
			assert.Equal(t, tt.field, validationErr.Field)
			assert.Zero(t, repo.updateCalls, "an invalid input must be rejected before any repository write")
		})
	}
}

func TestContact_Delete_RemovesRow(t *testing.T) {
	repo := newFakeContactRepo()
	svc := newContactTestService(repo)
	existing := seedContact(repo, models.ContactEmail, "old@mandat.ua", "")

	err := svc.Delete(context.Background(), existing.ID)

	require.NoError(t, err)
	assert.NotContains(t, repo.items, existing.ID)
}

func TestContact_Delete_MissingNotFound(t *testing.T) {
	repo := newFakeContactRepo()
	svc := newContactTestService(repo)

	err := svc.Delete(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrContactNotFound), "expected ErrContactNotFound, got %v", err)
}

func TestContact_Move_DelegatesDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantUp    bool
	}{
		{"up", "up", true},
		{"down", "down", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)
			existing := seedContact(repo, models.ContactEmail, "a@b.co", "")

			err := svc.Move(context.Background(), existing.ID, tt.direction)

			require.NoError(t, err)
			assert.Equal(t, 1, repo.moveCalls)
			assert.Equal(t, existing.ID, repo.lastMoveID)
			assert.Equal(t, tt.wantUp, repo.lastMoveUp)
		})
	}
}

func TestContact_Move_InvalidDirectionFailsBeforeRepo(t *testing.T) {
	for _, direction := range []string{"left", "", "UP", "sideways"} {
		t.Run("direction "+direction, func(t *testing.T) {
			repo := newFakeContactRepo()
			svc := newContactTestService(repo)
			existing := seedContact(repo, models.ContactEmail, "a@b.co", "")

			err := svc.Move(context.Background(), existing.ID, direction)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
			assert.Equal(t, "direction", validationErr.Field)
			assert.Zero(t, repo.moveCalls, "an invalid direction must not reach the repository")
		})
	}
}

func TestContact_Move_MissingNotFound(t *testing.T) {
	repo := newFakeContactRepo()
	svc := newContactTestService(repo)

	err := svc.Move(context.Background(), uuid.New(), "up")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrContactNotFound), "expected ErrContactNotFound, got %v", err)
}

func TestContact_RepoErrorsAreWrapped(t *testing.T) {
	cause := errors.New("db is down")

	t.Run("list", func(t *testing.T) {
		repo := newFakeContactRepo()
		repo.listErr = cause
		svc := newContactTestService(repo)

		_, err := svc.List(context.Background())

		require.Error(t, err)
		assert.True(t, errors.Is(err, cause), "expected the repository cause, got %v", err)
	})

	t.Run("create", func(t *testing.T) {
		repo := newFakeContactRepo()
		repo.createErr = cause
		svc := newContactTestService(repo)

		_, err := svc.Create(context.Background(), validContactInput())

		require.Error(t, err)
		assert.True(t, errors.Is(err, cause), "expected the repository cause, got %v", err)
	})

	t.Run("update", func(t *testing.T) {
		repo := newFakeContactRepo()
		svc := newContactTestService(repo)
		existing := seedContact(repo, models.ContactEmail, "old@mandat.ua", "")
		repo.updateErr = cause

		_, err := svc.Update(context.Background(), existing.ID, validContactInput())

		require.Error(t, err)
		assert.True(t, errors.Is(err, cause), "expected the repository cause, got %v", err)
	})

	t.Run("delete", func(t *testing.T) {
		repo := newFakeContactRepo()
		svc := newContactTestService(repo)
		existing := seedContact(repo, models.ContactEmail, "old@mandat.ua", "")
		repo.deleteErr = cause

		err := svc.Delete(context.Background(), existing.ID)

		require.Error(t, err)
		assert.True(t, errors.Is(err, cause), "expected the repository cause, got %v", err)
	})

	t.Run("move", func(t *testing.T) {
		repo := newFakeContactRepo()
		svc := newContactTestService(repo)
		existing := seedContact(repo, models.ContactEmail, "old@mandat.ua", "")
		repo.moveErr = cause

		err := svc.Move(context.Background(), existing.ID, "up")

		require.Error(t, err)
		assert.True(t, errors.Is(err, cause), "expected the repository cause, got %v", err)
	})
}
