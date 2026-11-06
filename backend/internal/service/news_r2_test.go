package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestService_Create_UploadsCoverByFinalSlug(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: "https://cdn.example.com/covers/uploaded.jpg"}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)

	input := validInput()
	baseSlug := slugify(input.Title)
	repo.Conflicts[baseSlug] = true
	input.Cover = &CoverUpload{
		Body:        strings.NewReader("bytes"),
		Filename:    "Cat.JPG",
		ContentType: "image/jpeg",
		Size:        int64(len("bytes")),
	}

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, baseSlug+"-2", created.Slug)
	assert.Equal(t, 2, repo.CreateCalls, "the slug conflict must be retried once")
	require.Len(t, storage.uploadKeys, 1)
	assert.Equal(t, "covers/"+baseSlug+"-2.jpg", storage.uploadKeys[0],
		"the cover key must use the final slug and a lowercased extension")
	assert.Equal(t, "image/jpeg", storage.uploadTypes[0])
	assert.Equal(t, []byte("bytes"), storage.uploadBody, "the upload must receive the cover body")
	assert.Equal(t, storage.uploadURL, created.ImageUrl)
	assert.Equal(t, storage.uploadURL, repo.Items[created.ID].ImageUrl,
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
	seed.ImageUrl = coverTestPublicBaseURL + "/covers/old-news.png"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)
	require.Equal(t, "old-news", existing.Slug)
	require.Empty(t, storage.uploadKeys, "creating without a cover must not upload")

	update := seed
	update.Cover = &CoverUpload{
		Body:        strings.NewReader("webp-bytes"),
		Filename:    "new.webp",
		ContentType: "image/webp",
		Size:        int64(len("webp-bytes")),
	}

	updated, err := svc.Update(ctx, existing.Slug, update)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, storage.uploadKeys, 1)
	assert.Equal(t, "covers/old-news.webp", storage.uploadKeys[0])
	assert.Equal(t, storage.uploadURL, updated.ImageUrl)
	assert.Equal(t, storage.uploadURL, repo.Items[updated.ID].ImageUrl,
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
	seed.ImageUrl = coverTestPublicBaseURL + "/covers/pic.jpg"
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
	seed.ImageUrl = "https://other.example/pic.jpg"
	created, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	err = svc.Delete(ctx, created.Slug)

	require.NoError(t, err)
	assert.Empty(t, storage.deleteKeys,
		"a cover outside our bucket must not be deleted")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, created.ID)
}

func TestService_Create_UploadFailureKeepsArticle(t *testing.T) {
	storage := &fakeImageStorage{uploadErr: errors.New("storage is down")}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader("bytes"),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len("bytes")),
	}

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err, "a failed cover upload must not fail the create")
	require.NotNil(t, created, "the article must be returned even without its cover")
	require.Len(t, repo.createIDs, 1)
	assert.Equal(t, repo.createIDs[0], created.ID,
		"the returned article must be the row that was just created")
	assert.Empty(t, created.ImageUrl, "the article must be returned without a cover URL")
	assert.Zero(t, repo.deleteCalls, "a failed cover upload must not delete the created row")
	assert.Contains(t, repo.Items, created.ID, "the coverless article must survive")
	require.Len(t, storage.uploadKeys, 1, "the cover upload must be attempted once")
}

func TestService_Create_URLSaveFailureKeepsArticle(t *testing.T) {
	const preUploadURL = "https://pics.example/legacy.jpg"

	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/saved-url.jpg"}
	repo := &deleteCountingRepo{StubRepo: testsupport.NewStubRepo()}
	svc := newCoverTestService(repo, storage)

	input := validInput()
	input.ImageUrl = preUploadURL
	input.Cover = &CoverUpload{
		Body:        strings.NewReader("bytes"),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len("bytes")),
	}

	repo.UpdateErr = errors.New("database is down")

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err, "a failed cover URL save must not fail the create")
	require.NotNil(t, created)
	require.Len(t, repo.createIDs, 1)
	assert.Equal(t, repo.createIDs[0], created.ID,
		"the returned article must be the row that was just created")
	require.Len(t, storage.uploadKeys, 1)
	uploadedKey := storage.uploadKeys[0]
	assert.Equal(t, "covers/"+slugify(input.Title)+".jpg", uploadedKey)
	assert.Equal(t, []string{uploadedKey}, storage.deleteKeys,
		"the object uploaded for the failed URL save must be deleted")
	assert.Equal(t, preUploadURL, created.ImageUrl,
		"the returned article must get its pre-upload image URL back")
	assert.Zero(t, repo.deleteCalls, "a failed cover URL save must not delete the created row")
	assert.Contains(t, repo.Items, created.ID, "the article must survive the failed URL save")
	assert.Equal(t, preUploadURL, repo.Items[created.ID].ImageUrl,
		"the stored article must keep its pre-upload image URL")
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
		Body:        strings.NewReader("bytes"),
		Filename:    "cover.png",
		ContentType: "image/png",
		Size:        int64(len("bytes")),
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
	seed.ImageUrl = coverTestPublicBaseURL + "/covers/cleanup-article.webp"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	repo.UpdateErr = errors.New("database is down")

	input := seed
	input.Cover = &CoverUpload{
		Body:        strings.NewReader("png-bytes"),
		Filename:    "new.png",
		ContentType: "image/png",
		Size:        int64(len("png-bytes")),
	}

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	require.Equal(t, []string{"covers/cleanup-article.png"}, storage.deleteKeys,
		"an object uploaded for a failed update must be cleaned up")
	assert.Equal(t, existing.ImageUrl, repo.Items[existing.ID].ImageUrl,
		"the stored article must keep its previous cover after a failed update")
}

func TestService_Update_PreservesImageUrlWithoutCover(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := testsupport.NewStubRepo()
	svc := newCoverTestService(repo, storage)
	ctx := context.Background()

	seed := validInput()
	seed.Title = "legacy-cover"
	seed.ImageUrl = coverTestPublicBaseURL + "/covers/legacy-cover.png"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	input := validInput()
	input.Title = "legacy-cover"
	input.ImageUrl = ""

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, existing.ImageUrl, updated.ImageUrl,
		"an update without a new cover must keep the existing image URL")
	assert.Empty(t, storage.uploadKeys, "no cover upload may happen without a cover")
	assert.Empty(t, storage.deleteKeys, "an untouched cover must not be deleted")
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

func TestService_Create_FetchesExternalURLToR2(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/fetched.jpg"}
	repo := testsupport.NewStubRepo()
	fetcher := &fakeFetcher{data: fetchTestJPEG, mime: "image/jpeg"}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	input := validInput()
	input.ImageUrl = "https://pics.example/photo"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Len(t, fetcher.calls, 1, "the external cover must be fetched exactly once")
	assert.Equal(t, "https://pics.example/photo", fetcher.calls[0])
	require.Len(t, storage.uploadKeys, 1)
	assert.Equal(t, "covers/"+slugify(input.Title)+".jpg", storage.uploadKeys[0],
		"the fetched JPEG must be stored under the slug-derived key")
	assert.Equal(t, fetchTestJPEG, storage.uploadBody, "the fetched bytes must be uploaded")
	assert.Equal(t, storage.uploadURL, created.ImageUrl,
		"the external URL must be replaced by the stored object URL")
	assert.Equal(t, storage.uploadURL, repo.Items[created.ID].ImageUrl,
		"the stored cover URL must be persisted on the article")
}

func TestService_Create_FetchFailureWritesNothing(t *testing.T) {
	storage := &fakeImageStorage{}
	repo := testsupport.NewStubRepo()
	fetcher := &fakeFetcher{err: errors.New("fetch failed")}
	svc := newCoverTestServiceWithFetcher(repo, storage, fetcher)

	input := validInput()
	input.ImageUrl = "https://pics.example/photo"

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
	input.ImageUrl = ownedURL

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Empty(t, fetcher.calls, "a URL under our own bucket must not be refetched")
	assert.Empty(t, storage.uploadKeys, "no upload may happen without a new cover")
	assert.Equal(t, ownedURL, created.ImageUrl, "the owned URL must be kept as-is")
}

func TestService_Create_LegacyHotlinkWithoutFetcher(t *testing.T) {
	svc, repo := newTestService()

	input := validInput()
	input.ImageUrl = "https://pics.example/photo"

	created, err := svc.Create(context.Background(), input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "https://pics.example/photo", created.ImageUrl,
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
	seed.ImageUrl = coverTestPublicBaseURL + "/covers/old.jpg"
	existing, err := svc.Create(ctx, seed)
	require.NoError(t, err)
	require.Empty(t, storage.uploadKeys)

	fetcher := &fakeFetcher{data: fetchTestPNG, mime: "image/png"}
	svc = newCoverTestServiceWithFetcher(repo, storage, fetcher)

	update := seed
	update.ImageUrl = "https://pics.example/new-cover"
	updated, err := svc.Update(ctx, existing.Slug, update)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, fetcher.calls, 1)
	assert.Equal(t, "https://pics.example/new-cover", fetcher.calls[0])
	require.Len(t, storage.uploadKeys, 1)
	assert.Equal(t, "covers/old-news.png", storage.uploadKeys[0],
		"the pasted PNG must be stored under the article slug")
	assert.Equal(t, storage.uploadURL, updated.ImageUrl,
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
	input.ImageUrl = "https://pics.example/photo"

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
	seed.ImageUrl = coverTestPublicBaseURL + "/static/logo.png"
	created, err := svc.Create(ctx, seed)
	require.NoError(t, err)

	err = svc.Delete(ctx, created.Slug)

	require.NoError(t, err)
	assert.Empty(t, storage.deleteKeys,
		"a shared asset on our host must not be deleted")
	assert.Equal(t, 1, repo.deleteCalls)
	assert.NotContains(t, repo.Items, created.ID)
}
