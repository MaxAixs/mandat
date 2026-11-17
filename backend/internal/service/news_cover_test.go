package service

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
