package service

// Article media: shared direct-upload plumbing (presign, inspect, copy into
// place), the video (file or YouTube/Vimeo embed) and the image gallery.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/pkg/imagetype"
	"github.com/maxaixs/mandat/backend/pkg/videotype"
)

// Direct uploads wait under pending/<kind>/<32 hex>.<ext> until an article claims
// them; the article then owns <kind>/<slug>-<8 hex>.<ext>.
const (
	pendingIDBytes      = 16
	articleKeySuffixLen = 4
	// uploadSniffLen is how many leading bytes are read to check a pending upload.
	uploadSniffLen = 512
)

// UploadRequest describes a file the editor is about to upload directly to storage.
type UploadRequest struct {
	ContentType string
	Size        int64
}

// PresignedUpload is a direct upload: PUT the file to UploadURL, then send Key
// with the article.
type PresignedUpload struct {
	Key       string
	UploadURL string
	ExpiresAt time.Time
}

// uploadKind describes one kind of direct upload: where it waits, where the
// article keeps it, and what its bytes must look like.
type uploadKind struct {
	noun          string // used in client messages: "video", "image"
	field         string // request field named in validation errors
	typesHint     string // accepted formats for client messages
	pendingPrefix string
	articlePrefix string
	pendingKey    *regexp.Regexp // group 1 is the extension
	maxBytes      int64
	extForMIME    func(string) (string, bool)
	mimeForExt    func(string) (string, bool)
	sniff         func([]byte) string
}

// Video limits.
const (
	// MaxVideoBytes caps one uploaded article video.
	MaxVideoBytes = 500 << 20
	// VideoUploadTTL is how long a presigned direct-upload URL stays valid
	// (images use the same window).
	VideoUploadTTL = 15 * time.Minute
)

// Canonical embed player roots.
const (
	youTubeEmbedBase = "https://www.youtube-nocookie.com/embed/"
	vimeoEmbedBase   = "https://player.vimeo.com/video/"
)

var (
	youTubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	vimeoIDPattern   = regexp.MustCompile(`^[0-9]{1,12}$`)
)

// videoUploads: browsers upload to pending/videos/<32 hex>.<ext>; a saved article
// owns videos/<slug>-<8 hex>.<ext>.
var videoUploads = uploadKind{
	noun:          "video",
	field:         "video_key",
	typesHint:     "mp4 or webm",
	pendingPrefix: pendingKeyPrefix + "videos/",
	articlePrefix: "videos/",
	pendingKey:    regexp.MustCompile(`^pending/videos/[0-9a-f]{32}\.(mp4|webm)$`),
	maxBytes:      MaxVideoBytes,
	extForMIME:    videotype.ExtForMIME,
	mimeForExt:    videotype.MIMEForExt,
	sniff:         videotype.Sniff,
}

// videoChange is the validated video outcome of a create or update call.
// A non-empty pendingKey means an upload still has to be copied into place.
type videoChange struct {
	video      *models.Video
	pendingKey string
	ext        string
}

// MaxImages caps the extra images of one article.
const MaxImages = 10

// imageUploads: browsers upload to pending/images/<32 hex>.<ext>; a saved article
// owns images/<slug>-<8 hex>.<ext>. Images follow the cover size cap.
var imageUploads = uploadKind{
	noun:          "image",
	field:         "images",
	typesHint:     "jpg, png, webp or gif",
	pendingPrefix: pendingKeyPrefix + "images/",
	articlePrefix: "images/",
	pendingKey:    regexp.MustCompile(`^pending/images/[0-9a-f]{32}\.(jpg|png|webp|gif)$`),
	maxBytes:      MaxCoverBytes,
	extForMIME:    imagetype.ExtForMIME,
	mimeForExt:    imagetype.MIMEForExt,
	sniff:         imagetype.Sniff,
}

// imageItem is one validated entry of the requested image list. Exactly one
// source is set: a kept URL, a checked pending upload, or a downloaded link.
type imageItem struct {
	keepURL    string
	pendingKey string
	ext        string
	link       string
	image      *coverImage
}

// PresignVideoUpload validates the announced video file and returns a direct-upload URL.
func (s *NewsService) PresignVideoUpload(ctx context.Context, req UploadRequest) (*PresignedUpload, error) {
	const fc = "newsService_PresignVideoUpload"

	upload, err := s.presignUpload(ctx, videoUploads, req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return upload, nil
}

// PresignImageUpload validates the announced image file and returns a direct-upload URL.
func (s *NewsService) PresignImageUpload(ctx context.Context, req UploadRequest) (*PresignedUpload, error) {
	const fc = "newsService_PresignImageUpload"

	upload, err := s.presignUpload(ctx, imageUploads, req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return upload, nil
}

// prepareVideo validates the requested video before any write. current is the
// article's stored video (nil on create); its file URL keeps the file in place.
func (s *NewsService) prepareVideo(ctx context.Context, input NewsInput, current *models.Video) (videoChange, error) {
	const fc = "newsService_prepareVideo"

	if key := strings.TrimSpace(input.VideoKey); key != "" {
		ext, err := s.inspectPending(ctx, videoUploads, key)
		if err != nil {
			return videoChange{}, fmt.Errorf("%s: %w", fc, err)
		}

		return videoChange{pendingKey: key, ext: ext}, nil
	}

	raw := strings.TrimSpace(input.VideoURL)
	switch {
	case raw == "":
		return videoChange{}, nil
	case current != nil && current.Kind == models.VideoKindFile && raw == current.URL:
		return videoChange{video: current}, nil
	case s.isStorageURL(raw):
		reason := "only the article's current video file can be kept"

		return videoChange{}, fmt.Errorf("%s: %w", fc, errs.NewValidationError("video_url", reason))
	}

	embedURL, ok := normalizeEmbedURL(raw)
	if !ok {
		reason := "video link must be a YouTube or Vimeo video"

		return videoChange{}, fmt.Errorf("%s: %w", fc, errs.NewValidationError("video_url", reason))
	}

	return videoChange{video: &models.Video{Kind: models.VideoKindEmbed, URL: embedURL}}, nil
}

// prepareImages validates the requested image list before any write. current is
// the article's stored list (nil on create). A nil request list means "keep the
// current images"; an empty one clears them.
func (s *NewsService) prepareImages(ctx context.Context, requested []string, current []string) ([]imageItem, error) {
	const fc = "newsService_prepareImages"

	if requested == nil {
		items := make([]imageItem, 0, len(current))
		for _, imageURL := range current {
			items = append(items, imageItem{keepURL: imageURL})
		}

		return items, nil
	}

	items, err := s.classifyImages(requested, current)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	for i := range items {
		switch {
		case items[i].pendingKey != "":
			items[i].ext, err = s.inspectPending(ctx, imageUploads, items[i].pendingKey)
		case items[i].link != "":
			items[i].image, err = s.fetchImage(ctx, items[i].link)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fc, err)
		}
	}

	return items, nil
}

// classifyImages sorts every requested entry into its source without touching
// storage, so malformed lists are rejected before any I/O.
func (s *NewsService) classifyImages(requested []string, current []string) ([]imageItem, error) {
	const fc = "newsService_classifyImages"

	if len(requested) > MaxImages {
		reason := fmt.Sprintf("at most %d images are allowed", MaxImages)

		return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("images", reason))
	}

	items := make([]imageItem, 0, len(requested))
	seen := make(map[string]bool, len(requested))
	needsMedia, needsFetch := false, false
	for _, entry := range requested {
		entry = strings.TrimSpace(entry)
		if seen[entry] {
			return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("images", "the same image is listed twice"))
		}
		seen[entry] = true

		switch {
		case imageUploads.pendingKey.MatchString(entry):
			items = append(items, imageItem{pendingKey: entry})
			needsMedia = true
		case entry != "" && slices.Contains(current, entry):
			items = append(items, imageItem{keepURL: entry})
		case s.isStorageURL(entry):
			reason := "only the article's own images can be kept"

			return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("images", reason))
		case isHTTPURL(entry):
			items = append(items, imageItem{link: entry})
			needsMedia, needsFetch = true, true
		default:
			reason := "each image must be an uploaded file or an image link"

			return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("images", reason))
		}
	}

	if needsMedia && s.media == nil {
		return nil, fmt.Errorf("%s: %w: media storage is not configured", fc, errs.ErrServiceUnavailable)
	}
	if needsFetch && s.fetcher == nil {
		return nil, fmt.Errorf("%s: %w: image links cannot be downloaded", fc, errs.ErrServiceUnavailable)
	}

	return items, nil
}

// fetchImage downloads a image link and checks it is a supported image.
func (s *NewsService) fetchImage(ctx context.Context, link string) (*coverImage, error) {
	const fc = "newsService_fetchImage"

	data, mime, err := s.fetcher.Fetch(ctx, link)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if len(data) > MaxCoverBytes {
		return nil, fmt.Errorf("%s: %w: image exceeds %d bytes", fc, errs.ErrPayloadTooLarge, MaxCoverBytes)
	}

	ext, ok := imagetype.ExtForMIME(mime)
	if !ok {
		return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("images", "unsupported image type "+mime))
	}

	return &coverImage{data: data, ext: ext, contentType: mime}, nil
}

// storeImages puts every new image in place and returns the article's image URLs
// in request order plus the keys of the objects it created.
func (s *NewsService) storeImages(ctx context.Context, slug string, items []imageItem) ([]string, []string, error) {
	const fc = "newsService_storeImages"

	urls := make([]string, 0, len(items))
	var created []string
	for _, item := range items {
		switch {
		case item.keepURL != "":
			urls = append(urls, item.keepURL)
		case item.pendingKey != "":
			key, imageURL, err := s.copyPending(ctx, imageUploads, slug, item.pendingKey, item.ext)
			if err != nil {
				return nil, created, fmt.Errorf("%s: %w", fc, err)
			}
			created = append(created, key)
			urls = append(urls, imageURL)
		case item.image != nil:
			key := newArticleKey(imageUploads.articlePrefix, slug, item.image.ext)
			imageURL, err := s.media.Upload(ctx, key, bytes.NewReader(item.image.data), item.image.contentType)
			if err != nil {
				if !errors.Is(err, errs.ErrInvalidInput) && !errors.Is(err, errs.ErrPayloadTooLarge) {
					err = fmt.Errorf("%w: %w", errs.ErrServiceUnavailable, err)
				}

				return nil, created, fmt.Errorf("%s: upload %s: %w", fc, key, err)
			}
			created = append(created, key)
			urls = append(urls, imageURL)
		}
	}

	return urls, created, nil
}

// ownedVideoKey extracts the storage key of an article's own video file.
func (s *NewsService) ownedVideoKey(video *models.Video) (string, bool) {
	if video == nil || video.Kind != models.VideoKindFile {
		return "", false
	}

	return s.ownedMediaKey(video.URL, videoUploads.articlePrefix)
}

// ownedImageKeys lists the storage keys of the images an article owns.
func (s *NewsService) ownedImageKeys(imageURLs []string) []string {
	var keys []string
	for _, imageURL := range imageURLs {
		if key, owned := s.ownedMediaKey(imageURL, imageUploads.articlePrefix); owned {
			keys = append(keys, key)
		}
	}

	return keys
}

// presignUpload validates the announced file of kind and returns a direct-upload URL.
func (s *NewsService) presignUpload(ctx context.Context, kind uploadKind, req UploadRequest) (*PresignedUpload, error) {
	const fc = "newsService_presignUpload"

	ext, ok := kind.extForMIME(req.ContentType)
	if !ok {
		reason := kind.noun + " must be " + kind.typesHint

		return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("content_type", reason))
	}
	if req.Size <= 0 || req.Size > kind.maxBytes {
		reason := fmt.Sprintf("%s size must be between 1 byte and %d MB", kind.noun, kind.maxBytes>>20)

		return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("size", reason))
	}
	if s.media == nil {
		return nil, fmt.Errorf("%s: %w: media storage is not configured", fc, errs.ErrServiceUnavailable)
	}

	key := newPendingKey(kind.pendingPrefix, ext)
	expiresAt := time.Now().UTC().Add(VideoUploadTTL)
	uploadURL, err := s.media.PresignPut(ctx, key, req.ContentType, req.Size, VideoUploadTTL)
	if err != nil {
		err = fmt.Errorf("%s: %w: %w", fc, errs.ErrServiceUnavailable, err)
		s.logFailure(ctx, "failed to presign news "+kind.noun+" upload", fc, err)

		return nil, err
	}

	return &PresignedUpload{Key: key, UploadURL: uploadURL, ExpiresAt: expiresAt}, nil
}

// inspectPending checks that a pending upload of kind exists, fits the size cap and
// really is what its key extension claims; the client's MIME is not trusted.
// It returns the extension. A rejected existing upload is deleted right away.
func (s *NewsService) inspectPending(ctx context.Context, kind uploadKind, key string) (string, error) {
	const fc = "newsService_inspectPending"

	match := kind.pendingKey.FindStringSubmatch(key)
	if match == nil {
		return "", fmt.Errorf("%s: %w", fc, errs.NewValidationError(kind.field, "unknown upload key"))
	}
	if s.media == nil {
		return "", fmt.Errorf("%s: %w: media storage is not configured", fc, errs.ErrServiceUnavailable)
	}

	size, head, err := s.media.Inspect(ctx, key, uploadSniffLen)
	switch {
	case errors.Is(err, errs.ErrObjectNotFound):
		reason := "uploaded " + kind.noun + " not found"

		return "", fmt.Errorf("%s: %w", fc, errs.NewValidationError(kind.field, reason))
	case err != nil:
		return "", fmt.Errorf("%s: %w: %w", fc, errs.ErrServiceUnavailable, err)
	}

	ext := match[1]
	want, _ := kind.mimeForExt(ext)
	var reason string
	switch {
	case size > kind.maxBytes:
		reason = fmt.Sprintf("%s exceeds %d MB", kind.noun, kind.maxBytes>>20)
	case kind.sniff(head) != want:
		reason = "uploaded file is not a valid " + ext + " " + kind.noun
	}
	if reason == "" {
		return ext, nil
	}

	// The key is a verified pending upload, so the rejected file is safe to drop now.
	if err := s.media.Delete(ctx, key); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete rejected news upload", "fc", fc, "key", key, "error", err)
	}

	return "", fmt.Errorf("%s: %w", fc, errs.NewValidationError(kind.field, reason))
}

// copyPending copies a checked pending upload under the article's own key and
// returns that key and its public URL.
func (s *NewsService) copyPending(
	ctx context.Context,
	kind uploadKind,
	slug string,
	pendingKey string,
	ext string,
) (string, string, error) {
	const fc = "newsService_copyPending"

	key := newArticleKey(kind.articlePrefix, slug, ext)
	objectURL, err := s.media.Copy(ctx, pendingKey, key)
	switch {
	case errors.Is(err, errs.ErrObjectNotFound):
		reason := "uploaded " + kind.noun + " not found"

		return "", "", fmt.Errorf("%s: %w", fc, errs.NewValidationError(kind.field, reason))
	case err != nil:
		return "", "", fmt.Errorf("%s: copy %s: %w: %w", fc, pendingKey, errs.ErrServiceUnavailable, err)
	}

	return key, objectURL, nil
}

// isStorageURL reports links pointing into our public storage.
func (s *NewsService) isStorageURL(raw string) bool {
	return s.coverBaseURL != "" && strings.HasPrefix(raw, strings.TrimRight(s.coverBaseURL, "/")+"/")
}

// ownedMediaKey extracts the storage key of an object the article owns under prefix.
// Unsafe key shapes never qualify, and without media storage nothing is owned.
func (s *NewsService) ownedMediaKey(objectURL string, prefix string) (string, bool) {
	if s.media == nil || !s.isStorageURL(objectURL) {
		return "", false
	}

	key := strings.TrimPrefix(objectURL, strings.TrimRight(s.coverBaseURL, "/")+"/")
	if !strings.HasPrefix(key, prefix) || strings.HasPrefix(key, "/") || strings.Contains(key, "..") {
		return "", false
	}

	return key, true
}

// normalizeEmbedURL maps a YouTube or Vimeo link onto its canonical player URL.
func normalizeEmbedURL(raw string) (string, bool) {
	link, err := url.Parse(raw)
	if err != nil || (link.Scheme != "https" && link.Scheme != "http") {
		return "", false
	}

	host := strings.ToLower(link.Hostname())
	host = strings.TrimPrefix(strings.TrimPrefix(host, "www."), "m.")
	path := strings.TrimSuffix(link.Path, "/")

	var id string
	switch host {
	case "youtube.com", "youtube-nocookie.com":
		switch {
		case path == "/watch":
			id = link.Query().Get("v")
		case strings.HasPrefix(path, "/shorts/"):
			id = strings.TrimPrefix(path, "/shorts/")
		case strings.HasPrefix(path, "/embed/"):
			id = strings.TrimPrefix(path, "/embed/")
		}
	case "youtu.be":
		id = strings.TrimPrefix(path, "/")
	case "vimeo.com":
		id = strings.TrimPrefix(path, "/")
		if !vimeoIDPattern.MatchString(id) {
			return "", false
		}

		return vimeoEmbedBase + id, true
	case "player.vimeo.com":
		id = strings.TrimPrefix(path, "/video/")
		if !vimeoIDPattern.MatchString(id) {
			return "", false
		}

		return vimeoEmbedBase + id, true
	default:
		return "", false
	}
	if !youTubeIDPattern.MatchString(id) {
		return "", false
	}

	return youTubeEmbedBase + id, true
}

// isHTTPURL reports an absolute http(s) URL with a host.
func isHTTPURL(raw string) bool {
	link, err := url.Parse(raw)

	return err == nil && (link.Scheme == "http" || link.Scheme == "https") && link.Host != ""
}

// newArticleKey names an article-owned object under prefix: unique per upload.
func newArticleKey(prefix string, slug string, ext string) string {
	suffix := make([]byte, articleKeySuffixLen)
	_, _ = rand.Read(suffix) // crypto/rand.Read never fails; it aborts the program instead.

	return prefix + slug + "-" + hex.EncodeToString(suffix) + "." + ext
}

// newPendingKey names a fresh direct-upload object under prefix.
func newPendingKey(prefix string, ext string) string {
	id := make([]byte, pendingIDBytes)
	_, _ = rand.Read(id) // crypto/rand.Read never fails; it aborts the program instead.

	return prefix + hex.EncodeToString(id) + "." + ext
}
