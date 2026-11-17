package r2

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAPI is a scriptable stand-in for the S3 client used by Client.
type fakeAPI struct {
	putCalls   int
	delCalls   int
	lastPut    *s3.PutObjectInput
	lastDelete *s3.DeleteObjectInput
	putErr     error
	delErr     error
}

func (f *fakeAPI) PutObject(
	ctx context.Context,
	in *s3.PutObjectInput,
	optFns ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
	f.putCalls++
	f.lastPut = in

	return &s3.PutObjectOutput{}, f.putErr
}

func (f *fakeAPI) DeleteObject(
	ctx context.Context,
	in *s3.DeleteObjectInput,
	optFns ...func(*s3.Options),
) (*s3.DeleteObjectOutput, error) {
	f.delCalls++
	f.lastDelete = in

	return &s3.DeleteObjectOutput{}, f.delErr
}

// zeroReader yields an unbounded stream of zero bytes without allocating.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)

	return len(p), nil
}

// jpegBytes returns a byte slice that net/http.DetectContentType reports as image/jpeg.
func jpegBytes() []byte {
	return append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0}, 600)...)
}

// newTestClient builds a Client backed by the given fake API.
func newTestClient(t *testing.T, cfg Config, api *fakeAPI) *Client {
	t.Helper()

	client, err := NewClient(cfg, api)
	require.NoError(t, err, "NewClient must accept an injected API client")

	return client
}

func TestClient_Upload_RejectsSpoofedMIME(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)
	body := []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xFF\xFF")

	url, err := client.Upload(context.Background(), "covers/evil.jpg", bytes.NewReader(body), "image/jpeg")

	require.Error(t, err, "an executable body must not be accepted as image/jpeg")
	assert.Empty(t, url, "no public URL may be returned for a rejected upload")
	assert.Equal(t, 0, api.putCalls, "a rejected upload must never reach object storage")
}

func TestClient_Upload_RejectsOversized(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	// Valid JPEG prefix followed by enough zeros to exceed MaxUploadBytes.
	body := io.MultiReader(
		bytes.NewReader(jpegBytes()),
		io.LimitReader(zeroReader{}, int64(MaxUploadBytes)),
	)

	url, err := client.Upload(context.Background(), "covers/huge.jpg", body, "image/jpeg")

	require.Error(t, err, "a body larger than MaxUploadBytes must be rejected")
	assert.Empty(t, url, "no public URL may be returned for a rejected upload")
	assert.Equal(t, 0, api.putCalls, "an oversized upload must never reach object storage")
}

func TestClient_Upload_ReturnsPublicURLWithoutSecrets(t *testing.T) {
	api := &fakeAPI{}
	cfg := validConfig()
	cfg.PublicBaseURL = "https://pub.example.com/x"
	cfg.AccessKeyID = "AKID"
	cfg.SecretAccessKey = "SECRET"
	client := newTestClient(t, cfg, api)

	url, err := client.Upload(context.Background(), "covers/test.jpg", bytes.NewReader(jpegBytes()), "image/jpeg")

	require.NoError(t, err)
	assert.Equal(t, "https://pub.example.com/x/covers/test.jpg", url)
	assert.NotContains(t, url, "AKID", "the public URL must not leak the access key")
	assert.NotContains(t, url, "SECRET", "the public URL must not leak the secret key")

	require.Equal(t, 1, api.putCalls, "a successful upload must store the object exactly once")
	require.NotNil(t, api.lastPut)
	require.NotNil(t, api.lastPut.Bucket, "the upload must target a bucket")
	assert.Equal(t, cfg.Bucket, *api.lastPut.Bucket)
	require.NotNil(t, api.lastPut.Key, "the upload must use the requested key")
	assert.Equal(t, "covers/test.jpg", *api.lastPut.Key)
}

func TestClient_Delete_MissingKey_ReturnsNil(t *testing.T) {
	api := &fakeAPI{
		delErr: &smithy.GenericAPIError{Code: "NoSuchKey", Message: "missing"},
	}
	client := newTestClient(t, validConfig(), api)

	err := client.Delete(context.Background(), "covers/missing.jpg")

	require.NoError(t, err, "deleting an already absent object must be idempotent")
	assert.Equal(t, 1, api.delCalls, "Delete must issue exactly one DeleteObject call")
}

func TestClient_Upload_EmptyKeyFails(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	url, err := client.Upload(context.Background(), "", bytes.NewReader(jpegBytes()), "image/jpeg")

	require.Error(t, err, "an empty object key must be rejected")
	assert.Empty(t, url)
	assert.Equal(t, 0, api.putCalls, "an invalid key must never reach object storage")
}

// The declared content type is not trusted: sniffing decides, so a generic
// client type must not block an otherwise valid image.
func TestClient_Upload_IgnoresDeclaredType(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	url, err := client.Upload(
		context.Background(),
		"covers/photo.jpg",
		bytes.NewReader(jpegBytes()),
		"application/octet-stream",
	)

	require.NoError(t, err, "the declared content type must be ignored")
	assert.NotEmpty(t, url)
	require.Equal(t, 1, api.putCalls, "a valid image must still be stored once")
	require.NotNil(t, api.lastPut)
	require.NotNil(t, api.lastPut.ContentType)
	assert.Equal(t, "image/jpeg", *api.lastPut.ContentType,
		"the stored content type must come from sniffing, not the caller")
}

func TestClient_Upload_RejectsExtMIMEMismatch(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	url, err := client.Upload(
		context.Background(),
		"covers/photo.png",
		bytes.NewReader(jpegBytes()),
		"image/png",
	)

	require.Error(t, err, "a JPEG body under a .png key must be rejected")
	assert.Empty(t, url)
	assert.Equal(t, 0, api.putCalls, "a mismatched extension must never reach object storage")
}

func TestClient_Upload_RejectsEmptyBody(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	url, err := client.Upload(context.Background(), "covers/empty.jpg", bytes.NewReader(nil), "image/jpeg")

	require.Error(t, err, "an empty body is not an image")
	assert.Empty(t, url)
	assert.Equal(t, 0, api.putCalls, "an empty upload must never reach object storage")
}

func TestClient_Upload_RejectsHTML(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	body := []byte("<!DOCTYPE html><html><body>not an image</body></html>")

	url, err := client.Upload(context.Background(), "covers/page.png", bytes.NewReader(body), "image/png")

	require.Error(t, err, "HTML claimed as image/png must be rejected")
	assert.Empty(t, url)
	assert.Equal(t, 0, api.putCalls, "non-image content must never reach object storage")
}

func TestClient_Delete_PropagatesRealErrors(t *testing.T) {
	api := &fakeAPI{
		delErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
	}
	client := newTestClient(t, validConfig(), api)

	err := client.Delete(context.Background(), "covers/denied.jpg")

	require.Error(t, err, "a real storage failure must not be swallowed")
	assert.Equal(t, 1, api.delCalls, "Delete must issue exactly one DeleteObject call")
}
