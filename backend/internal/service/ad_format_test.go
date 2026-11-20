package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAdFormatRepo is an in-memory AdFormatRepository for service unit tests.
type fakeAdFormatRepo struct {
	items       map[uuid.UUID]*models.AdFormat
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
	deleteIDs   []uuid.UUID
	lastMoveID  uuid.UUID
	lastMoveUp  bool
}

func newFakeAdFormatRepo() *fakeAdFormatRepo {
	return &fakeAdFormatRepo{items: map[uuid.UUID]*models.AdFormat{}}
}

// List returns the seeded cards in insertion order.
func (r *fakeAdFormatRepo) List(_ context.Context) ([]*models.AdFormat, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make([]*models.AdFormat, 0, len(r.order))
	for _, id := range r.order {
		copied := *r.items[id]
		out = append(out, &copied)
	}

	return out, nil
}

// GetByID returns one card or errs.ErrAdFormatNotFound.
func (r *fakeAdFormatRepo) GetByID(_ context.Context, id uuid.UUID) (*models.AdFormat, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, errs.ErrAdFormatNotFound
	}
	copied := *item

	return &copied, nil
}

// Create appends the card and assigns its ID and position.
func (r *fakeAdFormatRepo) Create(_ context.Context, format *models.AdFormat) error {
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	if format.ID == uuid.Nil {
		format.ID = uuid.New()
	}
	format.Position = len(r.items) + 1
	stored := *format
	r.items[format.ID] = &stored
	r.order = append(r.order, format.ID)

	return nil
}

// Update replaces a stored card.
func (r *fakeAdFormatRepo) Update(_ context.Context, format *models.AdFormat) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	if _, ok := r.items[format.ID]; !ok {
		return errs.ErrAdFormatNotFound
	}
	stored := *format
	r.items[format.ID] = &stored

	return nil
}

// Delete removes a stored card.
func (r *fakeAdFormatRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.deleteCalls++
	r.deleteIDs = append(r.deleteIDs, id)
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.items[id]; !ok {
		return errs.ErrAdFormatNotFound
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
func (r *fakeAdFormatRepo) Move(_ context.Context, id uuid.UUID, up bool) error {
	r.moveCalls++
	r.lastMoveID = id
	r.lastMoveUp = up
	if r.moveErr != nil {
		return r.moveErr
	}
	if _, ok := r.items[id]; !ok {
		return errs.ErrAdFormatNotFound
	}

	return nil
}

// seedAdFormat inserts a card directly, bypassing service validation.
func seedAdFormat(r *fakeAdFormatRepo, title, imageURL string) *models.AdFormat {
	format := &models.AdFormat{
		ID:          uuid.New(),
		Title:       title,
		Description: "Опис",
		ImageURL:    imageURL,
		Position:    len(r.items) + 1,
	}
	stored := *format
	r.items[format.ID] = &stored
	r.order = append(r.order, format.ID)

	return format
}

func newAdFormatTestService(repo AdFormatRepository, storage ImageStorage) *AdFormatService {
	return NewAdFormatService(repo, slog.New(&captureHandler{}), storage, coverTestPublicBaseURL)
}

func validAdFormatInput() AdFormatInput {
	return AdFormatInput{
		Title:       "Просування в соцмережах",
		Description: "Короткий опис формату",
	}
}

func validAdFormatImage(filename string, body []byte) *CoverUpload {
	return &CoverUpload{
		Body:        bytes.NewReader(body),
		Filename:    filename,
		ContentType: "image/png",
		Size:        int64(len(body)),
	}
}

// assertAdFormatKey checks ad-formats/<id>-<8 lowercase hex>.<ext>.
func assertAdFormatKey(t *testing.T, key string, id uuid.UUID, ext string, msgAndArgs ...any) {
	t.Helper()

	pattern := `^ad-formats/` + regexp.QuoteMeta(id.String()) + `-[0-9a-f]{8}\.` + regexp.QuoteMeta(ext) + `$`
	assert.Regexp(t, regexp.MustCompile(pattern), key, msgAndArgs...)
}

func TestAdFormat_List_ReturnsRepoItemsInOrder(t *testing.T) {
	repo := newFakeAdFormatRepo()
	first := seedAdFormat(repo, "Перший", "")
	second := seedAdFormat(repo, "Другий", "")
	svc := newAdFormatTestService(repo, &fakeImageStorage{})

	items, err := svc.List(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, first.ID, items[0].ID)
	assert.Equal(t, second.ID, items[1].ID)
}

func TestAdFormat_List_RepoErrorPropagates(t *testing.T) {
	repo := newFakeAdFormatRepo()
	cause := errors.New("db is down")
	repo.listErr = cause
	svc := newAdFormatTestService(repo, &fakeImageStorage{})

	_, err := svc.List(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, cause), "expected the repository cause, got %v", err)
}

func TestAdFormat_Create_TrimsTitleAndDescription(t *testing.T) {
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, &fakeImageStorage{})

	input := validAdFormatInput()
	input.Title = "  Просування в соцмережах  "
	input.Description = "  Короткий опис   "

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "Просування в соцмережах", created.Title)
	assert.Equal(t, "Короткий опис", created.Description)
}

func TestAdFormat_Create_Validation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*AdFormatInput)
		field  string
	}{
		{"empty title", func(in *AdFormatInput) { in.Title = "" }, "title"},
		{"whitespace title", func(in *AdFormatInput) { in.Title = "   " }, "title"},
		{
			name:   "title too long",
			mutate: func(in *AdFormatInput) { in.Title = strings.Repeat("а", AdFormatTitleMaxLen+1) },
			field:  "title",
		},
		{
			name:   "description too long",
			mutate: func(in *AdFormatInput) { in.Description = strings.Repeat("б", AdFormatDescriptionMaxLen+1) },
			field:  "description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeAdFormatRepo()
			storage := &fakeImageStorage{}
			svc := newAdFormatTestService(repo, storage)
			input := validAdFormatInput()
			tt.mutate(&input)

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
			assert.Equal(t, tt.field, validationErr.Field)
			assert.Zero(t, repo.createCalls, "invalid input must be rejected before any repository write")
			assert.Empty(t, storage.uploadKeys, "invalid input must never be uploaded")
		})
	}
}

func TestAdFormat_Create_BoundaryLengthsOk(t *testing.T) {
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, &fakeImageStorage{})

	input := validAdFormatInput()
	input.Title = strings.Repeat("а", AdFormatTitleMaxLen)
	input.Description = strings.Repeat("б", AdFormatDescriptionMaxLen)

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Len(t, []rune(created.Title), AdFormatTitleMaxLen,
		"Cyrillic runes count as one, so 80 а-symbols must fit the 80-rune limit")
	assert.Len(t, []rune(created.Description), AdFormatDescriptionMaxLen)
}

func TestAdFormat_Create_WithoutImageCallsRepo(t *testing.T) {
	repo := newFakeAdFormatRepo()
	storage := &fakeImageStorage{}
	svc := newAdFormatTestService(repo, storage)

	created, err := svc.Create(context.Background(), validAdFormatInput())

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "", created.ImageURL)
	assert.Equal(t, 1, repo.createCalls)
	assert.Zero(t, repo.updateCalls, "creating without an image must not update")
	assert.Empty(t, storage.uploadKeys)
	require.Contains(t, repo.items, created.ID)
}

func TestAdFormat_Create_WithImageUploadsPNGKey(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/ad-formats/uploaded.png"}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)

	input := validAdFormatInput()
	input.Image = validAdFormatImage("a.png", fetchTestPNG)

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Len(t, storage.uploadKeys, 1)
	assertAdFormatKey(t, storage.uploadKeys[0], created.ID, "png",
		"the image key must be ad-formats/<id>-<8 hex>.png")
	assert.Equal(t, "image/png", storage.uploadTypes[0])
	assert.Equal(t, storage.uploadURL, created.ImageURL, "the returned card must carry the saved URL")
	assert.Equal(t, storage.uploadURL, repo.items[created.ID].ImageURL,
		"the uploaded image URL must be persisted via repo.Update")
}

func TestAdFormat_Create_ImageMismatchFailsBeforeWrite(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		body     []byte
	}{
		{"png bytes named jpg", "x.jpg", fetchTestPNG},
		{"jpeg bytes named png", "x.png", fetchTestJPEG},
		{"plain text named png", "x.png", []byte("hello")},
		{"unsupported extension", "evil.exe", fetchTestPNG},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeImageStorage{}
			repo := newFakeAdFormatRepo()
			svc := newAdFormatTestService(repo, storage)

			input := validAdFormatInput()
			input.Image = validAdFormatImage(tt.filename, tt.body)

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Zero(t, repo.createCalls,
				"a mismatched image must be rejected before any repository write")
			assert.Empty(t, storage.uploadKeys, "a mismatched image must never be uploaded")
			assert.Empty(t, repo.items)
		})
	}
}

func TestAdFormat_Create_OversizedImageFailsBeforeWrite(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)

	body := make([]byte, MaxCoverBytes+1)
	copy(body, fetchTestPNG)
	input := validAdFormatInput()
	input.Image = validAdFormatImage("big.png", body)

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrPayloadTooLarge),
		"an image larger than MaxCoverBytes must surface ErrPayloadTooLarge, got %v", err)
	assert.Zero(t, repo.createCalls, "an oversized image must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys, "an oversized image must never be uploaded")
}

func TestAdFormat_Create_UploadFailureDeletesRow(t *testing.T) {
	storage := &fakeImageStorage{uploadErr: errors.New("storage is down")}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)

	input := validAdFormatInput()
	input.Image = validAdFormatImage("a.png", fetchTestPNG)

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed upload must fail the create after the row was inserted")
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a failed upload must surface ErrServiceUnavailable, got %v", err)
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.items, "no orphan row may survive a failed upload")
	require.Len(t, storage.uploadKeys, 1)
}

func TestAdFormat_Create_URLSaveFailureDeletesRowAndObject(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/ad-formats/saved.png"}
	repo := newFakeAdFormatRepo()
	repo.updateErr = errors.New("database is down")
	svc := newAdFormatTestService(repo, storage)

	input := validAdFormatInput()
	input.Image = validAdFormatImage("a.png", fetchTestPNG)

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed URL save must fail the create")
	assert.Nil(t, created)
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.items)
	require.Len(t, storage.uploadKeys, 1)
	assert.Equal(t, storage.uploadKeys, storage.deleteKeys,
		"the uploaded object must be deleted when saving its URL fails")
}

func TestAdFormat_Create_NilStorageWithImageFailsUnavailable(t *testing.T) {
	repo := newFakeAdFormatRepo()
	svc := NewAdFormatService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL)

	input := validAdFormatInput()
	input.Image = validAdFormatImage("a.png", fetchTestPNG)

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"an image with nil storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.createCalls, "no row may be written when image storage is not configured")
	assert.Empty(t, repo.items)
}

func TestAdFormat_Update_UnknownIDNotFound(t *testing.T) {
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, &fakeImageStorage{})

	_, err := svc.Update(context.Background(), uuid.New(), validAdFormatInput())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound), "expected ErrAdFormatNotFound, got %v", err)
}

func TestAdFormat_Update_TextOnlyKeepsImageAndPosition(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	existing := seedAdFormat(repo, "Стара назва", coverTestPublicBaseURL+"/ad-formats/old-12345678.png")

	input := validAdFormatInput()
	input.Title = "Нова назва"
	input.Description = "Новий опис"

	updated, err := svc.Update(context.Background(), existing.ID, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Нова назва", updated.Title)
	assert.Equal(t, existing.ImageURL, updated.ImageURL,
		"a text-only update must keep the existing image URL")
	assert.Equal(t, existing.Position, updated.Position)
	assert.Empty(t, storage.uploadKeys)
	assert.Empty(t, storage.deleteKeys)
}

func TestAdFormat_Update_NewImageReplacesOwnedKey(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/ad-formats/new-abcdef01.png"}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	existing := seedAdFormat(repo, "Назва", coverTestPublicBaseURL+"/ad-formats/old-12345678.png")

	input := validAdFormatInput()
	input.Image = validAdFormatImage("b.png", fetchTestPNG)

	updated, err := svc.Update(context.Background(), existing.ID, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, storage.uploadKeys, 1)
	assertAdFormatKey(t, storage.uploadKeys[0], existing.ID, "png")
	assert.Equal(t, storage.uploadURL, updated.ImageURL)
	assert.Equal(t, []string{"ad-formats/old-12345678.png"}, storage.deleteKeys,
		"the replaced owned image must be removed after the save")
}

func TestAdFormat_Update_ForeignURLsNeverDeleted(t *testing.T) {
	tests := []struct {
		name     string
		imageURL string
	}{
		{"other host", "https://other.example/pic.png"},
		{"same host but covers prefix", coverTestPublicBaseURL + "/covers/x.png"},
		{"same host but static prefix", coverTestPublicBaseURL + "/static/logo.png"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/ad-formats/new-abcdef01.png"}
			repo := newFakeAdFormatRepo()
			svc := newAdFormatTestService(repo, storage)
			existing := seedAdFormat(repo, "Назва", tt.imageURL)

			input := validAdFormatInput()
			input.Image = validAdFormatImage("b.png", fetchTestPNG)

			updated, err := svc.Update(context.Background(), existing.ID, input)

			require.NoError(t, err)
			require.NotNil(t, updated)
			require.Len(t, storage.uploadKeys, 1)
			assert.Empty(t, storage.deleteKeys,
				"a foreign or non-ad-formats URL must never be deleted")
		})
	}
}

func TestAdFormat_Update_RepoFailureDeletesNewKeyKeepsOld(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/ad-formats/new-abcdef01.png"}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	oldURL := coverTestPublicBaseURL + "/ad-formats/old-12345678.png"
	existing := seedAdFormat(repo, "Назва", oldURL)
	repo.updateErr = errors.New("database is down")

	input := validAdFormatInput()
	input.Image = validAdFormatImage("b.png", fetchTestPNG)

	updated, err := svc.Update(context.Background(), existing.ID, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	require.Len(t, storage.uploadKeys, 1)
	require.Len(t, storage.deleteKeys, 1)
	assert.Equal(t, storage.uploadKeys[0], storage.deleteKeys[0],
		"only the freshly uploaded object may be cleaned up")
	assert.Equal(t, oldURL, repo.items[existing.ID].ImageURL,
		"the stored card must keep its previous image after a failed update")
}

func TestAdFormat_Update_RemoveImageClearsOwnedKey(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	existing := seedAdFormat(repo, "Назва", coverTestPublicBaseURL+"/ad-formats/old-12345678.png")

	input := validAdFormatInput()
	input.RemoveImage = true

	updated, err := svc.Update(context.Background(), existing.ID, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "", updated.ImageURL, "remove_image must clear the image URL")
	assert.Equal(t, "", repo.items[existing.ID].ImageURL)
	assert.Equal(t, []string{"ad-formats/old-12345678.png"}, storage.deleteKeys,
		"the old owned image must be deleted after the save")
	assert.Empty(t, storage.uploadKeys)
}

func TestAdFormat_Update_RemoveImageWithNewImagePrefersUpload(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/ad-formats/new-abcdef01.png"}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	existing := seedAdFormat(repo, "Назва", coverTestPublicBaseURL+"/ad-formats/old-12345678.png")

	input := validAdFormatInput()
	input.Image = validAdFormatImage("b.png", fetchTestPNG)
	input.RemoveImage = true

	updated, err := svc.Update(context.Background(), existing.ID, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, storage.uploadURL, updated.ImageURL,
		"a new image wins over remove_image")
	require.Len(t, storage.uploadKeys, 1)
	assert.Equal(t, []string{"ad-formats/old-12345678.png"}, storage.deleteKeys)
}

func TestAdFormat_Delete_UnknownIDNotFound(t *testing.T) {
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, &fakeImageStorage{})

	err := svc.Delete(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound), "expected ErrAdFormatNotFound, got %v", err)
}

func TestAdFormat_Delete_RemovesRowAndOwnedKey(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	existing := seedAdFormat(repo, "Назва", coverTestPublicBaseURL+"/ad-formats/old-12345678.png")

	err := svc.Delete(context.Background(), existing.ID)

	require.NoError(t, err)
	assert.NotContains(t, repo.items, existing.ID)
	assert.Equal(t, []string{"ad-formats/old-12345678.png"}, storage.deleteKeys)
}

func TestAdFormat_Delete_StorageFailureNotReturned(t *testing.T) {
	storage := &fakeImageStorage{deleteErr: errors.New("storage is down")}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	existing := seedAdFormat(repo, "Назва", coverTestPublicBaseURL+"/ad-formats/old-12345678.png")

	err := svc.Delete(context.Background(), existing.ID)

	require.NoError(t, err, "a cleanup failure must not fail the deletion")
	assert.NotContains(t, repo.items, existing.ID)
	assert.Equal(t, []string{"ad-formats/old-12345678.png"}, storage.deleteKeys)
}

func TestAdFormat_Delete_ForeignURLSkipped(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := newFakeAdFormatRepo()
	svc := newAdFormatTestService(repo, storage)
	existing := seedAdFormat(repo, "Назва", "https://other.example/pic.png")

	err := svc.Delete(context.Background(), existing.ID)

	require.NoError(t, err)
	assert.NotContains(t, repo.items, existing.ID)
	assert.Empty(t, storage.deleteKeys,
		"an image outside our bucket must not be deleted")
}

func TestAdFormat_Move_DelegatesDirection(t *testing.T) {
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
			repo := newFakeAdFormatRepo()
			svc := newAdFormatTestService(repo, &fakeImageStorage{})
			existing := seedAdFormat(repo, "Назва", "")

			err := svc.Move(context.Background(), existing.ID, tt.direction)

			require.NoError(t, err)
			assert.Equal(t, 1, repo.moveCalls)
			assert.Equal(t, existing.ID, repo.lastMoveID)
			assert.Equal(t, tt.wantUp, repo.lastMoveUp)
		})
	}
}

func TestAdFormat_Move_InvalidDirectionFailsBeforeRepo(t *testing.T) {
	for _, direction := range []string{"left", "", "UP", "sideways"} {
		t.Run("direction "+direction, func(t *testing.T) {
			repo := newFakeAdFormatRepo()
			svc := newAdFormatTestService(repo, &fakeImageStorage{})
			existing := seedAdFormat(repo, "Назва", "")

			err := svc.Move(context.Background(), existing.ID, direction)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Zero(t, repo.moveCalls, "an invalid direction must not reach the repository")
		})
	}
}

func TestAdFormat_Move_RepoNotFoundPropagates(t *testing.T) {
	repo := newFakeAdFormatRepo()
	repo.moveErr = errs.ErrAdFormatNotFound
	svc := newAdFormatTestService(repo, &fakeImageStorage{})

	err := svc.Move(context.Background(), uuid.New(), "up")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound), "expected ErrAdFormatNotFound, got %v", err)
}
