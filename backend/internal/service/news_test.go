package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Shared fakes and helpers: stub repo, storage/media/fetcher fakes, log capture, input builders ---
// Helpers and fakes shared by the tests below; no behavior is asserted here.

// captureHandler records slog records for assertions.
type captureHandler struct {
	records []slog.Record
	attrs   []slog.Attr
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, attrs...)
	return h
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) hasFC(fc string) bool {
	for _, r := range h.records {
		if r.Message == fc {
			return true
		}
		found := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Value.String() == fc {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
		for _, a := range h.attrs {
			if a.Value.String() == fc {
				return true
			}
		}
	}
	return false
}

func newTestService() (*NewsService, *testsupport.StubRepo) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	logger := slog.New(handler)
	return NewNewsService(repo, logger, nil, "", nil), repo
}

func validInput() NewsInput {
	return NewsInput{
		Category: models.CategoryNews,
		Title:    "Уряд схвалив новий бюджет",
		Text:     "Повний текст статті",
	}
}

func publishedFeaturedItems(repo *testsupport.StubRepo) []*models.News {
	var items []*models.News
	for _, item := range repo.Items {
		if item.Status == models.StatusPublished && item.Featured {
			items = append(items, item)
		}
	}

	return items
}

func publishedMainEventItems(repo *testsupport.StubRepo) []*models.News {
	var items []*models.News
	for _, item := range repo.Items {
		if item.Status == models.StatusPublished && item.MainEvent {
			items = append(items, item)
		}
	}

	return items
}

// coverTestWEBP is a minimal valid WebP image used for cover uploads.
var coverTestWEBP = []byte{
	'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00,
	'W', 'E', 'B', 'P', 'V', 'P', '8', ' ',
}

// assertCoverKey checks that a storage key is unique per upload:
// covers/<slug>-<8 lowercase hex>.<ext>.
func assertCoverKey(t *testing.T, key, slug, ext string, msgAndArgs ...any) {
	t.Helper()

	pattern := `^covers/` + regexp.QuoteMeta(slug) + `-[0-9a-f]{8}\.` + regexp.QuoteMeta(ext) + `$`
	assert.Regexp(t, regexp.MustCompile(pattern), key, msgAndArgs...)
}

// newCoverLoggingService builds a cover-enabled NewsService that records logs
// into handler for error-level assertions.
func newCoverLoggingService(
	repo NewsRepository,
	storage ImageStorage,
	handler *captureHandler,
) *NewsService {
	return NewNewsService(repo, slog.New(handler), storage, coverTestPublicBaseURL, nil)
}

// seedCoverArticle stores a draft with the given slug and image URL directly.
func seedCoverArticle(repo *testsupport.StubRepo, slug, imageURL string) *models.News {
	now := time.Now().UTC()
	id := uuid.New()
	news := &models.News{
		ID:        id,
		Slug:      slug,
		Category:  models.CategoryNews,
		Title:     slug,
		Text:      "Повний текст",
		ImageURL:  imageURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.Items[id] = news

	return news
}

// coverValidationService builds a cover-enabled service wired to handler so
// validation tests can assert that no error-level record is emitted.
func coverValidationService(
	repo *testsupport.StubRepo,
	storage *fakeImageStorage,
	handler *captureHandler,
) *NewsService {
	return newCoverLoggingService(repo, storage, handler)
}

// seedListNews stores n drafts directly and returns them in slug order.
func seedListNews(repo *testsupport.StubRepo, n int) {
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		id := uuid.New()
		repo.Items[id] = &models.News{
			ID:        id,
			Slug:      string(rune('a'+i)) + "-article",
			Category:  models.CategoryNews,
			Title:     string(rune('a'+i)) + "-article",
			Text:      "Повний текст",
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
}

// countErrorRecords counts the error-level records a service call emitted.
func countErrorRecords(h *captureHandler) int {
	count := 0
	for _, record := range h.records {
		if record.Level >= slog.LevelError {
			count++
		}
	}

	return count
}

// loggingService builds a NewsService wired to handler for log assertions.
func loggingService(repo *testsupport.StubRepo, handler *captureHandler) *NewsService {
	return NewNewsService(repo, slog.New(handler), nil, "", nil)
}

// imageTestGIF is a minimal GIF89a header sniffed as image/gif.
var imageTestGIF = []byte{'G', 'I', 'F', '8', '9', 'a', 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00}

// newImageTestService builds a NewsService with media, image and fetch storage enabled.
func newImageTestService(
	repo NewsRepository,
	media MediaStorage,
	storage ImageStorage,
	fetcher ImageFetcher,
) *NewsService {
	return NewNewsService(repo, slog.New(&captureHandler{}), storage, coverTestPublicBaseURL, fetcher, WithMediaStorage(media))
}

// imageTestPendingKey builds a valid pending image key with 32 lowercase hex digits.
func imageTestPendingKey(ext string) string {
	return "pending/images/0123456789abcdef0123456789abcdef." + ext
}

// imageTestElevenKeys builds MaxImages+1 distinct valid-shaped pending keys.
func imageTestElevenKeys() []string {
	keys := make([]string, 0, MaxImages+1)
	for i := 0; i < MaxImages+1; i++ {
		keys = append(keys, fmt.Sprintf("pending/images/%032x.jpg", i))
	}

	return keys
}

// assertImageDstKey checks images/<slug>-<8 lowercase hex>.<ext>.
func assertImageDstKey(t *testing.T, key, slug, ext string, msgAndArgs ...any) {
	t.Helper()

	pattern := `^images/` + regexp.QuoteMeta(slug) + `-[0-9a-f]{8}\.` + regexp.QuoteMeta(ext) + `$`
	assert.Regexp(t, regexp.MustCompile(pattern), key, msgAndArgs...)
}

// seedImageArticle stores a draft with the given slug and images directly.
func seedImageArticle(repo *testsupport.StubRepo, slug string, images []string) *models.News {
	now := time.Now().UTC()
	id := uuid.New()
	news := &models.News{
		ID:        id,
		Slug:      slug,
		Category:  models.CategoryNews,
		Title:     slug,
		Text:      "Повний текст",
		ImageURLs: images,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.Items[id] = news

	return news
}

// coverTestPublicBaseURL is the bucket base URL the service treats as its own.
const coverTestPublicBaseURL = "https://cdn.example.com"

// fakeImageStorage records upload and delete calls.
type fakeImageStorage struct {
	uploadURL   string
	uploadErr   error
	deleteErr   error
	uploadKeys  []string
	uploadTypes []string
	uploadBody  []byte
	deleteKeys  []string
}

// Upload records the object key, content type and body, then returns uploadURL.
func (f *fakeImageStorage) Upload(
	_ context.Context,
	key string,
	body io.Reader,
	contentType string,
) (string, error) {
	f.uploadKeys = append(f.uploadKeys, key)
	f.uploadTypes = append(f.uploadTypes, contentType)
	if body != nil {
		f.uploadBody, _ = io.ReadAll(body)
	}
	if f.uploadErr != nil {
		return "", f.uploadErr
	}

	return f.uploadURL, nil
}

// Delete records the object key and returns deleteErr.
func (f *fakeImageStorage) Delete(_ context.Context, key string) error {
	f.deleteKeys = append(f.deleteKeys, key)

	return f.deleteErr
}

// deleteCountingRepo records the IDs created and deleted through it.
type deleteCountingRepo struct {
	*testsupport.StubRepo
	createIDs   []uuid.UUID
	deleteCalls int
	deleteIDs   []uuid.UUID
}

// Create delegates to the stub and records the ID of a stored article.
func (r *deleteCountingRepo) Create(ctx context.Context, news *models.News) error {
	if err := r.StubRepo.Create(ctx, news); err != nil {
		return err
	}
	r.createIDs = append(r.createIDs, news.ID)

	return nil
}

// Delete counts the call and delegates to the stub.
func (r *deleteCountingRepo) Delete(ctx context.Context, id uuid.UUID) error {
	r.deleteCalls++
	r.deleteIDs = append(r.deleteIDs, id)

	return r.StubRepo.Delete(ctx, id)
}

// updateCountingRepo records how many times the article was updated.
type updateCountingRepo struct {
	*testsupport.StubRepo
	updateCalls int
}

// Update counts the call and delegates to the stub.
func (r *updateCountingRepo) Update(ctx context.Context, news *models.News) error {
	r.updateCalls++

	return r.StubRepo.Update(ctx, news)
}

// newCoverTestService builds a NewsService with cover storage enabled.
func newCoverTestService(repo NewsRepository, storage ImageStorage) *NewsService {
	return newCoverTestServiceWithFetcher(repo, storage, nil)
}

// newCoverTestServiceWithFetcher builds a cover-enabled NewsService that also
// fetches pasted image URLs into storage.
func newCoverTestServiceWithFetcher(
	repo NewsRepository,
	storage ImageStorage,
	fetcher ImageFetcher,
) *NewsService {
	handler := &captureHandler{}

	return NewNewsService(repo, slog.New(handler), storage, coverTestPublicBaseURL, fetcher)
}

// fetched image signatures used by the fetcher-backed service tests.
var (
	fetchTestJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}
	fetchTestPNG  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	fetchTestTIFF = []byte{'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00}
)

// fakeFetcher records fetched URLs and returns scripted bytes, MIME and error.
type fakeFetcher struct {
	data  []byte
	mime  string
	err   error
	calls []string
}

// fakeFetcher must satisfy the service-owned fetcher contract.
var _ ImageFetcher = (*fakeFetcher)(nil)

// Fetch records the URL and returns the scripted result.
func (f *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, string, error) {
	f.calls = append(f.calls, url)
	if f.err != nil {
		return nil, "", f.err
	}

	return f.data, f.mime, nil
}

// seedSlugArticle stores an article with an explicit slug, bypassing the
// service slug generation so collision layouts can be staged exactly.
func seedSlugArticle(repo *testsupport.StubRepo, slug, title string) *models.News {
	now := time.Now().UTC()
	id := uuid.New()
	news := &models.News{
		ID:        id,
		Slug:      slug,
		Category:  models.CategoryNews,
		Title:     title,
		Text:      "Повний текст",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.Items[id] = news

	return news
}

// fakeMediaStorage records MediaStorage calls and returns scripted results.
type fakeMediaStorage struct {
	presignURL  string
	presignErr  error
	presignKeys []string
	presignType []string
	presignSize []int64
	presignTTL  []time.Duration

	inspectSize int64
	inspectHead []byte
	inspectErr  error
	inspectKeys []string
	inspectN    []int

	copyURL string
	copyErr error
	copySrc []string
	copyDst []string

	uploadURL   string
	uploadErr   error
	uploadKeys  []string
	uploadTypes []string
	uploadBody  []byte

	deleteKeys []string
	deleteErr  error
}

// fakeMediaStorage must satisfy the service-owned video contract.
var _ MediaStorage = (*fakeMediaStorage)(nil)

// PresignPut records the key, content type, size and TTL, then returns presignURL.
func (f *fakeMediaStorage) PresignPut(
	_ context.Context,
	key string,
	contentType string,
	size int64,
	ttl time.Duration,
) (string, error) {
	f.presignKeys = append(f.presignKeys, key)
	f.presignType = append(f.presignType, contentType)
	f.presignSize = append(f.presignSize, size)
	f.presignTTL = append(f.presignTTL, ttl)
	if f.presignErr != nil {
		return "", f.presignErr
	}

	return f.presignURL, nil
}

// Inspect records the key and n, then returns the scripted size and head bytes.
func (f *fakeMediaStorage) Inspect(_ context.Context, key string, n int) (int64, []byte, error) {
	f.inspectKeys = append(f.inspectKeys, key)
	f.inspectN = append(f.inspectN, n)
	if f.inspectErr != nil {
		return 0, nil, f.inspectErr
	}

	return f.inspectSize, f.inspectHead, nil
}

// Copy records src/dst and returns copyURL.
func (f *fakeMediaStorage) Copy(_ context.Context, srcKey string, dstKey string) (string, error) {
	f.copySrc = append(f.copySrc, srcKey)
	f.copyDst = append(f.copyDst, dstKey)
	if f.copyErr != nil {
		return "", f.copyErr
	}

	return f.copyURL, nil
}

// Upload records the object key, content type and body, then returns uploadURL.
func (f *fakeMediaStorage) Upload(
	_ context.Context,
	key string,
	body io.Reader,
	contentType string,
) (string, error) {
	f.uploadKeys = append(f.uploadKeys, key)
	f.uploadTypes = append(f.uploadTypes, contentType)
	if body != nil {
		f.uploadBody, _ = io.ReadAll(body)
	}
	if f.uploadErr != nil {
		return "", f.uploadErr
	}

	return f.uploadURL, nil
}

// Delete records the key and returns deleteErr.
func (f *fakeMediaStorage) Delete(_ context.Context, key string) error {
	f.deleteKeys = append(f.deleteKeys, key)

	return f.deleteErr
}

// newVideoTestService builds a NewsService with media storage enabled.
func newVideoTestService(repo NewsRepository, media MediaStorage) *NewsService {
	return NewNewsService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil, WithMediaStorage(media))
}

// videoTestMP4Head is a minimal ftyp header: size + "ftyp" at offset 4.
func videoTestMP4Head() []byte {
	head := make([]byte, 32)
	head[4] = 'f'
	head[5] = 't'
	head[6] = 'y'
	head[7] = 'p'
	head[8] = 'i'
	head[9] = 's'
	head[10] = 'o'
	head[11] = 'm'

	return head
}

// videoTestWebMHead is the EBML magic 1A 45 DF A3.
func videoTestWebMHead() []byte {
	return []byte{0x1A, 0x45, 0xDF, 0xA3, 0x93, 0x42, 0x82, 0x88}
}

// videoTestPendingKey builds a valid pending key with 32 lowercase hex digits.
func videoTestPendingKey(ext string) string {
	return "pending/videos/0123456789abcdef0123456789abcdef." + ext
}

// assertVideoDstKey checks videos/<slug>-<8 lowercase hex>.<ext>.
func assertVideoDstKey(t *testing.T, key, slug, ext string, msgAndArgs ...any) {
	t.Helper()

	pattern := `^videos/` + regexp.QuoteMeta(slug) + `-[0-9a-f]{8}\.` + regexp.QuoteMeta(ext) + `$`
	assert.Regexp(t, regexp.MustCompile(pattern), key, msgAndArgs...)
}

// seedVideoArticle stores a draft with the given slug and video directly.
func seedVideoArticle(repo *testsupport.StubRepo, slug string, video *models.Video) *models.News {
	now := time.Now().UTC()
	id := uuid.New()
	news := &models.News{
		ID:        id,
		Slug:      slug,
		Category:  models.CategoryNews,
		Title:     slug,
		Text:      "Повний текст",
		Video:     video,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.Items[id] = news

	return news
}

// seededID finds the article ID by slug in the stub repo.
func seededID(repo *testsupport.StubRepo, slug string) uuid.UUID {
	for id, item := range repo.Items {
		if item.Slug == slug {
			return id
		}
	}

	return uuid.Nil
}

// --- Create/Get/List/Update/Delete: core article rules and validation ---
// Covers creation, reads, listing, updates and deletes, including validation.

func TestCreate_GeneratesSlugAndDefaults(t *testing.T) {
	svc, repo := newTestService()

	created, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)
	assert.Equal(t, "uriad-skhvalyv-novyi-biudzhet", created.Slug)
	assert.Equal(t, "", created.Language, "language stays as provided, no default")
	assert.Equal(t, models.StatusDraft, created.Status)
	assert.NotZero(t, created.ID)
	assert.Len(t, repo.Items, 1)
}

func TestCreate_RejectsTitleWithoutTransliteration(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Title = "🔥🔥🔥"

	_, err := svc.Create(context.Background(), input)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
}

func TestCreate_RetriesAutoSlugOnConflict(t *testing.T) {
	svc, repo := newTestService()
	repo.Conflicts["uriad-skhvalyv-novyi-biudzhet"] = true

	created, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)
	assert.Equal(t, "uriad-skhvalyv-novyi-biudzhet-2", created.Slug)
	assert.Equal(t, 2, repo.CreateCalls)
}

func TestCreate_ExhaustsSlugAttempts(t *testing.T) {
	svc, repo := newTestService()
	repo.Conflicts["uriad-skhvalyv-novyi-biudzhet"] = true
	for attempt := 2; attempt <= maxSlugAttempts; attempt++ {
		repo.Conflicts[fmt.Sprintf("uriad-skhvalyv-novyi-biudzhet-%d", attempt)] = true
	}

	_, err := svc.Create(context.Background(), validInput())
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrConflict), "expected ErrConflict, got %v", err)
	assert.Equal(t, maxSlugAttempts, repo.CreateCalls)
}

func TestCreate_Validation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(input *NewsInput)
	}{
		{"empty title", func(in *NewsInput) { in.Title = "" }},
		{"unknown category", func(in *NewsInput) { in.Category = models.Category("Фантастика") }},
		{"unknown status", func(in *NewsInput) { in.Status = models.Status("archived") }},
		{"published without text", func(in *NewsInput) { in.Status = models.StatusPublished; in.Text = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService()
			input := validInput()
			tt.mutate(&input)

			_, err := svc.Create(context.Background(), input)
			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
		})
	}
}

func TestCreate_ReturnsFieldScopedValidationError(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Status = models.StatusPublished
	input.Text = ""

	_, err := svc.Create(context.Background(), input)
	require.Error(t, err)

	var validationErr *errs.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected *errs.ValidationError, got %v", err)
	assert.Equal(t, "text", validationErr.Field)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput))
}

func TestCreate_AllowsDraftWithoutText(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Text = ""

	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDraft, created.Status)
}

func TestCreate_AcceptsNewCategories(t *testing.T) {
	tests := []struct {
		name     string
		category models.Category
	}{
		{"economics", models.CategoryEconomics},
		{"politics", models.CategoryPolitics},
		{"regional", models.CategoryRegional},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			input := validInput()
			input.Category = tt.category

			created, err := svc.Create(context.Background(), input)
			require.NoError(t, err, "a new rubric must pass category validation")
			assert.Equal(t, tt.category, created.Category)
			assert.Contains(t, repo.Items, created.ID)
		})
	}
}

func TestCreate_PublishSetsPublishedAt(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Status = models.StatusPublished

	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, created.PublishedAt)
}

func TestCreate_PersistsMainEvent(t *testing.T) {
	svc, repo := newTestService()

	input := validInput()
	input.MainEvent = true
	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)

	assert.True(t, created.MainEvent)
	require.Contains(t, repo.Items, created.ID)
	assert.True(t, repo.Items[created.ID].MainEvent)
}

func TestCreate_PublishedMainEventReplacesPrevious(t *testing.T) {
	svc, repo := newTestService()

	previousInput := validInput()
	previousInput.Title = "Перша головна подія"
	previousInput.Status = models.StatusPublished
	previousInput.MainEvent = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	replacementInput := validInput()
	replacementInput.Title = "Друга головна подія"
	replacementInput.Status = models.StatusPublished
	replacementInput.MainEvent = true
	replacement, err := svc.Create(context.Background(), replacementInput)
	require.NoError(t, err)

	mainEvents := publishedMainEventItems(repo)
	require.Len(t, mainEvents, 1, "exactly one published article may remain the main event")
	assert.Equal(t, replacement.ID, mainEvents[0].ID)
}

func TestCreate_MainEventDraftDoesNotReplacePublished(t *testing.T) {
	svc, repo := newTestService()

	publishedInput := validInput()
	publishedInput.Title = "Опублікована головна подія"
	publishedInput.Status = models.StatusPublished
	publishedInput.MainEvent = true
	published, err := svc.Create(context.Background(), publishedInput)
	require.NoError(t, err)

	draftInput := validInput()
	draftInput.Title = "Чернетка головної події"
	draftInput.MainEvent = true
	draft, err := svc.Create(context.Background(), draftInput)
	require.NoError(t, err)

	mainEvents := publishedMainEventItems(repo)
	require.Len(t, mainEvents, 1, "a main-event draft must not displace the published main event")
	assert.Equal(t, published.ID, mainEvents[0].ID)
	assert.True(t, repo.Items[draft.ID].MainEvent, "main-event intent must be retained on the draft")
}

func TestCreate_MainEventPerCategory(t *testing.T) {
	svc, repo := newTestService()

	newsInput := validInput()
	newsInput.Category = models.CategoryNews
	newsInput.Title = "Головна подія в новинах"
	newsInput.Status = models.StatusPublished
	newsInput.MainEvent = true
	newsArticle, err := svc.Create(context.Background(), newsInput)
	require.NoError(t, err)

	sportInput := validInput()
	sportInput.Category = models.CategoryInsides
	sportInput.Title = "Головна подія у спорті"
	sportInput.Status = models.StatusPublished
	sportInput.MainEvent = true
	sportArticle, err := svc.Create(context.Background(), sportInput)
	require.NoError(t, err)

	require.NotEqual(t, newsArticle.Slug, sportArticle.Slug)

	storedNews, err := repo.GetBySlug(context.Background(), newsArticle.Slug)
	require.NoError(t, err)
	assert.True(t, storedNews.MainEvent,
		"a main event in Новини must survive a main event in Спорт")
	assert.True(t, repo.Items[newsArticle.ID].MainEvent)

	storedSport, err := repo.GetBySlug(context.Background(), sportArticle.Slug)
	require.NoError(t, err)
	assert.True(t, storedSport.MainEvent)

	assert.Len(t, publishedMainEventItems(repo), 2,
		"each category may keep its own published main event")
}

func TestCreate_PublishedFeaturedReplacesPrevious(t *testing.T) {
	svc, repo := newTestService()

	previousInput := validInput()
	previousInput.Title = "Перша стаття"
	previousInput.Status = models.StatusPublished
	previousInput.Featured = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	replacementInput := validInput()
	replacementInput.Title = "Друга стаття"
	replacementInput.Status = models.StatusPublished
	replacementInput.Featured = true
	replacement, err := svc.Create(context.Background(), replacementInput)
	require.NoError(t, err)

	featured := publishedFeaturedItems(repo)
	require.Len(t, featured, 1, "exactly one published article may remain featured")
	assert.Equal(t, replacement.ID, featured[0].ID)
}

func TestCreate_FeaturedDraftDoesNotReplacePublished(t *testing.T) {
	svc, repo := newTestService()

	publishedInput := validInput()
	publishedInput.Title = "Перша стаття"
	publishedInput.Status = models.StatusPublished
	publishedInput.Featured = true
	published, err := svc.Create(context.Background(), publishedInput)
	require.NoError(t, err)

	draftInput := validInput()
	draftInput.Title = "Чернетка статті"
	draftInput.Featured = true
	draft, err := svc.Create(context.Background(), draftInput)
	require.NoError(t, err)

	featured := publishedFeaturedItems(repo)
	require.Len(t, featured, 1, "a featured draft must not displace the published featured article")
	assert.Equal(t, published.ID, featured[0].ID)
	assert.True(t, repo.Items[draft.ID].Featured, "featured intent must be retained on the draft")
}

func TestCreate_RejectsBlankTitle(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Title = "   "

	_, err := svc.Create(context.Background(), input)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
}

func TestCreate_TrimsTitle(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Title = "  Уряд схвалив бюджет  "

	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "Уряд схвалив бюджет", created.Title)
}

func TestGetBySlug_NotFound(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.GetBySlug(context.Background(), "missing-article")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}

func TestList_ValidatesPagination(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.List(context.Background(), 0, 10, "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput))

	_, err = svc.List(context.Background(), 1, 0, "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput))
}

func TestList_ClampsLimitToMax(t *testing.T) {
	svc, repo := newTestService()

	result, err := svc.List(context.Background(), 1, 100000, "", "")
	require.NoError(t, err)
	assert.Equal(t, MaxPageLimit, result.Limit)
	assert.Equal(t, MaxPageLimit+1, repo.LastFilter.Limit)
	assert.Equal(t, 0, repo.LastFilter.Offset)
}

func TestList_RejectsUnknownStatus(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.List(context.Background(), 1, 10, "archived", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput))
}

func TestList_EmptyStatusMeansNoFilter(t *testing.T) {
	svc, repo := newTestService()

	_, err := svc.List(context.Background(), 1, 10, "", "")
	require.NoError(t, err)
	assert.Nil(t, repo.LastFilter.Status)
}

func TestUpdate_NotFound(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.Update(context.Background(), "missing-article", validInput())
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}

func TestUpdate_KeepsPublishedAtWhenRevertingToDraft(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Status = models.StatusPublished
	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, created.PublishedAt)
	publishedAt := *created.PublishedAt

	update := validInput()
	update.Status = models.StatusDraft
	updated, err := svc.Update(context.Background(), created.Slug, update)
	require.NoError(t, err)
	require.NotNil(t, updated.PublishedAt)
	assert.Equal(t, publishedAt, *updated.PublishedAt, "published_at must survive reverting to draft")
}

func TestUpdate_AdvancesUpdatedAt(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)

	time.Sleep(time.Millisecond)

	updated, err := svc.Update(context.Background(), created.Slug, validInput())
	require.NoError(t, err)

	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt), "update must advance updated_at")
}

func TestUpdate_PreservesCreatedAt(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), created.Slug, validInput())
	require.NoError(t, err)

	assert.Equal(t, created.CreatedAt, updated.CreatedAt, "update must keep created_at")
}

func TestUpdate_DoesNotOverwritePublishedAt(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Status = models.StatusPublished
	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	publishedAt := *created.PublishedAt

	update := validInput()
	update.Status = models.StatusPublished
	updated, err := svc.Update(context.Background(), created.Slug, update)
	require.NoError(t, err)
	require.NotNil(t, updated.PublishedAt)
	assert.Equal(t, publishedAt, *updated.PublishedAt)
}

func TestUpdate_FirstPublicationSetsPublishedAt(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)
	require.Nil(t, created.PublishedAt)

	update := validInput()
	update.Status = models.StatusPublished

	updated, err := svc.Update(context.Background(), created.Slug, update)
	require.NoError(t, err)
	require.NotNil(t, updated.PublishedAt, "first publication must set published_at")
}

func TestUpdate_PersistsMainEvent(t *testing.T) {
	svc, repo := newTestService()

	created, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)

	update := validInput()
	update.MainEvent = true
	updated, err := svc.Update(context.Background(), created.Slug, update)
	require.NoError(t, err)

	assert.True(t, updated.MainEvent)
	assert.True(t, repo.Items[created.ID].MainEvent)
}

func TestUpdate_PublishedMainEventReplacesPrevious(t *testing.T) {
	svc, repo := newTestService()

	previousInput := validInput()
	previousInput.Title = "Перша головна подія"
	previousInput.Status = models.StatusPublished
	previousInput.MainEvent = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	targetInput := validInput()
	targetInput.Title = "Друга головна подія"
	targetInput.Status = models.StatusPublished
	target, err := svc.Create(context.Background(), targetInput)
	require.NoError(t, err)

	updateInput := targetInput
	updateInput.MainEvent = true
	updated, err := svc.Update(context.Background(), target.Slug, updateInput)
	require.NoError(t, err)

	mainEvents := publishedMainEventItems(repo)
	require.Len(t, mainEvents, 1, "exactly one published article may remain the main event")
	assert.Equal(t, updated.ID, mainEvents[0].ID)
}

func TestUpdate_MainEventDraftDoesNotReplacePublished(t *testing.T) {
	svc, repo := newTestService()

	publishedInput := validInput()
	publishedInput.Title = "Опублікована головна подія"
	publishedInput.Status = models.StatusPublished
	publishedInput.MainEvent = true
	published, err := svc.Create(context.Background(), publishedInput)
	require.NoError(t, err)

	draftInput := validInput()
	draftInput.Title = "Чернетка головної події"
	draft, err := svc.Create(context.Background(), draftInput)
	require.NoError(t, err)

	updateInput := draftInput
	updateInput.MainEvent = true
	updated, err := svc.Update(context.Background(), draft.Slug, updateInput)
	require.NoError(t, err)

	mainEvents := publishedMainEventItems(repo)
	require.Len(t, mainEvents, 1, "a main-event draft must not displace the published main event")
	assert.Equal(t, published.ID, mainEvents[0].ID)
	assert.True(t, updated.MainEvent, "main-event intent must be retained on the draft")
	assert.True(t, repo.Items[draft.ID].MainEvent)
}

func TestUpdate_PublishedFeaturedReplacesPrevious(t *testing.T) {
	svc, repo := newTestService()

	previousInput := validInput()
	previousInput.Title = "Перша стаття"
	previousInput.Status = models.StatusPublished
	previousInput.Featured = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	targetInput := validInput()
	targetInput.Title = "Друга стаття"
	targetInput.Status = models.StatusPublished
	target, err := svc.Create(context.Background(), targetInput)
	require.NoError(t, err)

	updateInput := targetInput
	updateInput.Featured = true
	updated, err := svc.Update(context.Background(), target.Slug, updateInput)
	require.NoError(t, err)

	featured := publishedFeaturedItems(repo)
	require.Len(t, featured, 1, "exactly one published article may remain featured")
	assert.Equal(t, updated.ID, featured[0].ID)
}

func TestUpdate_AppliesEditableFields(t *testing.T) {
	svc, _ := newTestService()

	created, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)

	update := validInput()
	update.Title = "Оновлений заголовок"
	update.Category = models.CategoryAnalytics
	update.Description = "Новий опис"
	update.Featured = true
	update.ImageURL = "https://example.com/cover.jpg"

	updated, err := svc.Update(context.Background(), created.Slug, update)
	require.NoError(t, err)
	assert.Equal(t, "Оновлений заголовок", updated.Title)
	assert.Equal(t, models.CategoryAnalytics, updated.Category)
	assert.Equal(t, "Новий опис", updated.Description)
	assert.True(t, updated.Featured)
	assert.Equal(t, "https://example.com/cover.jpg", updated.ImageURL)
}

func TestDelete_NotFound(t *testing.T) {
	svc, _ := newTestService()

	err := svc.Delete(context.Background(), "missing-article")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}

func TestCreate_LogsFailureWithFunctionContext(t *testing.T) {
	repo := testsupport.NewStubRepo()
	repo.CreateErr = errors.New("db is down")
	handler := &captureHandler{}
	svc := NewNewsService(repo, slog.New(handler), nil, "", nil)

	_, err := svc.Create(context.Background(), validInput())
	require.Error(t, err)
	assert.True(t, handler.hasFC("newsService_Create"), "expected a log record carrying the fc identifier")
}

func TestListPublished_ValidCategoryReachesFilter(t *testing.T) {
	tests := []struct {
		name     string
		category models.Category
	}{
		{"news", models.CategoryNews},
		{"international", models.CategoryAnalytics},
		{"sport", models.CategoryInsides},
		{"interview", models.CategoryImportant},
		{"economics", models.CategoryEconomics},
		{"politics", models.CategoryPolitics},
		{"regional", models.CategoryRegional},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			result, err := svc.ListPublished(context.Background(), 1, 10, string(tt.category))
			require.NoError(t, err)

			require.NotNil(t, repo.LastPublishedFilter.Category,
				"a valid category must be forwarded to the repository")
			assert.Equal(t, tt.category, *repo.LastPublishedFilter.Category,
				"the filter must carry the exact category value")
			assert.Equal(t, 1, result.Page)
			assert.Equal(t, 10, result.Limit)
		})
	}
}

func TestListPublished_EmptyCategoryLeavesFilterNil(t *testing.T) {
	svc, repo := newTestService()

	_, err := svc.ListPublished(context.Background(), 1, 10, "")
	require.NoError(t, err)

	assert.Nil(t, repo.LastPublishedFilter.Category,
		"an omitted category must leave the filter unfiltered")
}

func TestListPublished_RejectsUnknownCategory(t *testing.T) {
	tests := []struct {
		name     string
		category string
	}{
		{"unknown rubric", "Фантастика"},
		{"lowercase sport", "спорт"},
		{"trailing space", "Спорт "},
		{"arbitrary text", "unknown"},
		{"old investigation rubric", "Про нас"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			_, err := svc.ListPublished(context.Background(), 1, 10, tt.category)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, models.ListFilter{}, repo.LastPublishedFilter,
				"an unknown category must not reach the repository")
		})
	}
}

func TestListPublished_UnknownCategorySkipsRepositoryEvenOnFailure(t *testing.T) {
	cause := errors.New("db is down")
	svc, repo := newTestService()
	repo.ListPublishedErr = cause

	_, err := svc.ListPublished(context.Background(), 1, 10, "Фантастика")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.False(t, errors.Is(err, cause),
		"category validation must happen before any repository access")
	assert.Equal(t, models.ListFilter{}, repo.LastPublishedFilter,
		"an unknown category must not reach the repository")
}

func TestList_ValidCategoryReachesFilter(t *testing.T) {
	tests := []struct {
		name     string
		category models.Category
	}{
		{"news", models.CategoryNews},
		{"international", models.CategoryAnalytics},
		{"sport", models.CategoryInsides},
		{"interview", models.CategoryImportant},
		{"economics", models.CategoryEconomics},
		{"politics", models.CategoryPolitics},
		{"regional", models.CategoryRegional},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			result, err := svc.List(context.Background(), 1, 10, "", string(tt.category))
			require.NoError(t, err)

			require.NotNil(t, repo.LastFilter.Category,
				"a valid category must be forwarded to the repository")
			assert.Equal(t, tt.category, *repo.LastFilter.Category,
				"the filter must carry the exact category value")
			assert.Nil(t, repo.LastFilter.Status,
				"an omitted status must leave the filter unfiltered")
			assert.Equal(t, 1, result.Page)
			assert.Equal(t, 10, result.Limit)
		})
	}
}

func TestList_EmptyCategoryLeavesFilterNil(t *testing.T) {
	svc, repo := newTestService()

	_, err := svc.List(context.Background(), 1, 10, "", "")
	require.NoError(t, err)

	assert.Nil(t, repo.LastFilter.Category,
		"an omitted category must leave the filter unfiltered")
}

func TestList_RejectsUnknownCategory(t *testing.T) {
	tests := []struct {
		name     string
		category string
	}{
		{"unknown rubric", "Фантастика"},
		{"lowercase sport", "спорт"},
		{"trailing space", "Спорт "},
		{"arbitrary text", "unknown"},
		{"old investigation rubric", "Про нас"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			_, err := svc.List(context.Background(), 1, 10, "", tt.category)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, models.ListFilter{}, repo.LastFilter,
				"an unknown category must not reach the repository")
		})
	}
}

func TestList_StatusAndCategoryTogetherPopulateFilter(t *testing.T) {
	svc, repo := newTestService()

	result, err := svc.List(context.Background(), 2, 5, string(models.StatusPublished), string(models.CategoryInsides))
	require.NoError(t, err)

	require.NotNil(t, repo.LastFilter.Status,
		"a valid status must be forwarded to the repository")
	assert.Equal(t, models.StatusPublished, *repo.LastFilter.Status)
	require.NotNil(t, repo.LastFilter.Category,
		"a valid category must be forwarded to the repository")
	assert.Equal(t, models.CategoryInsides, *repo.LastFilter.Category)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 5, result.Limit)
}

func TestList_UnknownCategorySkipsRepositoryEvenOnFailure(t *testing.T) {
	cause := errors.New("db is down")
	svc, repo := newTestService()
	repo.ListErr = cause

	_, err := svc.List(context.Background(), 1, 10, "", "Фантастика")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.False(t, errors.Is(err, cause),
		"category validation must happen before any repository access")
	assert.Equal(t, models.ListFilter{}, repo.LastFilter,
		"an unknown category must not reach the repository")
}

// --- Pagination: has_more ---
// The repository gets limit+1 rows; the extra row is trimmed and sets has_more.

func TestList_RequestsExtraRowForNextPage(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		limit      int
		wantLimit  int
		wantOffset int
	}{
		{"first page", 1, 10, 11, 0},
		{"second page", 2, 10, 11, 10},
		{"third page small limit", 3, 2, 3, 4},
		{"clamped limit", 1, 100000, MaxPageLimit + 1, 0},
		{"clamped limit later page", 3, 100000, MaxPageLimit + 1, 2 * MaxPageLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			result, err := svc.List(context.Background(), tt.page, tt.limit, "", "")

			require.NoError(t, err)
			assert.Equal(t, tt.wantLimit, repo.LastFilter.Limit,
				"the repository must receive one extra row to detect a next page")
			assert.Equal(t, tt.wantOffset, repo.LastFilter.Offset)
			assert.False(t, result.HasMore, "an empty repository has no next page")
		})
	}
}

func TestList_TrimsExtraRowAndSetsHasMore(t *testing.T) {
	svc, repo := newTestService()
	seedListNews(repo, 3)

	result, err := svc.List(context.Background(), 1, 2, "", "")

	require.NoError(t, err)
	require.Len(t, result.Items, 2, "the extra probe row must be trimmed from the result")
	assert.True(t, result.HasMore, "three rows with limit 2 must report a next page")
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.Limit)
}

func TestList_HasMoreFalseWhenFittingInLimit(t *testing.T) {
	tests := []struct {
		name   string
		stored int
		limit  int
	}{
		{"fewer than limit", 1, 2},
		{"exactly limit", 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()
			seedListNews(repo, tt.stored)

			result, err := svc.List(context.Background(), 1, tt.limit, "", "")

			require.NoError(t, err)
			assert.Len(t, result.Items, tt.stored)
			assert.False(t, result.HasMore)
		})
	}
}

func TestList_SecondPageHasMore(t *testing.T) {
	svc, repo := newTestService()
	seedListNews(repo, 5)

	result, err := svc.List(context.Background(), 2, 2, "", "")

	require.NoError(t, err)
	assert.Equal(t, 3, repo.LastFilter.Limit)
	assert.Equal(t, 2, repo.LastFilter.Offset)
	require.Len(t, result.Items, 2)
	assert.True(t, result.HasMore, "rows beyond the second page must report a next page")
}

func TestListPublished_RequestsExtraRowForNextPage(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		limit      int
		wantLimit  int
		wantOffset int
	}{
		{"first page", 1, 10, 11, 0},
		{"second page", 2, 5, 6, 5},
		{"clamped limit", 2, 100000, MaxPageLimit + 1, MaxPageLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			result, err := svc.ListPublished(context.Background(), tt.page, tt.limit, "")

			require.NoError(t, err)
			assert.Equal(t, tt.wantLimit, repo.LastPublishedFilter.Limit,
				"the repository must receive one extra row to detect a next page")
			assert.Equal(t, tt.wantOffset, repo.LastPublishedFilter.Offset)
			assert.False(t, result.HasMore, "an empty repository has no next page")
		})
	}
}

func TestListPublished_TrimsExtraRowAndSetsHasMore(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	createPublished(t, svc, "First published")
	createPublished(t, svc, "Second published")
	createPublished(t, svc, "Third published")

	result, err := svc.ListPublished(ctx, 1, 2, "")

	require.NoError(t, err)
	require.Len(t, result.Items, 2, "the extra probe row must be trimmed from the result")
	assert.True(t, result.HasMore, "three published rows with limit 2 must report a next page")
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.Limit)
}

func TestListPublished_HasMoreFalseWhenFittingInLimit(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	createPublished(t, svc, "Only published")

	result, err := svc.ListPublished(ctx, 1, 2, "")

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.False(t, result.HasMore)
}

func TestSearchPublished_AlwaysHasMoreFalse(t *testing.T) {
	repo := &searchFakeRepo{}
	svc := newSearchTestService(repo)
	ctx := context.Background()

	for _, title := range []string{"Уряд один", "Уряд два", "Уряд три"} {
		repo.items = append(repo.items, &models.News{Title: title, Status: models.StatusPublished})
	}

	result, err := svc.SearchPublished(ctx, "уряд", 2)

	require.NoError(t, err)
	assert.False(t, result.HasMore, "search is a single suggestion page and never reports a next page")
}

// --- Slug: fixed at creation, never changes on update ---
// Title edits keep the original slug, including suffixed collision slugs.

func TestUpdate_KeepsSlugWhenTitleChanges(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	created, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	originalSlug := created.Slug

	update := validInput()
	update.Title = "Цілком інший заголовок"

	updated, err := svc.Update(ctx, originalSlug, update)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, originalSlug, updated.Slug,
		"the slug is fixed at creation and must not follow a title change")
	assert.Equal(t, "Цілком інший заголовок", updated.Title)
	stored, err := repo.GetBySlug(ctx, originalSlug)
	require.NoError(t, err, "the article must stay reachable under its original slug")
	assert.Equal(t, originalSlug, stored.Slug)
}

func TestUpdate_SuffixedSlugRemainsStable(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	seedSlugArticle(repo, "foo", "Foo")
	existing := seedSlugArticle(repo, "foo-2", "Foo")

	update := validInput()
	update.Title = "Foo revised"

	updated, err := svc.Update(ctx, "foo-2", update)

	require.NoError(t, err,
		"updating an article whose slug carries a numeric suffix must succeed while foo exists")
	require.NotNil(t, updated)
	assert.Equal(t, "foo-2", updated.Slug,
		"a suffixed slug must be kept, not renamed or rejected as a conflict")
	assert.Equal(t, "Foo revised", updated.Title)
	assert.Equal(t, "foo-2", repo.Items[existing.ID].Slug)
}

// --- Logging: what is logged and what stays quiet ---
// Expected misses stay quiet; infra failures log with function context.

func TestGetBySlug_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.GetBySlug(context.Background(), "missing-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a missing article is an expected outcome and must not be logged as an error")
}

func TestGetBySlug_InfraFailureStillLogs(t *testing.T) {
	cause := errors.New("db is down")
	repo := testsupport.NewStubRepo()
	repo.GetErr = cause
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.GetBySlug(context.Background(), "any-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, cause), "the repository cause must be preserved, got %v", err)
	assert.True(t, handler.hasFC("newsService_GetBySlug"),
		"an infrastructure failure must still be logged with its function context")
}

func TestGetPublishedBySlug_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.GetPublishedBySlug(context.Background(), "missing-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a missing published article is an expected outcome and must not be logged as an error")
}

func TestGetPublishedBySlug_DraftEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	draft, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)

	_, err = svc.GetPublishedBySlug(context.Background(), draft.Slug)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a draft behind the public endpoint is an expected outcome and must not be logged as an error")
}

func TestUpdate_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.Update(context.Background(), "missing-article", validInput())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"updating a missing article is an expected outcome and must not be logged as an error")
}

func TestDelete_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	err := svc.Delete(context.Background(), "missing-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"deleting a missing article is an expected outcome and must not be logged as an error")
}

func TestCreate_ValidationFailureEmitsNoErrorLog(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(input *NewsInput)
	}{
		{"empty title", func(in *NewsInput) { in.Title = "" }},
		{"unknown category", func(in *NewsInput) { in.Category = "Фантастика" }},
		{"published without text", func(in *NewsInput) {
			in.Status = "published"
			in.Text = ""
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testsupport.NewStubRepo()
			handler := &captureHandler{}
			svc := loggingService(repo, handler)
			input := validInput()
			tt.mutate(&input)

			_, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Zero(t, countErrorRecords(handler),
				"a validation failure is an expected outcome and must not be logged as an error")
		})
	}
}

func TestUpdate_ValidationFailureEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)
	ctx := context.Background()

	created, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Zero(t, countErrorRecords(handler))

	update := validInput()
	update.Title = ""

	_, err = svc.Update(ctx, created.Slug, update)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a validation failure is an expected outcome and must not be logged as an error")
}

func TestCreate_SlugConflictEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	repo.Conflicts["uriad-skhvalyv-novyi-biudzhet"] = true
	for attempt := 2; attempt <= maxSlugAttempts; attempt++ {
		repo.Conflicts[fmt.Sprintf("uriad-skhvalyv-novyi-biudzhet-%d", attempt)] = true
	}

	_, err := svc.Create(context.Background(), validInput())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrConflict), "expected ErrConflict, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"an exhausted slug retry is an expected outcome and must not be logged as an error")
}

// --- Covers: uploaded files, pasted links, R2 keys and cleanup ---
// Covers uploads, pasted URLs, R2 keys and orphan cleanup on failure.

func TestService_Create_CoverKeyUniquePerUpload(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/first.jpg"}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Len(t, storage.uploadKeys, 1)
	assertCoverKey(t, storage.uploadKeys[0], created.Slug, "jpg",
		"every upload must use a fresh unique key, never covers/<slug>.<ext>")
}

func TestService_CoverKeys_DifferAcrossUploads(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/rotated.jpg"}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "first.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}
	created, err := svc.Create(ctx, input)
	require.NoError(t, err)
	require.Len(t, storage.uploadKeys, 1)

	update := validInput()
	update.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "second.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}
	updated, err := svc.Update(ctx, created.Slug, update)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, storage.uploadKeys, 2)

	assertCoverKey(t, storage.uploadKeys[0], created.Slug, "jpg")
	assertCoverKey(t, storage.uploadKeys[1], updated.Slug, "jpg")
	assert.NotEqual(t, storage.uploadKeys[0], storage.uploadKeys[1],
		"two uploads for the same article must produce different keys")
}

func TestService_Create_FetchedCoverKeyUniquePerUpload(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/fetched.jpg"}
	repo := testsupport.NewStubRepo()
	fetcher := &fakeFetcher{data: fetchTestPNG, mime: "image/png"}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	input := validInput()
	input.ImageURL = "https://pics.example/pasted-cover"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Len(t, storage.uploadKeys, 1)
	assertCoverKey(t, storage.uploadKeys[0], created.Slug, "png",
		"a fetched pasted URL must be stored under a unique key")
	assert.Equal(t, fetchTestPNG, storage.uploadBody)
}

func TestService_Update_FailedRepoDeletesOnlyNewKey(t *testing.T) {
	const oldKey = "covers/cleanup-article-aaaaaaaa.jpg"
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/cleanup-article-new.jpg"}
	repo := testsupport.NewStubRepo()
	existing := seedCoverArticle(repo, "cleanup-article", coverTestPublicBaseURL+"/"+oldKey)
	svc := newCoverTestService(repo, storage)

	repo.UpdateErr = assert.AnError

	input := validInput()
	input.Title = "cleanup-article"
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "replacement.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	updated, err := svc.Update(context.Background(), existing.Slug, input)

	require.Error(t, err, "a failed repository update must fail the cover rotation")
	assert.Nil(t, updated)
	require.Len(t, storage.uploadKeys, 1, "the replacement cover must be uploaded before the update")
	newKey := storage.uploadKeys[0]
	assertCoverKey(t, newKey, existing.Slug, "jpg")
	require.Len(t, storage.deleteKeys, 1,
		"only the newly uploaded object may be cleaned up after a failed update")
	assert.Equal(t, newKey, storage.deleteKeys[0])
	assert.NotContains(t, storage.deleteKeys, oldKey,
		"the existing cover must never be deleted when the update fails")
	assert.Equal(t, coverTestPublicBaseURL+"/"+oldKey, repo.Items[existing.ID].ImageURL,
		"the stored article must keep its previous cover after a failed update")
}

func TestService_Update_SuccessDeletesOldOwnedKey(t *testing.T) {
	const oldKey = "covers/legacy-article-aaaaaaaa.jpg"
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/legacy-article-new.jpg"}
	repo := testsupport.NewStubRepo()
	existing := seedCoverArticle(repo, "legacy-article", coverTestPublicBaseURL+"/"+oldKey)
	svc := newCoverTestService(repo, storage)

	input := validInput()
	input.Title = "legacy-article"
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "replacement.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	updated, err := svc.Update(context.Background(), existing.Slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, storage.uploadKeys, 1)
	assertCoverKey(t, storage.uploadKeys[0], existing.Slug, "jpg")
	assert.Equal(t, []string{oldKey}, storage.deleteKeys,
		"after a successful update the old owned cover must be deleted")
	assert.Equal(t, storage.uploadURL, updated.ImageURL)
}

func TestService_Create_CoverBytesMismatchFailsBeforeWrite(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		body     []byte
	}{
		{"png bytes named jpg", "x.jpg", fetchTestPNG},
		{"jpeg bytes named png", "x.png", fetchTestJPEG},
		{"plain text named png", "x.png", []byte("hello")},
		{"plain text named jpg", "x.jpg", []byte("hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/mismatch.jpg"}
			repo := testsupport.NewStubRepo()
			handler := &captureHandler{}
			svc := coverValidationService(repo, storage, handler)

			input := validInput()
			input.Cover = &CoverUpload{
				Body:        bytes.NewReader(tt.body),
				Filename:    tt.filename,
				ContentType: "image/jpeg",
				Size:        int64(len(tt.body)),
			}

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr),
				"a cover whose bytes do not match its extension must be a ValidationError, got %v", err)
			assert.Equal(t, "cover", validationErr.Field)
			assert.Zero(t, repo.CreateCalls, "a mismatched cover must be rejected before any repository write")
			assert.Empty(t, storage.uploadKeys, "a mismatched cover must never be uploaded")
			assert.Empty(t, repo.Items)
			assert.Zero(t, countErrorRecords(handler),
				"a cover validation failure must not emit an error-level log record")
		})
	}
}

func TestService_Update_CoverBytesMismatchFailsBeforeWrite(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/mismatch.jpg"}
	repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
	handler := &captureHandler{}
	svc := coverValidationService(repo.StubRepo, storage, handler)
	ctx := context.Background()

	existing, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Zero(t, repo.updateCalls)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestPNG)),
		Filename:    "x.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestPNG)),
	}

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	var validationErr *errs.ValidationError
	require.True(t, errors.As(err, &validationErr),
		"a cover whose bytes do not match its extension must be a ValidationError, got %v", err)
	assert.Equal(t, "cover", validationErr.Field)
	assert.Zero(t, repo.updateCalls, "a mismatched cover must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys, "a mismatched cover must never be uploaded")
	assert.Zero(t, countErrorRecords(handler),
		"a cover validation failure must not emit an error-level log record")
}

func TestService_Create_OversizedCoverFailsBeforeWrite(t *testing.T) {
	const oversized = (10 << 20) + 1

	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/huge.jpg"}
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := coverValidationService(repo, storage, handler)

	body := make([]byte, oversized)
	copy(body, fetchTestJPEG)
	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(body),
		Filename:    "huge.jpg",
		ContentType: "image/jpeg",
		Size:        oversized,
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrPayloadTooLarge),
		"a cover larger than 10 MB must surface ErrPayloadTooLarge, got %v", err)
	assert.Zero(t, repo.CreateCalls, "an oversized cover must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys, "an oversized cover must never be uploaded")
	assert.Empty(t, repo.Items)
	assert.Zero(t, countErrorRecords(handler),
		"an oversized cover must not emit an error-level log record")
}

func TestService_Update_OversizedCoverFailsBeforeWrite(t *testing.T) {
	const oversized = (10 << 20) + 1

	storage := &fakeImageStorage{}
	repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
	handler := &captureHandler{}
	svc := coverValidationService(repo.StubRepo, storage, handler)
	ctx := context.Background()

	existing, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Zero(t, repo.updateCalls)

	body := make([]byte, oversized)
	copy(body, fetchTestPNG)
	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(body),
		Filename:    "huge.png",
		ContentType: "image/png",
		Size:        oversized,
	}

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrPayloadTooLarge),
		"a cover larger than 10 MB must surface ErrPayloadTooLarge, got %v", err)
	assert.Zero(t, repo.updateCalls, "an oversized cover must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys, "an oversized cover must never be uploaded")
	assert.Zero(t, countErrorRecords(handler),
		"an oversized cover must not emit an error-level log record")
}

func TestService_Create_UploadFailureRemovesRowAndFails(t *testing.T) {
	storage := &fakeImageStorage{uploadErr: errors.New("r2 down")}
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := coverValidationService(repo, storage, handler)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "an upload failure after the row was inserted must fail the create")
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"an upload failure must surface ErrServiceUnavailable, got %v", err)
	assert.Empty(t, repo.Items, "the created row must be removed when the upload fails")
}

func TestService_Create_CoverURLSaveFailureRemovesRowAndObject(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/saved.jpg"}
	inner := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := coverValidationService(inner, storage, handler)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}
	inner.UpdateErr = errors.New("database is down")

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed cover URL save must fail the create")
	assert.Nil(t, created)
	require.Len(t, storage.uploadKeys, 1, "the cover upload must be attempted once")
	assert.Equal(t, storage.uploadKeys, storage.deleteKeys,
		"the uploaded object must be deleted when saving its URL fails")
	assert.Empty(t, inner.Items, "the created row must be removed when saving its URL fails")
}

func TestService_Create_UploadsCoverByFinalSlug(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: "https://cdn.example.com/covers/uploaded.jpg"}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)

	input := validInput()
	baseSlug := slugify(input.Title)
	repo.Conflicts[baseSlug] = true
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "Cat.JPG",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, baseSlug+"-2", created.Slug)
	assert.Equal(t, 2, repo.CreateCalls, "the slug conflict must be retried once")
	require.Len(t, storage.uploadKeys, 1)
	assertCoverKey(t, storage.uploadKeys[0], baseSlug+"-2", "jpg",
		"the cover key must use the final slug, a unique suffix and a lowercased extension")
	assert.Equal(t, "image/jpeg", storage.uploadTypes[0])
	assert.Equal(t, fetchTestJPEG, storage.uploadBody, "the upload must receive the cover body")
	assert.Equal(t, storage.uploadURL, created.ImageURL)
	assert.Equal(t, storage.uploadURL, repo.Items[created.ID].ImageURL,
		"the uploaded cover URL must be persisted on the article")
}

func TestService_Create_InvalidCoverExtFails(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader("payload"),
		Filename:    "evil.exe",
		ContentType: "image/jpeg",
		Size:        int64(len("payload")),
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Zero(t, repo.CreateCalls,
		"an invalid cover extension must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys)
}

func TestService_Update_NewCoverReplacesOldKey(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/old-news.webp"}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.Title = "old-news"
	seed.ImageURL = coverTestPublicBaseURL + "/covers/old-news.png"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)
	require.Equal(t, "old-news", existing.Slug)
	require.Empty(t, storage.uploadKeys, "creating without a cover must not upload")

	update := seed
	update.Cover = &CoverUpload{
		Body:        strings.NewReader(string(coverTestWEBP)),
		Filename:    "new.webp",
		ContentType: "image/webp",
		Size:        int64(len(coverTestWEBP)),
	}

	updated, err := svc.Update(ctx, existing.Slug, update)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, storage.uploadKeys, 1)
	assertCoverKey(t, storage.uploadKeys[0], "old-news", "webp")
	assert.Equal(t, storage.uploadURL, updated.ImageURL)
	assert.Equal(t, storage.uploadURL, repo.Items[updated.ID].ImageURL,
		"the new cover URL must be persisted")
	assert.Equal(t, []string{"covers/old-news.png"}, storage.deleteKeys,
		"the replaced cover must be removed from our bucket")
}

func TestService_Delete_RemovesCoverBestEffort(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.ImageURL = coverTestPublicBaseURL + "/covers/pic.jpg"
	created, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	storage.deleteErr = errors.New("storage is down")

	err = svc.Delete(ctx, created.Slug)

	require.NoError(t, err, "a cover cleanup failure must not fail the article deletion")
	assert.Equal(t, 1, repo.deleteCalls, "the article must be deleted from the repository")
	assert.Equal(t, []string{"covers/pic.jpg"}, storage.deleteKeys)
	assert.NotContains(t, repo.Items, created.ID)
}

func TestService_Delete_ExternalURLSkipped(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.ImageURL = "https://other.example/pic.jpg"
	created, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	err = svc.Delete(ctx, created.Slug)

	require.NoError(t, err)
	assert.Empty(t, storage.deleteKeys,
		"a cover outside our bucket must not be deleted")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, created.ID)
}

func TestService_Create_UploadFailureRemovesRow(t *testing.T) {
	storage := &fakeImageStorage{uploadErr: errors.New("storage is down")}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed cover upload must fail the create after the row was inserted")
	assert.Nil(t, created, "no article may be returned without its cover")
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a failed cover upload must surface ErrServiceUnavailable, got %v", err)
	require.Len(t, repo.createIDs, 1, "the row must have been inserted before the upload")
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.Items, "no orphan row may survive a failed cover upload")
	require.Len(t, storage.uploadKeys, 1, "the cover upload must be attempted once")
	assertCoverKey(t, storage.uploadKeys[0], slugify(input.Title), "jpg")
}

func TestService_Create_URLSaveFailureRemovesRowAndObject(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/saved-url.jpg"}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestJPEG)),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	repo.UpdateErr = errors.New("database is down")

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed cover URL save must fail the create")
	assert.Nil(t, created, "no article may be returned when its cover URL was not saved")
	require.Len(t, repo.createIDs, 1, "the row must have been inserted before the upload")
	require.Len(t, storage.uploadKeys, 1)
	uploadedKey := storage.uploadKeys[0]
	assertCoverKey(t, uploadedKey, slugify(input.Title), "jpg")
	assert.Equal(t, []string{uploadedKey}, storage.deleteKeys,
		"the object uploaded for the failed URL save must be deleted")
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.Items, "no orphan row may survive a failed cover URL save")
}

func TestService_Update_UploadFailureWritesNothing(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	existing, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Zero(t, repo.updateCalls, "creating without a cover must not update")

	storage.uploadErr = errors.New("storage is down")

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestPNG)),
		Filename:    "cover.png",
		ContentType: "image/png",
		Size:        int64(len(fetchTestPNG)),
	}

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Zero(t, repo.updateCalls,
		"a failed upload must abort the update before the repository is written")
}

func TestService_Update_RepoFailureCleansObject(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/cleanup-article.png"}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.Title = "cleanup-article"
	seed.ImageURL = coverTestPublicBaseURL + "/covers/cleanup-article.webp"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	repo.UpdateErr = errors.New("database is down")

	input := seed
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestPNG)),
		Filename:    "new.png",
		ContentType: "image/png",
		Size:        int64(len(fetchTestPNG)),
	}

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	require.Len(t, storage.uploadKeys, 1, "the replacement cover must be uploaded before the update")
	assertCoverKey(t, storage.uploadKeys[0], "cleanup-article", "png")
	require.Len(t, storage.deleteKeys, 1,
		"only the object uploaded for the failed update may be cleaned up")
	assertCoverKey(t, storage.deleteKeys[0], "cleanup-article", "png")
	assert.Equal(t, storage.uploadKeys[0], storage.deleteKeys[0],
		"only the freshly uploaded object is cleaned up")
	for _, key := range storage.deleteKeys {
		assert.NotContains(t, key, "cleanup-article.webp",
			"the existing cover must survive a failed update")
	}
	assert.Equal(t, existing.ImageURL, repo.Items[existing.ID].ImageURL,
		"the stored article must keep its previous cover after a failed update")
}

func TestService_Update_PreservesImageURLWithoutCover(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.Title = "legacy-cover"
	seed.ImageURL = coverTestPublicBaseURL + "/covers/legacy-cover.png"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	input := validInput()
	input.Title = "legacy-cover"
	input.ImageURL = ""

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, existing.ImageURL, updated.ImageURL,
		"an update without a new cover must keep the existing image URL")
	assert.Empty(t, storage.uploadKeys, "no cover upload may happen without a cover")
	assert.Empty(t, storage.deleteKeys, "an untouched cover must not be deleted")
}

func TestService_Create_FetchesExternalURLToR2(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/fetched.jpg"}
	repo := testsupport.NewStubRepo()
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	input := validInput()
	input.ImageURL = "https://pics.example/image"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Len(t, fetcher.calls, 1, "the external cover must be fetched exactly once")
	assert.Equal(t, "https://pics.example/image", fetcher.calls[0])
	require.Len(t, storage.uploadKeys, 1)
	assertCoverKey(t, storage.uploadKeys[0], slugify(input.Title), "jpg",
		"the fetched JPEG must be stored under the slug-derived key")
	assert.Equal(t, fetchTestJPEG, storage.uploadBody, "the fetched bytes must be uploaded")
	assert.Equal(t, storage.uploadURL, created.ImageURL,
		"the external URL must be replaced by the stored object URL")
	assert.Equal(t, storage.uploadURL, repo.Items[created.ID].ImageURL,
		"the stored cover URL must be persisted on the article")
}

func TestService_Create_FetchFailureWritesNothing(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := testsupport.NewStubRepo()
	fetcher := &fakeFetcher{err: errors.New("fetch failed")}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	input := validInput()
	input.ImageURL = "https://pics.example/image"

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.Zero(t, repo.CreateCalls,
		"a failed fetch must abort before any repository write")
	assert.Empty(t, storage.uploadKeys)
}

func TestService_Create_OwnURLNotRefetched(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := testsupport.NewStubRepo()
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	ownedURL := coverTestPublicBaseURL + "/covers/x.jpg"
	input := validInput()
	input.ImageURL = ownedURL

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Empty(t, fetcher.calls, "a URL under our own bucket must not be refetched")
	assert.Empty(t, storage.uploadKeys, "no upload may happen without a new cover")
	assert.Equal(t, ownedURL, created.ImageURL, "the owned URL must be kept as-is")
}

func TestService_Create_LegacyHotlinkWithoutFetcher(t *testing.T) {
	svc, repo := newTestService()

	input := validInput()
	input.ImageURL = "https://pics.example/image"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "https://pics.example/image", created.ImageURL,
		"without a fetcher an external URL must be stored as-is")
	assert.Equal(t, 1, repo.CreateCalls)
}

func TestService_Update_PastedURLReplacesCover(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/old-news.png"}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.Title = "old-news"
	seed.ImageURL = coverTestPublicBaseURL + "/covers/old.jpg"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)
	require.Empty(t, storage.uploadKeys)

	fetcher := &fakeFetcher{data: fetchTestPNG, mime: "image/png"}
	svc = newCoverTestServiceWithFetcher(repo, storage, fetcher)

	update := seed
	update.ImageURL = "https://pics.example/new-cover"
	updated, err := svc.Update(ctx, existing.Slug, update)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, fetcher.calls, 1)
	assert.Equal(t, "https://pics.example/new-cover", fetcher.calls[0])
	require.Len(t, storage.uploadKeys, 1)
	assertCoverKey(t, storage.uploadKeys[0], "old-news", "png",
		"the pasted PNG must be stored under the article slug")
	assert.Equal(t, storage.uploadURL, updated.ImageURL,
		"the pasted URL must be replaced by the stored object URL")
	assert.Equal(t, []string{"covers/old.jpg"}, storage.deleteKeys,
		"the replaced cover must be removed from our bucket")
}

func TestService_Create_UnsupportedFetchedTypeFails(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := testsupport.NewStubRepo()
	fetcher := &fakeFetcher{data: fetchTestTIFF, mime: "image/tiff"}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	input := validInput()
	input.ImageURL = "https://pics.example/image"

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Zero(t, repo.CreateCalls,
		"an unsupported fetched image type must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys)
}

func TestService_Delete_NonCoversURLSkipped(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.ImageURL = coverTestPublicBaseURL + "/static/logo.png"
	created, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	err = svc.Delete(ctx, created.Slug)

	require.NoError(t, err)
	assert.Empty(t, storage.deleteKeys,
		"a shared asset on our host must not be deleted")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, created.ID)
}

func TestService_Update_SameExternalURLSkipsFetch(t *testing.T) {
	tests := []struct {
		name     string
		fetchErr error
	}{
		{"fetcher healthy", nil},
		{"fetcher fails with 403", errors.New("403 Forbidden")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/should-not-upload.jpg"}
			repo := testsupport.NewStubRepo()
			ctx := context.Background()

			externalURL := "https://thumb.wikimedia.org/wikipedia/commons/thumb/cover.jpg"
			seedSvc := newCoverTestService(repo, storage)
			seed := validInput()
			seed.ImageURL = externalURL
			existing, err := seedSvc.Create(ctx, seed)
			require.NoError(t, err)
			require.Equal(t, externalURL, existing.ImageURL,
				"seeding without a fetcher must keep the external URL as-is")
			require.Empty(t, storage.uploadKeys, "seeding without a cover must not upload")

			fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg", err: tt.fetchErr}
			svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

			update := seed
			updated, err := svc.Update(ctx, existing.Slug, update)

			require.NoError(t, err,
				"resaving an article with an unchanged external cover must succeed even when the remote host fails")
			require.NotNil(t, updated)
			assert.Equal(t, externalURL, updated.ImageURL,
				"the unchanged external cover must be kept as-is")
			assert.Equal(t, externalURL, repo.Items[updated.ID].ImageURL,
				"the stored article must keep its external cover")
			assert.Empty(t, fetcher.calls,
				"an unchanged external cover must not be refetched")
			assert.Empty(t, storage.uploadKeys,
				"an unchanged external cover must not be uploaded")
			assert.Empty(t, storage.deleteKeys,
				"an unchanged external cover must not trigger any deletion")
		})
	}
}

func TestService_Update_NewExternalURLFetchesCover(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/new-cover.png"}
	repo := testsupport.NewStubRepo()
	ctx := context.Background()

	oldExternalURL := "https://thumb.wikimedia.org/wikipedia/commons/thumb/old-cover.jpg"
	seedSvc := newCoverTestService(repo, storage)
	seed := validInput()
	seed.ImageURL = oldExternalURL
	existing, err := seedSvc.Create(ctx, seed)
	require.NoError(t, err)
	require.Equal(t, oldExternalURL, existing.ImageURL,
		"seeding without a fetcher must keep the external URL as-is")
	require.Empty(t, storage.uploadKeys, "seeding without a cover must not upload")

	fetcher := &fakeFetcher{data: fetchTestPNG, mime: "image/png"}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	newExternalURL := "https://pics.example/new-cover"
	update := seed
	update.ImageURL = newExternalURL
	updated, err := svc.Update(ctx, existing.Slug, update)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, fetcher.calls, 1, "a changed external cover must be fetched exactly once")
	assert.Equal(t, newExternalURL, fetcher.calls[0])
	require.Len(t, storage.uploadKeys, 1, "a changed external cover must be uploaded")
	assertCoverKey(t, storage.uploadKeys[0], existing.Slug, "png",
		"the fetched PNG must be stored under the article slug")
	assert.Equal(t, storage.uploadURL, updated.ImageURL,
		"the new external URL must be replaced by the stored object URL")
	assert.Equal(t, storage.uploadURL, repo.Items[updated.ID].ImageURL,
		"the stored cover URL must be persisted on the article")
}

// Without cover storage a valid upload has nowhere to go: Create must fail
// with ErrServiceUnavailable before any row is written.
func TestService_Create_NilStorageWithCoverFailsUnavailable(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "x.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a cover with nil storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.CreateCalls, "no row may be written when cover storage is not configured")
	assert.Empty(t, repo.Items)
}

// Without cover storage a rotation has nowhere to go: Update must fail with
// ErrServiceUnavailable before the repository is written.
func TestService_Update_NilStorageWithCoverFailsUnavailable(t *testing.T) {
	repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
	plain := NewNewsService(repo.StubRepo, slog.New(&captureHandler{}), nil, "", nil)

	seed, err := plain.Create(context.Background(), validInput())
	require.NoError(t, err)
	require.Zero(t, repo.updateCalls)

	svc := NewNewsService(repo.StubRepo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "x.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	updated, err := svc.Update(context.Background(), seed.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a cover with nil storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.updateCalls, "no row may be updated when cover storage is not configured")
}

// --- Video: presign, pending uploads, embed links, update/delete cleanup ---
// Video presigns, pending keys, embeds and cleanup on update and delete.

func TestVideo_Constants(t *testing.T) {
	assert.Equal(t, 500<<20, MaxVideoBytes, "MaxVideoBytes must cap one video at 500 MB")
	assert.Equal(t, 15*time.Minute, VideoUploadTTL, "VideoUploadTTL must keep presigned PUTs alive for 15 minutes")
}

func TestVideo_Presign_AcceptsMP4AndWebM(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		ext         string
	}{
		{"mp4", "video/mp4", "mp4"},
		{"webm", "video/webm", "webm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{presignURL: "https://r2.example/upload?sig=1"}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)

			before := time.Now()
			upload, err := svc.PresignVideoUpload(context.Background(), UploadRequest{
				ContentType: tt.contentType,
				Size:        1024,
			})

			require.NoError(t, err)
			require.NotNil(t, upload)
			assert.Equal(t, "https://r2.example/upload?sig=1", upload.UploadURL,
				"the upload URL must be what storage returned")
			assert.Regexp(t, regexp.MustCompile(`^pending/videos/[0-9a-f]{32}\.`+tt.ext+`$`), upload.Key,
				"the key must be pending/videos/<32 hex>.<ext from type>")
			require.Len(t, storage.presignKeys, 1)
			assert.Equal(t, upload.Key, storage.presignKeys[0])
			assert.Equal(t, []string{tt.contentType}, storage.presignType)
			assert.Equal(t, []int64{1024}, storage.presignSize)
			require.Len(t, storage.presignTTL, 1)
			assert.Equal(t, VideoUploadTTL, storage.presignTTL[0],
				"storage must receive the 15 minute TTL")
			assert.WithinDuration(t, before.Add(VideoUploadTTL), upload.ExpiresAt, 2*time.Minute,
				"ExpiresAt must be now plus 15 minutes")
			assert.Empty(t, repo.Items)
		})
	}
}

func TestVideo_Presign_RejectsTypeAndSize(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		size        int64
	}{
		{"quicktime", "video/quicktime", 1024},
		{"png image", "image/png", 1024},
		{"empty type", "", 1024},
		{"json", "application/json", 1024},
		{"zero size", "video/mp4", 0},
		{"negative size", "video/webm", -1},
		{"over limit", "video/mp4", int64(MaxVideoBytes) + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{presignURL: "https://r2.example/upload"}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)

			upload, err := svc.PresignVideoUpload(context.Background(), UploadRequest{
				ContentType: tt.contentType,
				Size:        tt.size,
			})

			require.Error(t, err)
			assert.Nil(t, upload)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Empty(t, storage.presignKeys, "invalid input must not reach storage")
			assert.Empty(t, repo.Items)
		})
	}
}

func TestVideo_Presign_AtMaxSizeOk(t *testing.T) {
	storage := &fakeMediaStorage{presignURL: "https://r2.example/upload"}
	svc := newVideoTestService(testsupport.NewStubRepo(), storage)

	upload, err := svc.PresignVideoUpload(context.Background(), UploadRequest{
		ContentType: "video/mp4",
		Size:        int64(MaxVideoBytes),
	})

	require.NoError(t, err)
	require.NotNil(t, upload)
	assert.Regexp(t, regexp.MustCompile(`^pending/videos/[0-9a-f]{32}\.mp4$`), upload.Key)
}

func TestVideo_Presign_StorageErrorUnavailable(t *testing.T) {
	storage := &fakeMediaStorage{presignErr: errors.New("r2 is down")}
	svc := newVideoTestService(testsupport.NewStubRepo(), storage)

	upload, err := svc.PresignVideoUpload(context.Background(), UploadRequest{
		ContentType: "video/mp4",
		Size:        1024,
	})

	require.Error(t, err)
	assert.Nil(t, upload)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a storage outage must surface ErrServiceUnavailable, got %v", err)
}

func TestVideo_Presign_NoStorageUnavailable(t *testing.T) {
	svc := NewNewsService(testsupport.NewStubRepo(), slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	upload, err := svc.PresignVideoUpload(context.Background(), UploadRequest{
		ContentType: "video/mp4",
		Size:        1024,
	})

	require.Error(t, err)
	assert.Nil(t, upload)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a missing video storage must surface ErrServiceUnavailable, got %v", err)
}

func TestVideo_Embed_NormalizesYouTubeAndVimeo(t *testing.T) {
	const ytID = "dQw4w9WgXcQ"
	ytCanonical := "https://www.youtube-nocookie.com/embed/" + ytID
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"watch", "https://www.youtube.com/watch?v=" + ytID, ytCanonical},
		{"watch extra params", "https://www.youtube.com/watch?v=" + ytID + "&list=PL123&t=10s", ytCanonical},
		{"watch http www", "http://www.youtube.com/watch?v=" + ytID, ytCanonical},
		{"watch mobile host", "https://m.youtube.com/watch?v=" + ytID, ytCanonical},
		{"watch bare host", "https://youtube.com/watch?v=" + ytID, ytCanonical},
		{"short link", "https://youtu.be/" + ytID, ytCanonical},
		{"shorts", "https://www.youtube.com/shorts/" + ytID, ytCanonical},
		{"embed", "https://www.youtube.com/embed/" + ytID, ytCanonical},
		{"nocookie", "https://www.youtube-nocookie.com/embed/" + ytID, ytCanonical},
		{"vimeo", "https://vimeo.com/123456789", "https://player.vimeo.com/video/123456789"},
		{"vimeo player", "https://player.vimeo.com/video/123456", "https://player.vimeo.com/video/123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)

			input := validInput()
			input.VideoURL = tt.raw

			created, err := svc.Create(context.Background(), input)

			require.NoError(t, err)
			require.NotNil(t, created)
			require.NotNil(t, created.Video, "an embed link must attach a video")
			assert.Equal(t, models.VideoKindEmbed, created.Video.Kind)
			assert.Equal(t, tt.want, created.Video.URL, "the embed URL must be canonical")
			assert.Equal(t, tt.want, repo.Items[created.ID].Video.URL,
				"the canonical embed URL must be persisted")
			assert.Empty(t, storage.inspectKeys, "an embed needs no video storage")
			assert.Empty(t, storage.copySrc)
			assert.Empty(t, storage.deleteKeys)
		})
	}
}

func TestVideo_Embed_RejectsInvalid(t *testing.T) {
	const ytID = "dQw4w9WgXcQ"
	tests := []struct {
		name string
		raw  string
	}{
		{"evil host", "https://evil.com/watch?v=" + ytID},
		{"lookalike host", "https://youtube.com.evil.com/watch?v=" + ytID},
		{"javascript", "javascript:alert(1)"},
		{"short id", "https://www.youtube.com/watch?v=short"},
		{"long id", "https://www.youtube.com/watch?v=toolongid123456789"},
		{"bad chars id", "https://www.youtube.com/watch?v=bad!!id!!!!!"},
		{"missing v param", "https://www.youtube.com/watch"},
		{"youtu short id", "https://youtu.be/short"},
		{"vimeo non-digit", "https://vimeo.com/abc123"},
		{"vimeo missing id", "https://vimeo.com/"},
		{"vimeo mixed id", "https://player.vimeo.com/video/12ab34"},
		{"non-url text", "just some text, not a url"},
		{"empty path youtube", "https://www.youtube.com/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)

			input := validInput()
			input.VideoURL = tt.raw

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Zero(t, repo.CreateCalls, "an invalid embed must be rejected before any repository write")
			assert.Empty(t, repo.Items)
			assert.Empty(t, storage.inspectKeys)
		})
	}
}

func TestVideo_Embed_NeedsNoMediaStorage(t *testing.T) {
	svc := NewNewsService(testsupport.NewStubRepo(), slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	input := validInput()
	input.VideoURL = "https://youtu.be/dQw4w9WgXcQ"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, created.Video)
	assert.Equal(t, models.VideoKindEmbed, created.Video.Kind)
	assert.Equal(t, "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", created.Video.URL)
}

func TestVideo_Create_StorageURLWithoutCurrentInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"other videos key", coverTestPublicBaseURL + "/videos/other-12345678.mp4"},
		{"covers key", coverTestPublicBaseURL + "/covers/x.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newVideoTestService(testsupport.NewStubRepo(), &fakeMediaStorage{})

			input := validInput()
			input.VideoURL = tt.raw

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
		})
	}
}

func TestVideo_Create_TextWithMarkerLinesSucceeds(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, &fakeMediaStorage{})

	input := validInput()
	input.Text = "a\n[video]\nb\n[video]\nc"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err, "the backend no longer interprets [video] lines in the text")
	require.NotNil(t, created)
	assert.Nil(t, created.Video)
	require.Contains(t, repo.Items, created.ID)
	assert.Equal(t, input.Text, repo.Items[created.ID].Text, "the text must be stored unchanged")
}

func TestVideo_Create_VideoKeyInvalidFormat(t *testing.T) {
	hex32 := "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name string
		key  string
	}{
		{"wrong prefix", "videos/other/" + hex32 + ".mp4"},
		{"final location", "videos/some-slug-abcdef12.mp4"},
		{"old pending prefix", "videos/pending/" + hex32 + ".mp4"},
		{"short hex", "pending/videos/abc123.mp4"},
		{"31 hex", "pending/videos/" + strings.Repeat("a", 31) + ".mp4"},
		{"33 hex", "pending/videos/" + strings.Repeat("a", 33) + ".mp4"},
		{"traversal", "pending/videos/../evil.mp4"},
		{"mov ext", "pending/videos/" + hex32 + ".mov"},
		{"no ext", "pending/videos/" + hex32},
		{"upper ext", "pending/videos/" + hex32 + ".MP4"},
		{"upper hex", "pending/videos/" + strings.ToUpper(hex32) + ".mp4"},
		{"covers key", "covers/slug-abcdef12.mp4"},
		{"bare name", "evil.mp4"},
		{"empty", ""},
	}

	// Empty means no video, not an error: drop it from the invalid table.
	valid := tests[:len(tests)-1]

	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{
				inspectSize: 8,
				inspectHead: videoTestMP4Head(),
				copyURL:     coverTestPublicBaseURL + "/videos/x-abcdef12.mp4",
			}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)

			input := validInput()
			if tt.key != "" {
				input.VideoKey = tt.key
			} else {
				input.VideoKey = " "
			}

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Empty(t, storage.inspectKeys, "an invalid key format must not reach storage")
			assert.Empty(t, storage.deleteKeys, "an invalid key format must delete nothing")
			assert.Empty(t, storage.copySrc)
			assert.Zero(t, repo.CreateCalls)
			assert.Empty(t, repo.Items)
		})
	}
}

func TestVideo_Create_VideoKeyInspectNotFound(t *testing.T) {
	storage := &fakeMediaStorage{inspectErr: errs.ErrObjectNotFound}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)

	input := validInput()
	input.VideoKey = videoTestPendingKey("mp4")

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "a missing pending object must be invalid, got %v", err)
	assert.Equal(t, []string{videoTestPendingKey("mp4")}, storage.inspectKeys)
	assert.Empty(t, storage.deleteKeys, "a missing object must not be deleted")
	assert.Empty(t, storage.copySrc, "a missing object must not be copied")
	assert.Empty(t, repo.Items, "no row may survive a missing pending object")
}

func TestVideo_Create_VideoKeyOversized(t *testing.T) {
	pending := videoTestPendingKey("mp4")
	storage := &fakeMediaStorage{
		inspectSize: int64(MaxVideoBytes) + 1,
		inspectHead: videoTestMP4Head(),
	}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)

	input := validInput()
	input.VideoKey = pending

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Equal(t, []string{pending}, storage.deleteKeys,
		"an oversized pending object must be deleted right away")
	assert.Empty(t, storage.copySrc)
	assert.Empty(t, repo.Items)
}

func TestVideo_Create_VideoKeyBadSignature(t *testing.T) {
	tests := []struct {
		name string
		key  string
		head []byte
	}{
		{"plain text as mp4", videoTestPendingKey("mp4"), []byte("hello world, not a video")},
		{"png as mp4", videoTestPendingKey("mp4"), []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}},
		{"webm bytes under mp4", videoTestPendingKey("mp4"), videoTestWebMHead()},
		{"mp4 bytes under webm", videoTestPendingKey("webm"), videoTestMP4Head()},
		{"short input", videoTestPendingKey("mp4"), []byte{0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{inspectSize: int64(len(tt.head)), inspectHead: tt.head}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)

			input := validInput()
			input.VideoKey = tt.key

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, []string{tt.key}, storage.deleteKeys,
				"an invalid pending object must be deleted right away")
			assert.Empty(t, storage.copySrc, "bad content must not be copied")
			assert.Empty(t, repo.Items)
		})
	}
}

func TestVideo_Create_VideoKeyInspectErrorUnavailable(t *testing.T) {
	storage := &fakeMediaStorage{inspectErr: errors.New("r2 is down")}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)

	input := validInput()
	input.VideoKey = videoTestPendingKey("mp4")

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a storage outage must surface ErrServiceUnavailable, got %v", err)
	assert.Empty(t, storage.deleteKeys, "a storage outage must delete nothing")
	assert.Empty(t, repo.Items)
}

func TestVideo_Create_VideoKeySuccess(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		head func() []byte
	}{
		{"mp4", "mp4", videoTestMP4Head},
		{"webm", "webm", videoTestWebMHead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := videoTestPendingKey(tt.ext)
			storage := &fakeMediaStorage{
				inspectSize: 12345,
				inspectHead: tt.head(),
			}
			storage.copyURL = coverTestPublicBaseURL + "/videos/final-abcdef12." + tt.ext
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)
			ctx := context.Background()

			input := validInput()
			input.VideoKey = pending

			created, err := svc.Create(ctx, input)

			require.NoError(t, err)
			require.NotNil(t, created)
			require.NotNil(t, created.Video, "a valid pending object must attach a file video")
			assert.Equal(t, models.VideoKindFile, created.Video.Kind)
			assert.Equal(t, storage.copyURL, created.Video.URL)
			assert.Equal(t, storage.copyURL, repo.Items[created.ID].Video.URL,
				"the copied video URL must be persisted")
			require.Len(t, storage.inspectKeys, 1)
			assert.Equal(t, pending, storage.inspectKeys[0])
			require.Len(t, storage.inspectN, 1)
			assert.Equal(t, 512, storage.inspectN[0], "Inspect must read the first 512 bytes")
			require.Len(t, storage.copySrc, 1)
			assert.Equal(t, pending, storage.copySrc[0])
			require.Len(t, storage.copyDst, 1)
			assertVideoDstKey(t, storage.copyDst[0], created.Slug, tt.ext,
				"the copy destination must be videos/<slug>-<8 hex>.<ext>")
			assert.Contains(t, storage.deleteKeys, pending,
				"the pending object must be deleted afterwards")
		})
	}
}

func TestVideo_Create_VideoKeyCopyErrorRemovesRow(t *testing.T) {
	storage := &fakeMediaStorage{
		inspectSize: 100,
		inspectHead: videoTestMP4Head(),
		copyErr:     errors.New("copy failed"),
	}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newVideoTestService(repo, storage)

	input := validInput()
	input.VideoKey = videoTestPendingKey("mp4")

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a copy failure must surface ErrServiceUnavailable, got %v", err)
	require.Len(t, repo.createIDs, 1, "the row must have been inserted before the copy")
	assert.Equal(t, 1, repo.deleteCalls, "the half-created row must be removed")
	assert.Empty(t, repo.Items)
}

func TestVideo_Create_VideoKeySaveFailureCleansCopy(t *testing.T) {
	pending := videoTestPendingKey("mp4")
	storage := &fakeMediaStorage{
		inspectSize: 100,
		inspectHead: videoTestMP4Head(),
	}
	storage.copyURL = coverTestPublicBaseURL + "/videos/new-abcdef12.mp4"
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	repo.UpdateErr = errors.New("database is down")
	svc := newVideoTestService(repo, storage)

	input := validInput()
	input.VideoKey = pending

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed video URL save must fail the create")
	assert.Nil(t, created)
	require.Len(t, storage.copyDst, 1, "the pending object must have been copied before the save")
	assert.Contains(t, storage.deleteKeys, storage.copyDst[0],
		"the new copy must be deleted when saving its URL fails")
	assert.NotContains(t, storage.deleteKeys, pending,
		"the pending object must stay so the editor can retry")
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.Items)
}

func TestVideo_Create_VideoKeyWinsOverURL(t *testing.T) {
	pending := videoTestPendingKey("mp4")
	storage := &fakeMediaStorage{
		inspectSize: 100,
		inspectHead: videoTestMP4Head(),
		copyURL:     coverTestPublicBaseURL + "/videos/winner-abcdef12.mp4",
	}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)

	input := validInput()
	input.VideoKey = pending
	input.VideoURL = "https://youtu.be/dQw4w9WgXcQ"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, created.Video)
	assert.Equal(t, models.VideoKindFile, created.Video.Kind, "VideoKey must win over VideoURL")
	assert.Equal(t, storage.copyURL, created.Video.URL)
}

func TestVideo_Create_VideoKeyNoStorageUnavailable(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	input := validInput()
	input.VideoKey = videoTestPendingKey("mp4")

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a video key with no video storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.CreateCalls, "no row may be written without video storage")
	assert.Empty(t, repo.Items)
}

func TestVideo_Update_KeepCurrentFileURL(t *testing.T) {
	storage := &fakeMediaStorage{}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	currentURL := coverTestPublicBaseURL + "/videos/" + slug + "-abcdef12.mp4"
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: currentURL})

	input := validInput()
	input.VideoURL = currentURL

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Video)
	assert.Equal(t, models.VideoKindFile, updated.Video.Kind)
	assert.Equal(t, currentURL, updated.Video.URL, "the current file video must be kept")
	assert.Empty(t, storage.inspectKeys, "keeping the video must not touch storage")
	assert.Empty(t, storage.copySrc)
	assert.Empty(t, storage.deleteKeys, "keeping the video must delete nothing")
}

func TestVideo_Update_ReplaceWithVideoKey(t *testing.T) {
	storage := &fakeMediaStorage{
		inspectSize: 200,
		inspectHead: videoTestMP4Head(),
		copyURL:     coverTestPublicBaseURL + "/videos/video-article-12345678.mp4",
	}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})
	pending := videoTestPendingKey("mp4")

	input := validInput()
	input.VideoKey = pending

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Video)
	assert.Equal(t, models.VideoKindFile, updated.Video.Kind)
	assert.Equal(t, storage.copyURL, updated.Video.URL)
	require.Len(t, storage.copySrc, 1)
	assert.Equal(t, pending, storage.copySrc[0])
	assertVideoDstKey(t, storage.copyDst[0], slug, "mp4")
	assert.Contains(t, storage.deleteKeys, pending, "the pending object must be deleted")
	assert.Contains(t, storage.deleteKeys, oldKey, "the old file must be deleted after the save")
}

func TestVideo_Update_ReplaceWithEmbed(t *testing.T) {
	storage := &fakeMediaStorage{}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})

	input := validInput()
	input.VideoURL = "https://youtu.be/dQw4w9WgXcQ"

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Video)
	assert.Equal(t, models.VideoKindEmbed, updated.Video.Kind)
	assert.Equal(t, "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", updated.Video.URL)
	assert.Equal(t, []string{oldKey}, storage.deleteKeys,
		"the previous owned file must be deleted after the save")
}

func TestVideo_Update_ClearWithEmpty(t *testing.T) {
	storage := &fakeMediaStorage{}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})

	input := validInput()

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Nil(t, updated.Video, "empty video fields must clear the video")
	assert.Equal(t, []string{oldKey}, storage.deleteKeys)
}

func TestVideo_Update_RejectsForeignStorageURL(t *testing.T) {
	slug := "video-article"
	currentURL := coverTestPublicBaseURL + "/videos/" + slug + "-abcdef12.mp4"
	tests := []struct {
		name string
		raw  string
	}{
		{"other videos key", coverTestPublicBaseURL + "/videos/other-12345678.mp4"},
		{"same prefix different suffix", coverTestPublicBaseURL + "/videos/" + slug + "-00000000.mp4"},
		{"covers key", coverTestPublicBaseURL + "/covers/x.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)
			ctx := context.Background()

			seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: currentURL})

			input := validInput()
			input.VideoURL = tt.raw

			updated, err := svc.Update(ctx, slug, input)

			require.Error(t, err)
			assert.Nil(t, updated)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Empty(t, storage.deleteKeys, "a rejected URL must delete nothing")
			stored := repo.Items[seededID(repo, slug)]
			require.NotNil(t, stored.Video)
			assert.Equal(t, currentURL, stored.Video.URL, "the stored video must stay untouched")
		})
	}
}

func TestVideo_Update_SaveFailureKeepsOld(t *testing.T) {
	pending := videoTestPendingKey("mp4")
	storage := &fakeMediaStorage{
		inspectSize: 200,
		inspectHead: videoTestMP4Head(),
		copyURL:     coverTestPublicBaseURL + "/videos/video-article-12345678.mp4",
	}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})
	repo.UpdateErr = errors.New("database is down")

	input := validInput()
	input.VideoKey = pending

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	require.Len(t, storage.copyDst, 1, "the new copy must have been written before the save")
	assert.Contains(t, storage.deleteKeys, storage.copyDst[0], "the new copy must be deleted")
	assert.NotContains(t, storage.deleteKeys, oldKey, "the old file must survive a failed update")
	id := seededID(repo, slug)
	assert.Equal(t, oldURL, repo.Items[id].Video.URL)
}

func TestVideo_Update_ClearEmbedNoStorageCall(t *testing.T) {
	storage := &fakeMediaStorage{}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "embed-article"
	seedVideoArticle(repo, slug, &models.Video{
		Kind: models.VideoKindEmbed,
		URL:  "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
	})

	input := validInput()

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Nil(t, updated.Video, "empty fields must clear an embed video")
	assert.Empty(t, storage.deleteKeys, "clearing an embed must not touch storage")
}

func TestVideo_Delete_RemovesOwnedFile(t *testing.T) {
	storage := &fakeMediaStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	key := "videos/" + slug + "-abcdef12.mp4"
	seeded := seedVideoArticle(repo.StubRepo, slug, &models.Video{
		Kind: models.VideoKindFile,
		URL:  coverTestPublicBaseURL + "/" + key,
	})

	err := svc.Delete(ctx, seeded.Slug)

	require.NoError(t, err)
	assert.Equal(t, []string{key}, storage.deleteKeys)
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestVideo_Delete_StorageErrorStillSucceeds(t *testing.T) {
	storage := &fakeMediaStorage{deleteErr: errors.New("r2 is down")}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	key := "videos/" + slug + "-abcdef12.mp4"
	seeded := seedVideoArticle(repo.StubRepo, slug, &models.Video{
		Kind: models.VideoKindFile,
		URL:  coverTestPublicBaseURL + "/" + key,
	})

	err := svc.Delete(ctx, seeded.Slug)

	require.NoError(t, err, "a video cleanup failure must not fail the article deletion")
	assert.Equal(t, []string{key}, storage.deleteKeys)
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestVideo_Delete_EmbedNoStorageCall(t *testing.T) {
	storage := &fakeMediaStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	seeded := seedVideoArticle(repo.StubRepo, "embed-article", &models.Video{
		Kind: models.VideoKindEmbed,
		URL:  "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
	})

	err := svc.Delete(ctx, seeded.Slug)

	require.NoError(t, err)
	assert.Empty(t, storage.deleteKeys, "deleting an embed video must not touch storage")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestVideo_Update_VideoKeyCopyErrorKeepsOld(t *testing.T) {
	pending := videoTestPendingKey("mp4")
	storage := &fakeMediaStorage{
		inspectSize: 200,
		inspectHead: videoTestMP4Head(),
		copyErr:     errors.New("copy failed"),
	}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})

	input := validInput()
	input.VideoKey = pending

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a copy failure must surface ErrServiceUnavailable, got %v", err)
	require.Len(t, storage.copySrc, 1, "the pending object must have been copied before the failure")
	assert.Equal(t, pending, storage.copySrc[0])
	assert.NotContains(t, storage.deleteKeys, oldKey, "the old file must survive a failed copy")
	assert.NotContains(t, storage.deleteKeys, pending, "a failed copy must delete nothing")
	id := seededID(repo, slug)
	require.NotNil(t, repo.Items[id])
	require.NotNil(t, repo.Items[id].Video)
	assert.Equal(t, oldURL, repo.Items[id].Video.URL, "the stored video must stay untouched")
}

func TestVideo_Delete_NoStorageWithFileVideoSucceeds(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)
	ctx := context.Background()

	slug := "video-article"
	key := "videos/" + slug + "-abcdef12.mp4"
	seeded := seedVideoArticle(repo, slug, &models.Video{
		Kind: models.VideoKindFile,
		URL:  coverTestPublicBaseURL + "/" + key,
	})

	var err error

	require.NotPanics(t, func() {
		err = svc.Delete(ctx, seeded.Slug)
	}, "deleting a file video without video storage must not panic")
	require.NoError(t, err)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestVideo_Update_VideoKeyInvalidFormatDeletesNothing(t *testing.T) {
	hex32 := "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name string
		key  string
	}{
		{"wrong prefix", "videos/other/" + hex32 + ".mp4"},
		{"old pending prefix", "videos/pending/" + hex32 + ".mp4"},
		{"short hex", "pending/videos/abc123.mp4"},
		{"traversal", "pending/videos/../evil.mp4"},
		{"mov ext", "pending/videos/" + hex32 + ".mov"},
		{"upper ext", "pending/videos/" + hex32 + ".MP4"},
		{"bare name", "evil.mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{
				inspectSize: 8,
				inspectHead: videoTestMP4Head(),
			}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)
			ctx := context.Background()

			slug := "video-article"
			oldKey := "videos/" + slug + "-abcdef12.mp4"
			oldURL := coverTestPublicBaseURL + "/" + oldKey
			seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})

			input := validInput()
			input.VideoKey = tt.key

			updated, err := svc.Update(ctx, slug, input)

			require.Error(t, err)
			assert.Nil(t, updated)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Empty(t, storage.inspectKeys, "an invalid key format must not reach storage")
			assert.Empty(t, storage.deleteKeys, "an invalid key format must delete nothing")
			assert.Empty(t, storage.copySrc)
			stored := repo.Items[seededID(repo, slug)]
			require.NotNil(t, stored.Video)
			assert.Equal(t, oldURL, stored.Video.URL, "the stored video must stay untouched")
		})
	}
}

func TestVideo_Update_VideoKeyInspectNotFoundDeletesNothing(t *testing.T) {
	storage := &fakeMediaStorage{inspectErr: errs.ErrObjectNotFound}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})
	pending := videoTestPendingKey("mp4")

	input := validInput()
	input.VideoKey = pending

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "a missing pending object must be invalid, got %v", err)
	assert.Equal(t, []string{pending}, storage.inspectKeys)
	assert.Empty(t, storage.deleteKeys, "a missing object must not be deleted")
	assert.Empty(t, storage.copySrc, "a missing object must not be copied")
	stored := repo.Items[seededID(repo, slug)]
	require.NotNil(t, stored.Video)
	assert.Equal(t, oldURL, stored.Video.URL, "the stored video must stay untouched")
}

func TestVideo_Update_VideoKeyOversizedDeletesPending(t *testing.T) {
	pending := videoTestPendingKey("mp4")
	storage := &fakeMediaStorage{
		inspectSize: int64(MaxVideoBytes) + 1,
		inspectHead: videoTestMP4Head(),
	}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})

	input := validInput()
	input.VideoKey = pending

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Equal(t, []string{pending}, storage.deleteKeys,
		"an oversized pending object must be deleted right away")
	assert.Empty(t, storage.copySrc)
	stored := repo.Items[seededID(repo, slug)]
	require.NotNil(t, stored.Video)
	assert.Equal(t, oldURL, stored.Video.URL, "the stored video must stay untouched")
}

func TestVideo_Update_VideoKeyBadSignatureDeletesPending(t *testing.T) {
	tests := []struct {
		name string
		key  string
		head []byte
	}{
		{"plain text as mp4", videoTestPendingKey("mp4"), []byte("hello world, not a video")},
		{"png as mp4", videoTestPendingKey("mp4"), []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}},
		{"webm bytes under mp4", videoTestPendingKey("mp4"), videoTestWebMHead()},
		{"mp4 bytes under webm", videoTestPendingKey("webm"), videoTestMP4Head()},
		{"short input", videoTestPendingKey("mp4"), []byte{0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{inspectSize: int64(len(tt.head)), inspectHead: tt.head}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)
			ctx := context.Background()

			slug := "video-article"
			oldKey := "videos/" + slug + "-abcdef12.mp4"
			oldURL := coverTestPublicBaseURL + "/" + oldKey
			seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})

			input := validInput()
			input.VideoKey = tt.key

			updated, err := svc.Update(ctx, slug, input)

			require.Error(t, err)
			assert.Nil(t, updated)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, []string{tt.key}, storage.deleteKeys,
				"an invalid pending object must be deleted right away")
			assert.Empty(t, storage.copySrc, "bad content must not be copied")
			stored := repo.Items[seededID(repo, slug)]
			require.NotNil(t, stored.Video)
			assert.Equal(t, oldURL, stored.Video.URL, "the stored video must stay untouched")
		})
	}
}

func TestVideo_Update_VideoKeyInspectErrorDeletesNothing(t *testing.T) {
	storage := &fakeMediaStorage{inspectErr: errors.New("r2 is down")}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})
	pending := videoTestPendingKey("mp4")

	input := validInput()
	input.VideoKey = pending

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a storage outage must surface ErrServiceUnavailable, got %v", err)
	assert.Empty(t, storage.deleteKeys, "a storage outage must delete nothing")
	stored := repo.Items[seededID(repo, slug)]
	require.NotNil(t, stored.Video)
	assert.Equal(t, oldURL, stored.Video.URL, "the stored video must stay untouched")
}

func TestVideo_Create_VideoKeyValidationDeleteErrorStillInvalid(t *testing.T) {
	tests := []struct {
		name string
		key  string
		head []byte
		size int64
	}{
		{"oversized", videoTestPendingKey("mp4"), videoTestMP4Head(), int64(MaxVideoBytes) + 1},
		{"bad signature", videoTestPendingKey("mp4"), []byte("hello world, not a video"), int64(len("hello world, not a video"))},
		{"ext mismatch", videoTestPendingKey("webm"), videoTestMP4Head(), int64(32)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{
				inspectSize: tt.size,
				inspectHead: tt.head,
				deleteErr:   errors.New("delete failed"),
			}
			repo := testsupport.NewStubRepo()
			svc := newVideoTestService(repo, storage)

			input := validInput()
			input.VideoKey = tt.key

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput),
				"a failed cleanup must keep the validation error, got %v", err)
			assert.Equal(t, []string{tt.key}, storage.deleteKeys,
				"the invalid pending object must still be offered for deletion")
			assert.Empty(t, storage.copySrc)
			assert.Empty(t, repo.Items)
		})
	}
}

func TestVideo_Update_VideoKeyValidationDeleteErrorStillInvalid(t *testing.T) {
	pending := videoTestPendingKey("mp4")
	storage := &fakeMediaStorage{
		inspectSize: int64(MaxVideoBytes) + 1,
		inspectHead: videoTestMP4Head(),
		deleteErr:   errors.New("delete failed"),
	}
	repo := testsupport.NewStubRepo()
	svc := newVideoTestService(repo, storage)
	ctx := context.Background()

	slug := "video-article"
	oldKey := "videos/" + slug + "-abcdef12.mp4"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedVideoArticle(repo, slug, &models.Video{Kind: models.VideoKindFile, URL: oldURL})

	input := validInput()
	input.VideoKey = pending

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput),
		"a failed cleanup must keep the validation error, got %v", err)
	assert.Equal(t, []string{pending}, storage.deleteKeys,
		"the invalid pending object must still be offered for deletion")
	assert.Empty(t, storage.copySrc)
	stored := repo.Items[seededID(repo, slug)]
	require.NotNil(t, stored.Video)
	assert.Equal(t, oldURL, stored.Video.URL, "the stored video must stay untouched")
}

// --- Images: presign, pending uploads, links, order, limits, cleanup ---
// Image presigns, pending keys and links, order and limits, update and delete cleanup.

func TestImages_MaxImagesIsTen(t *testing.T) {
	assert.Equal(t, 10, MaxImages, "at most 10 extra images per article")
}

func TestImage_Presign_AcceptsImages(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		ext         string
	}{
		{"jpeg", "image/jpeg", "jpg"},
		{"png", "image/png", "png"},
		{"webp", "image/webp", "webp"},
		{"gif", "image/gif", "gif"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{presignURL: "https://r2.example/upload?sig=1"}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, storage, &fakeImageStorage{}, nil)

			before := time.Now()
			upload, err := svc.PresignImageUpload(context.Background(), UploadRequest{
				ContentType: tt.contentType,
				Size:        1024,
			})

			require.NoError(t, err)
			require.NotNil(t, upload)
			assert.Equal(t, "https://r2.example/upload?sig=1", upload.UploadURL,
				"the upload URL must be what storage returned")
			assert.Regexp(t, regexp.MustCompile(`^pending/images/[0-9a-f]{32}\.`+tt.ext+`$`), upload.Key,
				"the key must be pending/images/<32 hex>.<ext from type>")
			require.Len(t, storage.presignKeys, 1)
			assert.Equal(t, upload.Key, storage.presignKeys[0])
			assert.Equal(t, []string{tt.contentType}, storage.presignType)
			assert.Equal(t, []int64{1024}, storage.presignSize)
			require.Len(t, storage.presignTTL, 1)
			assert.Equal(t, VideoUploadTTL, storage.presignTTL[0],
				"storage must receive the 15 minute TTL")
			assert.WithinDuration(t, before.Add(VideoUploadTTL), upload.ExpiresAt, 2*time.Minute,
				"ExpiresAt must be now plus 15 minutes")
			assert.Empty(t, repo.Items)
		})
	}
}

func TestImage_Presign_RejectsTypeAndSize(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		size        int64
	}{
		{"mp4 video", "video/mp4", 1024},
		{"webm video", "video/webm", 1024},
		{"tiff image", "image/tiff", 1024},
		{"bmp image", "image/bmp", 1024},
		{"jpg mime alias", "image/jpg", 1024},
		{"empty type", "", 1024},
		{"upper MIME", "IMAGE/JPEG", 1024},
		{"json", "application/json", 1024},
		{"zero size", "image/jpeg", 0},
		{"negative size", "image/png", -1},
		{"over limit", "image/jpeg", int64(MaxCoverBytes) + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeMediaStorage{presignURL: "https://r2.example/upload"}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, storage, &fakeImageStorage{}, nil)

			upload, err := svc.PresignImageUpload(context.Background(), UploadRequest{
				ContentType: tt.contentType,
				Size:        tt.size,
			})

			require.Error(t, err)
			assert.Nil(t, upload)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Empty(t, storage.presignKeys, "invalid input must not reach storage")
			assert.Empty(t, repo.Items)
		})
	}
}

func TestImage_Presign_AtMaxSizeOk(t *testing.T) {
	storage := &fakeMediaStorage{presignURL: "https://r2.example/upload"}
	svc := newImageTestService(testsupport.NewStubRepo(), storage, &fakeImageStorage{}, nil)

	upload, err := svc.PresignImageUpload(context.Background(), UploadRequest{
		ContentType: "image/jpeg",
		Size:        int64(MaxCoverBytes),
	})

	require.NoError(t, err)
	require.NotNil(t, upload)
	assert.Regexp(t, regexp.MustCompile(`^pending/images/[0-9a-f]{32}\.jpg$`), upload.Key)
}

func TestImage_Presign_StorageErrorUnavailable(t *testing.T) {
	storage := &fakeMediaStorage{presignErr: errors.New("r2 is down")}
	svc := newImageTestService(testsupport.NewStubRepo(), storage, &fakeImageStorage{}, nil)

	upload, err := svc.PresignImageUpload(context.Background(), UploadRequest{
		ContentType: "image/jpeg",
		Size:        1024,
	})

	require.Error(t, err)
	assert.Nil(t, upload)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a storage outage must surface ErrServiceUnavailable, got %v", err)
}

func TestImage_Presign_NoStorageUnavailable(t *testing.T) {
	svc := NewNewsService(testsupport.NewStubRepo(), slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	upload, err := svc.PresignImageUpload(context.Background(), UploadRequest{
		ContentType: "image/jpeg",
		Size:        1024,
	})

	require.Error(t, err)
	assert.Nil(t, upload)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a missing media storage must surface ErrServiceUnavailable, got %v", err)
}

func TestImage_Create_PendingSuccess(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		head []byte
		size int64
	}{
		{"jpg", "jpg", fetchTestJPEG, 12345},
		{"png at max size", "png", fetchTestPNG, int64(MaxCoverBytes)},
		{"webp", "webp", coverTestWEBP, 100},
		{"gif", "gif", imageTestGIF, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := imageTestPendingKey(tt.ext)
			storage := &fakeMediaStorage{
				inspectSize: tt.size,
				inspectHead: tt.head,
				copyURL:     coverTestPublicBaseURL + "/images/final-abcdef12." + tt.ext,
			}
			repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
			svc := newImageTestService(repo, storage, &fakeImageStorage{}, nil)

			input := validInput()
			input.Images = []string{pending}

			created, err := svc.Create(context.Background(), input)

			require.NoError(t, err)
			require.NotNil(t, created)
			require.Len(t, created.ImageURLs, 1, "a valid pending object must attach one image")
			assert.Equal(t, storage.copyURL, created.ImageURLs[0])
			assert.Equal(t, storage.copyURL, repo.Items[created.ID].ImageURLs[0],
				"the copied image URL must be persisted")
			require.Len(t, storage.inspectKeys, 1)
			assert.Equal(t, pending, storage.inspectKeys[0])
			require.Len(t, storage.inspectN, 1)
			assert.Equal(t, 512, storage.inspectN[0], "Inspect must read the first 512 bytes")
			require.Len(t, storage.copySrc, 1)
			assert.Equal(t, pending, storage.copySrc[0])
			require.Len(t, storage.copyDst, 1)
			assertImageDstKey(t, storage.copyDst[0], created.Slug, tt.ext,
				"the copy destination must be images/<slug>-<8 hex>.<ext>")
			assert.Contains(t, storage.deleteKeys, pending,
				"the pending object must be deleted afterwards")
			assert.Equal(t, 1, repo.updateCalls, "images are stored after the row INSERT, then one Update")
		})
	}
}

func TestImage_Create_MixedOrderPreserved(t *testing.T) {
	pending := imageTestPendingKey("png")
	media := &fakeMediaStorage{
		inspectSize: 100,
		inspectHead: fetchTestPNG,
		copyURL:     coverTestPublicBaseURL + "/images/mixed-aaaaaaaa.png",
		uploadURL:   coverTestPublicBaseURL + "/images/mixed-bbbbbbbb.jpg",
	}
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)

	link := "https://pics.example/shot"
	input := validInput()
	input.Images = []string{pending, link}

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, []string{media.copyURL, media.uploadURL}, created.ImageURLs,
		"the saved ImageURLs must follow the request order exactly")
	assert.Equal(t, created.ImageURLs, repo.Items[created.ID].ImageURLs,
		"the ordered ImageURLs must be persisted")
	require.Len(t, fetcher.calls, 1, "the external link must be fetched exactly once")
	assert.Equal(t, link, fetcher.calls[0])
	require.Len(t, media.uploadKeys, 1)
	assertImageDstKey(t, media.uploadKeys[0], created.Slug, "jpg",
		"the fetched JPEG must be stored under the slug key with the MIME ext")
	assert.Equal(t, []string{"image/jpeg"}, media.uploadTypes)
	assert.Equal(t, fetchTestJPEG, media.uploadBody, "the fetched bytes must be uploaded")
	require.Len(t, media.copyDst, 1)
	assertImageDstKey(t, media.copyDst[0], created.Slug, "png")
	assert.Contains(t, media.deleteKeys, pending)
}

func TestImage_Create_ExternalSuccessExtFromMIME(t *testing.T) {
	media := &fakeMediaStorage{uploadURL: coverTestPublicBaseURL + "/images/ext-cccccccc.webp"}
	fetcher := &fakeFetcher{data: coverTestWEBP, mime: "image/webp"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)

	input := validInput()
	input.Images = []string{"https://pics.example/image.png"}

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Len(t, created.ImageURLs, 1)
	assert.Equal(t, media.uploadURL, created.ImageURLs[0],
		"the external URL must be replaced by the stored object URL")
	require.Len(t, media.uploadKeys, 1)
	assertImageDstKey(t, media.uploadKeys[0], created.Slug, "webp",
		"the key ext must come from the fetched MIME, not the link suffix")
	assert.Equal(t, []string{"image/webp"}, media.uploadTypes)
}

func TestImage_Create_RejectedBeforeStorage(t *testing.T) {
	hex32 := "0123456789abcdef0123456789abcdef"
	link := "https://pics.example/a"
	otherLink := "https://pics.example/b"
	tests := []struct {
		name   string
		images []string
	}{
		{"eleven items", imageTestElevenKeys()},
		{"duplicate pending", []string{imageTestPendingKey("jpg"), imageTestPendingKey("jpg")}},
		{"duplicate links", []string{link, link}},
		{"duplicate foreign is still duplicate", []string{link, otherLink, link}},
		{"other article image", []string{coverTestPublicBaseURL + "/images/other-12345678.jpg"}},
		{"covers key", []string{coverTestPublicBaseURL + "/covers/x.jpg"}},
		{"videos key", []string{coverTestPublicBaseURL + "/videos/x-12345678.mp4"}},
		{"empty string", []string{""}},
		{"blank string", []string{"   "}},
		{"non-url text", []string{"just some text, not a url"}},
		{"ftp scheme", []string{"ftp://example.com/x.jpg"}},
		{"videos prefix", []string{"pending/videos/" + hex32 + ".mp4"}},
		{"images prefix with mp4", []string{"pending/images/" + hex32 + ".mp4"}},
		{"short hex", []string{"pending/images/abc123.jpg"}},
		{"31 hex", []string{"pending/images/" + hex32[:31] + ".jpg"}},
		{"33 hex", []string{"pending/images/" + hex32 + "a.jpg"}},
		{"traversal", []string{"pending/images/../evil.jpg"}},
		{"unknown ext", []string{"pending/images/" + hex32 + ".bmp"}},
		{"jpeg ext is not canonical", []string{"pending/images/" + hex32 + ".jpeg"}},
		{"upper ext", []string{"pending/images/" + hex32 + ".JPG"}},
		{"upper hex", []string{"pending/images/" + "0123456789ABCDEF0123456789ABCDEF" + ".jpg"}},
		{"bare name", []string{"evil.jpg"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &fakeMediaStorage{
				inspectSize: 8,
				inspectHead: fetchTestJPEG,
				copyURL:     coverTestPublicBaseURL + "/images/x-abcdef12.jpg",
				uploadURL:   coverTestPublicBaseURL + "/images/x-abcdef12.jpg",
			}
			fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, media, nil, fetcher)

			input := validInput()
			input.Images = tt.images

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Zero(t, repo.CreateCalls, "a rejected image list must be refused before any repository write")
			assert.Empty(t, repo.Items)
			assert.Empty(t, media.inspectKeys, "a rejected image list must not reach storage")
			assert.Empty(t, media.copySrc)
			assert.Empty(t, media.deleteKeys, "a rejected image list must delete nothing")
			assert.Empty(t, media.uploadKeys, "a rejected image list must not upload")
			assert.Empty(t, fetcher.calls, "a rejected image list must not fetch")
		})
	}
}

func TestImage_Create_PendingNotFound(t *testing.T) {
	media := &fakeMediaStorage{inspectErr: errs.ErrObjectNotFound}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)

	input := validInput()
	input.Images = []string{imageTestPendingKey("jpg")}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "a missing pending object must be invalid, got %v", err)
	assert.Equal(t, []string{imageTestPendingKey("jpg")}, media.inspectKeys)
	assert.Empty(t, media.deleteKeys, "a missing object must not be deleted")
	assert.Empty(t, media.copySrc, "a missing object must not be copied")
	assert.Empty(t, repo.Items, "no row may survive a missing pending object")
}

func TestImage_Create_PendingOversized(t *testing.T) {
	pending := imageTestPendingKey("jpg")
	media := &fakeMediaStorage{
		inspectSize: int64(MaxCoverBytes) + 1,
		inspectHead: fetchTestJPEG,
	}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)

	input := validInput()
	input.Images = []string{pending}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Equal(t, []string{pending}, media.deleteKeys,
		"an oversized pending object must be deleted right away")
	assert.Empty(t, media.copySrc)
	assert.Empty(t, repo.Items)
}

func TestImage_Create_PendingBadSignature(t *testing.T) {
	tests := []struct {
		name string
		key  string
		head []byte
	}{
		{"plain text as jpg", imageTestPendingKey("jpg"), []byte("hello world, not an image")},
		{"png as jpg", imageTestPendingKey("jpg"), fetchTestPNG},
		{"jpeg as png", imageTestPendingKey("png"), fetchTestJPEG},
		{"webp as gif", imageTestPendingKey("gif"), coverTestWEBP},
		{"gif as webp", imageTestPendingKey("webp"), imageTestGIF},
		{"short input", imageTestPendingKey("jpg"), []byte{0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &fakeMediaStorage{inspectSize: int64(len(tt.head)), inspectHead: tt.head}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)

			input := validInput()
			input.Images = []string{tt.key}

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, []string{tt.key}, media.deleteKeys,
				"an invalid pending object must be deleted right away")
			assert.Empty(t, media.copySrc, "bad content must not be copied")
			assert.Empty(t, repo.Items)
		})
	}
}

func TestImage_Create_PendingInspectErrorUnavailable(t *testing.T) {
	media := &fakeMediaStorage{inspectErr: errors.New("r2 is down")}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)

	input := validInput()
	input.Images = []string{imageTestPendingKey("jpg")}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a storage outage must surface ErrServiceUnavailable, got %v", err)
	assert.Empty(t, media.deleteKeys, "a storage outage must delete nothing")
	assert.Empty(t, repo.Items)
}

func TestImage_Create_PendingValidationDeleteErrorStillInvalid(t *testing.T) {
	tests := []struct {
		name string
		key  string
		head []byte
		size int64
	}{
		{"oversized", imageTestPendingKey("jpg"), fetchTestJPEG, int64(MaxCoverBytes) + 1},
		{"bad signature", imageTestPendingKey("png"), fetchTestJPEG, int64(len(fetchTestJPEG))},
		{"ext mismatch", imageTestPendingKey("gif"), coverTestWEBP, int64(len(coverTestWEBP))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &fakeMediaStorage{
				inspectSize: tt.size,
				inspectHead: tt.head,
				deleteErr:   errors.New("delete failed"),
			}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)

			input := validInput()
			input.Images = []string{tt.key}

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput),
				"a failed cleanup must keep the validation error, got %v", err)
			assert.Equal(t, []string{tt.key}, media.deleteKeys,
				"the invalid pending object must still be offered for deletion")
			assert.Empty(t, media.copySrc)
			assert.Empty(t, repo.Items)
		})
	}
}

func TestImage_Create_ExternalUnsupportedType(t *testing.T) {
	media := &fakeMediaStorage{}
	fetcher := &fakeFetcher{data: fetchTestTIFF, mime: "image/tiff"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)

	input := validInput()
	input.Images = []string{"https://pics.example/image"}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Zero(t, repo.CreateCalls,
		"an unsupported fetched image type must be rejected before any repository write")
	assert.Empty(t, media.uploadKeys)
	assert.Empty(t, media.copySrc)
}

func TestImage_Create_ExternalTooLarge(t *testing.T) {
	oversized := make([]byte, MaxCoverBytes+1)
	copy(oversized, fetchTestJPEG)
	media := &fakeMediaStorage{}
	fetcher := &fakeFetcher{data: oversized, mime: "image/jpeg"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)

	input := validInput()
	input.Images = []string{"https://pics.example/huge"}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrPayloadTooLarge), "expected ErrPayloadTooLarge, got %v", err)
	assert.Zero(t, repo.CreateCalls, "an oversized fetch must be rejected before any repository write")
	assert.Empty(t, media.uploadKeys, "an oversized fetch must not be uploaded")
}

func TestImage_Create_FetchErrors(t *testing.T) {
	invalidErr := errs.NewValidationError("images", "host is blocked")
	plainErr := errors.New("connection reset")
	tests := []struct {
		name     string
		fetchErr error
		check    func(t *testing.T, err error)
	}{
		{"validation stays invalid", invalidErr, func(t *testing.T, err error) {
			t.Helper()
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
		}},
		{"plain error as is", plainErr, func(t *testing.T, err error) {
			t.Helper()
			assert.True(t, errors.Is(err, plainErr), "a fetch error must be returned as-is, got %v", err)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &fakeMediaStorage{}
			fetcher := &fakeFetcher{err: tt.fetchErr}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, media, nil, fetcher)

			input := validInput()
			input.Images = []string{"https://pics.example/image"}

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			tt.check(t, err)
			assert.Zero(t, repo.CreateCalls, "a failed fetch must abort before any repository write")
			assert.Empty(t, media.uploadKeys)
			assert.Empty(t, media.copySrc)
		})
	}
}

func TestImage_Create_NilMediaWithPendingUnavailable(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), &fakeImageStorage{}, coverTestPublicBaseURL, nil)

	input := validInput()
	input.Images = []string{imageTestPendingKey("jpg")}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a image pending key with no media storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.CreateCalls, "no row may be written without media storage")
	assert.Empty(t, repo.Items)
}

func TestImage_Create_NilMediaWithLinkUnavailable(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), &fakeImageStorage{}, coverTestPublicBaseURL,
		&fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"})

	input := validInput()
	input.Images = []string{"https://pics.example/image"}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a image link with no media storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.CreateCalls, "no row may be written without media storage")
}

func TestImage_Create_NilFetcherWithLinkUnavailable(t *testing.T) {
	media := &fakeMediaStorage{}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, nil)

	input := validInput()
	input.Images = []string{"https://pics.example/image"}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a image link with no fetcher must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.CreateCalls, "no row may be written without a fetcher")
	assert.Empty(t, media.uploadKeys)
}

func TestImage_Create_NilImageStorageWithLinkSucceeds(t *testing.T) {
	media := &fakeMediaStorage{uploadURL: coverTestPublicBaseURL + "/images/link-abcdef12.jpg"}
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)

	input := validInput()
	input.Images = []string{"https://pics.example/image"}

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err, "a link image needs media and fetcher but not image storage")
	require.NotNil(t, created)
	require.Len(t, created.ImageURLs, 1)
	assert.Equal(t, media.uploadURL, created.ImageURLs[0],
		"the external URL must be replaced by the stored object URL")
	require.Len(t, media.uploadKeys, 1)
	assertImageDstKey(t, media.uploadKeys[0], created.Slug, "jpg",
		"the fetched JPEG must be stored under images/<slug>-<8 hex>.<ext>")
	assert.Equal(t, []string{"image/jpeg"}, media.uploadTypes)
	assert.Equal(t, fetchTestJPEG, media.uploadBody, "the fetched bytes must be uploaded")
	assert.Equal(t, created.ImageURLs, repo.Items[created.ID].ImageURLs,
		"the stored image URL must be persisted")
}

func TestImage_Create_CopyErrorRemovesRow(t *testing.T) {
	media := &fakeMediaStorage{
		inspectSize: 100,
		inspectHead: fetchTestJPEG,
		copyErr:     errors.New("copy failed"),
	}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)

	input := validInput()
	input.Images = []string{imageTestPendingKey("jpg")}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a copy failure must surface ErrServiceUnavailable, got %v", err)
	require.Len(t, repo.createIDs, 1, "the row must have been inserted before the copy")
	assert.Equal(t, 1, repo.deleteCalls, "the half-created row must be removed")
	assert.Empty(t, repo.Items)
}

func TestImage_Create_UploadErrorRemovesRow(t *testing.T) {
	media := &fakeMediaStorage{uploadErr: errors.New("storage is down")}
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newImageTestService(repo, media, nil, fetcher)

	input := validInput()
	input.Images = []string{"https://pics.example/image"}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a failed image upload must surface ErrServiceUnavailable, got %v", err)
	require.Len(t, repo.createIDs, 1, "the row must have been inserted before the upload")
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.Items, "no orphan row may survive a failed image upload")
	require.Len(t, media.uploadKeys, 1, "the image upload must be attempted once")
}

func TestImage_Create_SaveFailureCleansNewObjects(t *testing.T) {
	pending := imageTestPendingKey("jpg")
	media := &fakeMediaStorage{
		inspectSize: 100,
		inspectHead: fetchTestJPEG,
		copyURL:     coverTestPublicBaseURL + "/images/new-abcdef12.jpg",
		uploadURL:   coverTestPublicBaseURL + "/images/new-12345678.jpg",
	}
	fetcher := &fakeFetcher{data: fetchTestPNG, mime: "image/png"}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	repo.UpdateErr = errors.New("database is down")
	svc := newImageTestService(repo, media, nil, fetcher)

	input := validInput()
	input.Images = []string{pending, "https://pics.example/image"}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed image URL save must fail the create")
	assert.Nil(t, created)
	require.Len(t, media.copyDst, 1, "the pending object must have been copied before the save")
	require.Len(t, media.uploadKeys, 1, "the link must have been uploaded before the save")
	assert.Contains(t, media.deleteKeys, media.copyDst[0],
		"the new copy must be deleted when saving its URL fails")
	assert.Contains(t, media.deleteKeys, media.uploadKeys[0],
		"the new upload must be deleted through media storage when saving its URL fails")
	assert.NotContains(t, media.deleteKeys, pending,
		"the pending object must stay so the editor can retry")
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.Items)
}

func TestImage_Create_SaveFailureAfterLinkUploadDeletesUploadedKey(t *testing.T) {
	media := &fakeMediaStorage{uploadURL: coverTestPublicBaseURL + "/images/link-abcdef12.jpg"}
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	repo.UpdateErr = errors.New("database is down")
	svc := newImageTestService(repo, media, nil, fetcher)

	input := validInput()
	input.Images = []string{"https://pics.example/image"}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed image URL save must fail the create")
	assert.Nil(t, created)
	require.Len(t, media.uploadKeys, 1, "the link must have been uploaded before the save")
	assert.Contains(t, media.deleteKeys, media.uploadKeys[0],
		"the uploaded link object must be deleted through media storage after a save failure")
	assert.Equal(t, 1, repo.deleteCalls, "the inserted row must be removed")
	assert.Empty(t, repo.Items, "no orphan row may survive a failed image URL save")
}

func TestImage_Update_KeepCurrent(t *testing.T) {
	media := &fakeMediaStorage{}
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)
	ctx := context.Background()

	slug := "image-article"
	old1 := coverTestPublicBaseURL + "/images/" + slug + "-aaaaaaaa.jpg"
	old2 := coverTestPublicBaseURL + "/images/" + slug + "-bbbbbbbb.png"
	seedImageArticle(repo, slug, []string{old1, old2})

	input := validInput()
	input.Images = []string{old2, old1}

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, []string{old2, old1}, updated.ImageURLs, "kept images must stay in the requested order")
	assert.Equal(t, []string{old2, old1}, repo.Items[seededID(repo, slug)].ImageURLs)
	assert.Empty(t, media.inspectKeys, "keeping images must not touch storage")
	assert.Empty(t, media.copySrc)
	assert.Empty(t, media.deleteKeys, "keeping images must delete nothing")
	assert.Empty(t, media.uploadKeys)
	assert.Empty(t, fetcher.calls)
}

func TestImage_Update_NilImagesKeepsExisting(t *testing.T) {
	media := &fakeMediaStorage{}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	old := []string{coverTestPublicBaseURL + "/images/" + slug + "-aaaaaaaa.jpg"}
	seedImageArticle(repo, slug, old)

	updated, err := svc.Update(ctx, slug, validInput())

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, old, updated.ImageURLs, "an update without images must keep the existing list")
	assert.Empty(t, media.inspectKeys)
	assert.Empty(t, media.deleteKeys)
}

func TestImage_Update_KeepNeedsNothing(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)
	ctx := context.Background()

	slug := "image-article"
	old := []string{
		coverTestPublicBaseURL + "/images/" + slug + "-aaaaaaaa.jpg",
		coverTestPublicBaseURL + "/images/" + slug + "-bbbbbbbb.png",
	}
	seedImageArticle(repo, slug, old)

	input := validInput()
	input.Images = []string{old[0], old[1]}

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, old, updated.ImageURLs, "kept images need no storage at all")
}

func TestImage_Update_FullReplaceOrderAndCleanup(t *testing.T) {
	pending := imageTestPendingKey("jpg")
	media := &fakeMediaStorage{
		inspectSize: 200,
		inspectHead: fetchTestJPEG,
		copyURL:     coverTestPublicBaseURL + "/images/image-article-12345678.jpg",
		uploadURL:   coverTestPublicBaseURL + "/images/image-article-87654321.png",
	}
	fetcher := &fakeFetcher{data: fetchTestPNG, mime: "image/png"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)
	ctx := context.Background()

	slug := "image-article"
	oldKey := "images/" + slug + "-aaaaaaaa.jpg"
	droppedKey := "images/" + slug + "-bbbbbbbb.png"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	droppedURL := coverTestPublicBaseURL + "/" + droppedKey
	seedImageArticle(repo, slug, []string{droppedURL, oldURL})

	link := "https://pics.example/shot"
	input := validInput()
	input.Images = []string{pending, oldURL, link}

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, []string{media.copyURL, oldURL, media.uploadURL}, updated.ImageURLs,
		"the saved ImageURLs must follow the request order exactly: pending, kept, external")
	assert.Equal(t, updated.ImageURLs, repo.Items[seededID(repo, slug)].ImageURLs)
	require.Len(t, media.copyDst, 1)
	assertImageDstKey(t, media.copyDst[0], slug, "jpg")
	require.Len(t, media.uploadKeys, 1)
	assertImageDstKey(t, media.uploadKeys[0], slug, "png")
	assert.Contains(t, media.deleteKeys, pending, "the pending object must be deleted after the save")
	assert.Contains(t, media.deleteKeys, droppedKey, "the dropped image must be deleted after the save")
	assert.NotContains(t, media.deleteKeys, oldKey, "the kept image must survive")
}

func TestImage_Update_ClearWithEmpty(t *testing.T) {
	media := &fakeMediaStorage{}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	key1 := "images/" + slug + "-aaaaaaaa.jpg"
	key2 := "images/" + slug + "-bbbbbbbb.png"
	seedImageArticle(repo, slug, []string{
		coverTestPublicBaseURL + "/" + key1,
		coverTestPublicBaseURL + "/" + key2,
	})

	input := validInput()
	input.Images = []string{}

	updated, err := svc.Update(ctx, slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Empty(t, updated.ImageURLs, "empty Images must clear the list")
	assert.Empty(t, repo.Items[seededID(repo, slug)].ImageURLs)
	assert.ElementsMatch(t, []string{key1, key2}, media.deleteKeys,
		"clearing must delete all previously owned images")
}

func TestImage_Update_RejectsForeignStorageURL(t *testing.T) {
	slug := "image-article"
	currentURL := coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg"
	tests := []struct {
		name string
		raw  string
	}{
		{"other article image", coverTestPublicBaseURL + "/images/other-12345678.jpg"},
		{"same slug different suffix", coverTestPublicBaseURL + "/images/" + slug + "-00000000.jpg"},
		{"covers key", coverTestPublicBaseURL + "/covers/x.jpg"},
		{"videos key", coverTestPublicBaseURL + "/videos/x-12345678.mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &fakeMediaStorage{}
			fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, media, nil, fetcher)
			ctx := context.Background()

			seedImageArticle(repo, slug, []string{currentURL})

			input := validInput()
			input.Images = []string{tt.raw}

			updated, err := svc.Update(ctx, slug, input)

			require.Error(t, err)
			assert.Nil(t, updated)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Empty(t, media.inspectKeys)
			assert.Empty(t, media.deleteKeys, "a rejected URL must delete nothing")
			assert.Empty(t, media.uploadKeys)
			assert.Empty(t, fetcher.calls)
			assert.Equal(t, []string{currentURL}, repo.Items[seededID(repo, slug)].ImageURLs,
				"the stored images must stay untouched")
		})
	}
}

func TestImage_Update_RejectedBeforeStorage(t *testing.T) {
	hex32 := "0123456789abcdef0123456789abcdef"
	link := "https://pics.example/a"
	tests := []struct {
		name   string
		images func(current []string) []string
	}{
		{"eleven items", func([]string) []string { return imageTestElevenKeys() }},
		{"duplicate pending", func([]string) []string {
			return []string{imageTestPendingKey("jpg"), imageTestPendingKey("jpg")}
		}},
		{"duplicate kept", func(current []string) []string {
			return []string{current[0], current[0]}
		}},
		{"duplicate links", func([]string) []string { return []string{link, link} }},
		{"bad pending shape", func([]string) []string {
			return []string{"pending/images/" + hex32 + ".mp4"}
		}},
		{"empty string", func([]string) []string { return []string{""} }},
		{"non-url text", func([]string) []string { return []string{"not a image"} }},
		{"foreign storage url", func([]string) []string {
			return []string{coverTestPublicBaseURL + "/images/other-12345678.jpg"}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &fakeMediaStorage{inspectSize: 8, inspectHead: fetchTestJPEG}
			fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, media, nil, fetcher)
			ctx := context.Background()

			slug := "image-article"
			current := []string{coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg"}
			seedImageArticle(repo, slug, current)

			input := validInput()
			input.Images = tt.images(current)

			updated, err := svc.Update(ctx, slug, input)

			require.Error(t, err)
			assert.Nil(t, updated)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Empty(t, media.inspectKeys, "a rejected image list must not reach storage")
			assert.Empty(t, media.copySrc)
			assert.Empty(t, media.deleteKeys, "a rejected image list must delete nothing")
			assert.Empty(t, media.uploadKeys)
			assert.Empty(t, fetcher.calls)
			assert.Equal(t, current, repo.Items[seededID(repo, slug)].ImageURLs,
				"the stored images must stay untouched")
		})
	}
}

func TestImage_Update_PendingNotFoundDeletesNothing(t *testing.T) {
	media := &fakeMediaStorage{inspectErr: errs.ErrObjectNotFound}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	oldURL := coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg"
	seedImageArticle(repo, slug, []string{oldURL})
	pending := imageTestPendingKey("jpg")

	input := validInput()
	input.Images = []string{pending}

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "a missing pending object must be invalid, got %v", err)
	assert.Equal(t, []string{pending}, media.inspectKeys)
	assert.Empty(t, media.deleteKeys, "a missing object must not be deleted")
	assert.Empty(t, media.copySrc, "a missing object must not be copied")
	assert.Equal(t, []string{oldURL}, repo.Items[seededID(repo, slug)].ImageURLs,
		"the stored images must stay untouched")
}

func TestImage_Update_PendingOversizedDeletesPending(t *testing.T) {
	pending := imageTestPendingKey("jpg")
	media := &fakeMediaStorage{
		inspectSize: int64(MaxCoverBytes) + 1,
		inspectHead: fetchTestJPEG,
	}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	oldURL := coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg"
	seedImageArticle(repo, slug, []string{oldURL})

	input := validInput()
	input.Images = []string{pending}

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Equal(t, []string{pending}, media.deleteKeys,
		"an oversized pending object must be deleted right away")
	assert.Empty(t, media.copySrc)
	assert.Equal(t, []string{oldURL}, repo.Items[seededID(repo, slug)].ImageURLs,
		"the stored images must stay untouched")
}

func TestImage_Update_PendingBadSignatureDeletesPending(t *testing.T) {
	tests := []struct {
		name string
		key  string
		head []byte
	}{
		{"plain text as jpg", imageTestPendingKey("jpg"), []byte("hello world, not an image")},
		{"png as jpg", imageTestPendingKey("jpg"), fetchTestPNG},
		{"jpeg as png", imageTestPendingKey("png"), fetchTestJPEG},
		{"short input", imageTestPendingKey("jpg"), []byte{0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &fakeMediaStorage{inspectSize: int64(len(tt.head)), inspectHead: tt.head}
			repo := testsupport.NewStubRepo()
			svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
			ctx := context.Background()

			slug := "image-article"
			oldURL := coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg"
			seedImageArticle(repo, slug, []string{oldURL})

			input := validInput()
			input.Images = []string{tt.key}

			updated, err := svc.Update(ctx, slug, input)

			require.Error(t, err)
			assert.Nil(t, updated)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, []string{tt.key}, media.deleteKeys,
				"an invalid pending object must be deleted right away")
			assert.Empty(t, media.copySrc, "bad content must not be copied")
			assert.Equal(t, []string{oldURL}, repo.Items[seededID(repo, slug)].ImageURLs,
				"the stored images must stay untouched")
		})
	}
}

func TestImage_Update_PendingInspectErrorDeletesNothing(t *testing.T) {
	media := &fakeMediaStorage{inspectErr: errors.New("r2 is down")}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	oldURL := coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg"
	seedImageArticle(repo, slug, []string{oldURL})
	pending := imageTestPendingKey("jpg")

	input := validInput()
	input.Images = []string{pending}

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a storage outage must surface ErrServiceUnavailable, got %v", err)
	assert.Empty(t, media.deleteKeys, "a storage outage must delete nothing")
	assert.Equal(t, []string{oldURL}, repo.Items[seededID(repo, slug)].ImageURLs,
		"the stored images must stay untouched")
}

func TestImage_Update_SaveFailureKeepsOld(t *testing.T) {
	pending := imageTestPendingKey("jpg")
	media := &fakeMediaStorage{
		inspectSize: 200,
		inspectHead: fetchTestJPEG,
		copyURL:     coverTestPublicBaseURL + "/images/image-article-12345678.jpg",
		uploadURL:   coverTestPublicBaseURL + "/images/image-article-87654321.png",
	}
	fetcher := &fakeFetcher{data: fetchTestPNG, mime: "image/png"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)
	ctx := context.Background()

	slug := "image-article"
	oldKey := "images/" + slug + "-aaaaaaaa.jpg"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedImageArticle(repo, slug, []string{oldURL})
	repo.UpdateErr = errors.New("database is down")

	input := validInput()
	input.Images = []string{pending, "https://pics.example/shot"}

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	require.Len(t, media.copyDst, 1, "the new copy must have been written before the save")
	require.Len(t, media.uploadKeys, 1, "the new upload must have been written before the save")
	assert.Contains(t, media.deleteKeys, media.copyDst[0], "the new copy must be deleted")
	assert.Contains(t, media.deleteKeys, media.uploadKeys[0], "the new upload must be deleted")
	assert.NotContains(t, media.deleteKeys, oldKey, "the old image must survive a failed update")
	assert.NotContains(t, media.deleteKeys, pending, "the pending object must stay so the editor can retry")
	assert.Equal(t, []string{oldURL}, repo.Items[seededID(repo, slug)].ImageURLs,
		"the stored images must stay untouched")
}

func TestImage_Update_CopyErrorKeepsOld(t *testing.T) {
	pending := imageTestPendingKey("jpg")
	media := &fakeMediaStorage{
		inspectSize: 200,
		inspectHead: fetchTestJPEG,
		copyErr:     errors.New("copy failed"),
	}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	oldKey := "images/" + slug + "-abcdef12.jpg"
	oldURL := coverTestPublicBaseURL + "/" + oldKey
	seedImageArticle(repo, slug, []string{oldURL})

	input := validInput()
	input.Images = []string{pending}

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a copy failure must surface ErrServiceUnavailable, got %v", err)
	require.Len(t, media.copySrc, 1, "the pending object must have been copied before the failure")
	assert.Equal(t, pending, media.copySrc[0])
	assert.NotContains(t, media.deleteKeys, oldKey, "the old image must survive a failed copy")
	assert.NotContains(t, media.deleteKeys, pending, "a failed copy must delete nothing")
	assert.Equal(t, []string{oldURL}, repo.Items[seededID(repo, slug)].ImageURLs,
		"the stored images must stay untouched")
}

func TestImage_Update_UploadErrorKeepsOld(t *testing.T) {
	media := &fakeMediaStorage{uploadErr: errors.New("storage is down")}
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	repo := testsupport.NewStubRepo()
	svc := newImageTestService(repo, media, nil, fetcher)
	ctx := context.Background()

	slug := "image-article"
	oldURL := coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg"
	seedImageArticle(repo, slug, []string{oldURL})

	input := validInput()
	input.Images = []string{"https://pics.example/image"}

	updated, err := svc.Update(ctx, slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"an upload failure must surface ErrServiceUnavailable, got %v", err)
	require.Len(t, media.uploadKeys, 1, "the image upload must be attempted once")
	assert.Empty(t, media.deleteKeys, "a failed upload stores nothing to clean up")
	assert.Equal(t, []string{oldURL}, repo.Items[seededID(repo, slug)].ImageURLs,
		"the stored images must stay untouched")
}

func TestImage_Delete_RemovesOwnedImages(t *testing.T) {
	media := &fakeMediaStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	key1 := "images/" + slug + "-aaaaaaaa.jpg"
	key2 := "images/" + slug + "-bbbbbbbb.png"
	seeded := seedImageArticle(repo.StubRepo, slug, []string{
		coverTestPublicBaseURL + "/" + key1,
		coverTestPublicBaseURL + "/" + key2,
	})

	err := svc.Delete(ctx, seeded.Slug)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{key1, key2}, media.deleteKeys,
		"deleting the article must delete every owned image")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestImage_Delete_StorageErrorStillSucceeds(t *testing.T) {
	media := &fakeMediaStorage{deleteErr: errors.New("r2 is down")}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	key := "images/" + slug + "-abcdef12.jpg"
	seeded := seedImageArticle(repo.StubRepo, slug, []string{coverTestPublicBaseURL + "/" + key})

	err := svc.Delete(ctx, seeded.Slug)

	require.NoError(t, err, "a image cleanup failure must not fail the article deletion")
	assert.Equal(t, []string{key}, media.deleteKeys)
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestImage_Delete_SkipsNotOwned(t *testing.T) {
	media := &fakeMediaStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	slug := "image-article"
	ownedKey := "images/" + slug + "-abcdef12.jpg"
	seeded := seedImageArticle(repo.StubRepo, slug, []string{
		coverTestPublicBaseURL + "/" + ownedKey,
		"https://other.example/pic.jpg",
		coverTestPublicBaseURL + "/covers/shared.jpg",
	})

	err := svc.Delete(ctx, seeded.Slug)

	require.NoError(t, err)
	assert.Equal(t, []string{ownedKey}, media.deleteKeys,
		"only URLs under publicBaseURL + /images/ are owned")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestImage_Delete_NoImagesNoStorageCall(t *testing.T) {
	media := &fakeMediaStorage{}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newImageTestService(repo, media, &fakeImageStorage{}, nil)
	ctx := context.Background()

	seeded := seedImageArticle(repo.StubRepo, "image-article", nil)

	err := svc.Delete(ctx, seeded.Slug)

	require.NoError(t, err)
	assert.Empty(t, media.deleteKeys, "deleting an article without images must not touch storage")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, seeded.ID)
}

func TestImage_Delete_NoMediaWithOwnedImagesSucceeds(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)
	ctx := context.Background()

	slug := "image-article"
	seeded := seedImageArticle(repo, slug, []string{
		coverTestPublicBaseURL + "/images/" + slug + "-abcdef12.jpg",
	})

	var err error

	require.NotPanics(t, func() {
		err = svc.Delete(ctx, seeded.Slug)
	}, "deleting owned images without media storage must not panic")
	require.NoError(t, err)
	assert.NotContains(t, repo.Items, seeded.ID)
}
