package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	return NewNewsService(repo, logger), repo
}

func validInput() NewsInput {
	return NewsInput{
		Category: string(models.CategoryNews),
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
		{"unknown category", func(in *NewsInput) { in.Category = "Політика" }},
		{"unknown status", func(in *NewsInput) { in.Status = "archived" }},
		{"published without text", func(in *NewsInput) { in.Status = string(models.StatusPublished); in.Text = "" }},
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
	input.Status = string(models.StatusPublished)
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

func TestCreate_PublishSetsPublishedAt(t *testing.T) {
	svc, _ := newTestService()

	input := validInput()
	input.Status = string(models.StatusPublished)

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
	previousInput.Status = string(models.StatusPublished)
	previousInput.MainEvent = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	replacementInput := validInput()
	replacementInput.Title = "Друга головна подія"
	replacementInput.Status = string(models.StatusPublished)
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
	publishedInput.Status = string(models.StatusPublished)
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

func TestCreate_PublishedFeaturedReplacesPrevious(t *testing.T) {
	svc, repo := newTestService()

	previousInput := validInput()
	previousInput.Title = "Перша стаття"
	previousInput.Status = string(models.StatusPublished)
	previousInput.Featured = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	replacementInput := validInput()
	replacementInput.Title = "Друга стаття"
	replacementInput.Status = string(models.StatusPublished)
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
	publishedInput.Status = string(models.StatusPublished)
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

	_, err := svc.List(context.Background(), 0, 10, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput))

	_, err = svc.List(context.Background(), 1, 0, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput))
}

func TestList_ClampsLimitToMax(t *testing.T) {
	svc, repo := newTestService()

	result, err := svc.List(context.Background(), 1, 100000, "")
	require.NoError(t, err)
	assert.Equal(t, MaxPageLimit, result.Limit)
	assert.Equal(t, MaxPageLimit, repo.LastFilter.Limit)
}

func TestList_RejectsUnknownStatus(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.List(context.Background(), 1, 10, "archived")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput))
}

func TestList_EmptyStatusMeansNoFilter(t *testing.T) {
	svc, repo := newTestService()

	_, err := svc.List(context.Background(), 1, 10, "")
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
	input.Status = string(models.StatusPublished)
	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, created.PublishedAt)
	publishedAt := *created.PublishedAt

	update := validInput()
	update.Status = string(models.StatusDraft)
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
	input.Status = string(models.StatusPublished)
	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	publishedAt := *created.PublishedAt

	update := validInput()
	update.Status = string(models.StatusPublished)
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
	update.Status = string(models.StatusPublished)

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
	previousInput.Status = string(models.StatusPublished)
	previousInput.MainEvent = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	targetInput := validInput()
	targetInput.Title = "Друга головна подія"
	targetInput.Status = string(models.StatusPublished)
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
	publishedInput.Status = string(models.StatusPublished)
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
	previousInput.Status = string(models.StatusPublished)
	previousInput.Featured = true
	_, err := svc.Create(context.Background(), previousInput)
	require.NoError(t, err)

	targetInput := validInput()
	targetInput.Title = "Друга стаття"
	targetInput.Status = string(models.StatusPublished)
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
	update.Category = string(models.CategoryAnalytics)
	update.Description = "Новий опис"
	update.Featured = true
	update.ImageUrl = "https://example.com/cover.jpg"

	updated, err := svc.Update(context.Background(), created.Slug, update)
	require.NoError(t, err)
	assert.Equal(t, "Оновлений заголовок", updated.Title)
	assert.Equal(t, models.CategoryAnalytics, updated.Category)
	assert.Equal(t, "Новий опис", updated.Description)
	assert.True(t, updated.Featured)
	assert.Equal(t, "https://example.com/cover.jpg", updated.ImageUrl)
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
	svc := NewNewsService(repo, slog.New(handler))

	_, err := svc.Create(context.Background(), validInput())
	require.Error(t, err)
	assert.True(t, handler.hasFC("newsService_Create"), "expected a log record carrying the fc identifier")
}
