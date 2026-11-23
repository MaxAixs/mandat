package r2

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePresigner is a scriptable stand-in for the S3 presign client.
type fakePresigner struct {
	calls       int
	lastIn      *s3.PutObjectInput
	lastExpires time.Duration
	url         string
	err         error
}

func (f *fakePresigner) PresignPutObject(
	ctx context.Context,
	in *s3.PutObjectInput,
	optFns ...func(*s3.PresignOptions),
) (*v4.PresignedHTTPRequest, error) {
	f.calls++
	f.lastIn = in

	var opts s3.PresignOptions
	for _, fn := range optFns {
		fn(&opts)
	}
	f.lastExpires = opts.Expires

	if f.err != nil {
		return nil, f.err
	}

	return &v4.PresignedHTTPRequest{URL: f.url}, nil
}

func newPresignTestClient(t *testing.T, api *fakeAPI, ps *fakePresigner) *Client {
	t.Helper()

	client, err := NewClient(validConfig(), api, WithPresigner(ps))
	require.NoError(t, err, "NewClient must accept a presigner option")

	return client
}

func TestClient_PresignPut_SignsBucketKeyTypeLengthAndTTL(t *testing.T) {
	api := &fakeAPI{}
	ps := &fakePresigner{url: "https://signed.example.com/upload?sig=1"}
	cfg := validConfig()
	client, err := NewClient(cfg, api, WithPresigner(ps))
	require.NoError(t, err)

	ttl := 15 * time.Minute
	url, err := client.PresignPut(context.Background(), "pending/videos/abc.mp4", "video/mp4", 12345, ttl)

	require.NoError(t, err)
	assert.Equal(t, "https://signed.example.com/upload?sig=1", url)
	require.Equal(t, 1, ps.calls, "PresignPut must presign exactly once")
	require.NotNil(t, ps.lastIn)
	require.NotNil(t, ps.lastIn.Bucket, "the presigned PUT must target a bucket")
	assert.Equal(t, cfg.Bucket, *ps.lastIn.Bucket)
	require.NotNil(t, ps.lastIn.Key, "the presigned PUT must use the requested key")
	assert.Equal(t, "pending/videos/abc.mp4", *ps.lastIn.Key)
	require.NotNil(t, ps.lastIn.ContentType, "the presigned PUT must sign the content type")
	assert.Equal(t, "video/mp4", *ps.lastIn.ContentType)
	require.NotNil(t, ps.lastIn.ContentLength, "the presigned PUT must sign the content length")
	assert.Equal(t, int64(12345), *ps.lastIn.ContentLength)
	assert.Equal(t, ttl, ps.lastExpires, "the presigned URL must expire after the requested TTL")
	assert.Equal(t, 0, api.putCalls, "presigning must not store any object")
}

func TestClient_PresignPut_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		contentType string
		size        int64
	}{
		{
			name:        "empty key",
			key:         "",
			contentType: "video/mp4",
			size:        1024,
		},
		{
			name:        "zero size",
			key:         "pending/videos/abc.mp4",
			contentType: "video/mp4",
			size:        0,
		},
		{
			name:        "negative size",
			key:         "pending/videos/abc.mp4",
			contentType: "video/mp4",
			size:        -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{}
			ps := &fakePresigner{url: "https://signed.example.com/upload"}
			client := newPresignTestClient(t, api, ps)

			url, err := client.PresignPut(context.Background(), tt.key, tt.contentType, tt.size, 15*time.Minute)

			require.Error(t, err, "invalid presign input must be rejected")
			assert.ErrorIs(t, err, errs.ErrInvalidInput)
			assert.Empty(t, url, "no presigned URL may be returned for rejected input")
			assert.Equal(t, 0, ps.calls, "a rejected presign must never reach the presigner")
		})
	}
}

func TestClient_PresignPut_WithoutPresigner_ReturnsUnavailable(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	url, err := client.PresignPut(context.Background(), "pending/videos/abc.mp4", "video/mp4", 1024, 15*time.Minute)

	require.Error(t, err, "presigning without a presigner must fail")
	assert.ErrorIs(t, err, errs.ErrServiceUnavailable)
	assert.Empty(t, url)
}

func TestClient_PresignPut_WrapsPresignerError(t *testing.T) {
	api := &fakeAPI{}
	sentinel := errors.New("presign boom")
	ps := &fakePresigner{err: sentinel}
	client := newPresignTestClient(t, api, ps)

	url, err := client.PresignPut(context.Background(), "pending/videos/abc.mp4", "video/mp4", 1024, 15*time.Minute)

	require.Error(t, err, "a presigner failure must not be swallowed")
	assert.Empty(t, url)
	assert.ErrorIs(t, err, sentinel, "the presigner error must be wrapped, not replaced")
}

func TestClient_Inspect_ReturnsSizeAndHeadBytes(t *testing.T) {
	body := []byte("0123456789ABCDEF")
	tests := []struct {
		name      string
		n         int
		wantRange string
		wantHead  []byte
	}{
		{
			name:      "prefix of a longer body",
			n:         4,
			wantRange: "bytes=0-3",
			wantHead:  []byte("0123"),
		},
		{
			name:      "first 512 bytes",
			n:         512,
			wantRange: "bytes=0-511",
			wantHead:  body,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{
				headOut: &s3.HeadObjectOutput{ContentLength: aws.Int64(int64(len(body)))},
				getBody: body,
			}
			client := newTestClient(t, validConfig(), api)

			size, head, err := client.Inspect(context.Background(), "pending/videos/abc.mp4", tt.n)

			require.NoError(t, err)
			assert.Equal(t, int64(len(body)), size, "Inspect must report the HeadObject size")
			assert.Equal(t, tt.wantHead, head)
			require.Equal(t, 1, api.headCalls, "Inspect must issue exactly one HeadObject call")
			require.NotNil(t, api.lastHead)
			assert.Equal(t, validConfig().Bucket, aws.ToString(api.lastHead.Bucket))
			assert.Equal(t, "pending/videos/abc.mp4", aws.ToString(api.lastHead.Key))
			require.Equal(t, 1, api.getCalls, "Inspect must issue exactly one ranged GetObject call")
			require.NotNil(t, api.lastGet)
			assert.Equal(t, validConfig().Bucket, aws.ToString(api.lastGet.Bucket))
			assert.Equal(t, "pending/videos/abc.mp4", aws.ToString(api.lastGet.Key))
			require.NotNil(t, api.lastGet.Range, "the GET must carry an HTTP Range header")
			assert.Equal(t, tt.wantRange, *api.lastGet.Range)
		})
	}
}

func TestClient_Inspect_TruncatesLongBodyToNBytes(t *testing.T) {
	full := bytes.Repeat([]byte{0x1A}, 1024)
	api := &fakeAPI{
		headOut: &s3.HeadObjectOutput{ContentLength: aws.Int64(int64(len(full)))},
		getBody: full,
	}
	client := newTestClient(t, validConfig(), api)

	size, head, err := client.Inspect(context.Background(), "pending/videos/abc.mp4", 512)

	require.NoError(t, err)
	assert.Equal(t, int64(1024), size, "size comes from HeadObject, not from the ranged body")
	require.Len(t, head, 512, "only the first n bytes may be returned when the body is longer")
	assert.Equal(t, full[:512], head)
	require.NotNil(t, api.lastGet.Range)
	assert.Equal(t, "bytes=0-511", *api.lastGet.Range)
}

func TestClient_Inspect_RejectsEmptyKey(t *testing.T) {
	api := &fakeAPI{}
	client := newTestClient(t, validConfig(), api)

	size, head, err := client.Inspect(context.Background(), "", 512)

	require.Error(t, err, "an empty object key must be rejected")
	assert.ErrorIs(t, err, errs.ErrInvalidInput)
	assert.Equal(t, int64(0), size)
	assert.Empty(t, head)
	assert.Equal(t, 0, api.headCalls, "an invalid key must never reach object storage")
	assert.Equal(t, 0, api.getCalls, "an invalid key must never reach object storage")
}

func TestClient_Inspect_MapsNotFoundToErrObjectNotFound(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "NotFound",
			code: "NotFound",
		},
		{
			name: "NoSuchKey",
			code: "NoSuchKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{
				headErr: &smithy.GenericAPIError{Code: tt.code, Message: "missing"},
			}
			client := newTestClient(t, validConfig(), api)

			size, head, err := client.Inspect(context.Background(), "pending/videos/missing.mp4", 512)

			require.Error(t, err, "a missing object must be reported")
			assert.ErrorIs(t, err, errs.ErrObjectNotFound)
			assert.Equal(t, int64(0), size)
			assert.Empty(t, head)
			assert.Equal(t, 1, api.headCalls)
		})
	}
}

func TestClient_Inspect_WrapsOtherErrors(t *testing.T) {
	sentinel := &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"}
	api := &fakeAPI{headErr: sentinel}
	client := newTestClient(t, validConfig(), api)

	_, _, err := client.Inspect(context.Background(), "pending/videos/denied.mp4", 512)

	require.Error(t, err, "a real storage failure must not be swallowed")
	assert.ErrorIs(t, err, sentinel, "the storage error must be wrapped, not replaced")
	assert.NotErrorIs(t, err, errs.ErrObjectNotFound, "a permission failure is not a missing object")
}

func TestClient_Inspect_WrapsGetError(t *testing.T) {
	sentinel := errors.New("get boom")
	api := &fakeAPI{
		headOut: &s3.HeadObjectOutput{ContentLength: aws.Int64(16)},
		getErr:  sentinel,
	}
	client := newTestClient(t, validConfig(), api)

	_, _, err := client.Inspect(context.Background(), "pending/videos/abc.mp4", 512)

	require.Error(t, err, "a ranged GET failure must not be swallowed")
	assert.ErrorIs(t, err, sentinel, "the GET error must be wrapped, not replaced")
}

func TestClient_Copy_ReturnsPublicURLAndUsesBucketQualifiedSource(t *testing.T) {
	api := &fakeAPI{}
	cfg := validConfig()
	cfg.PublicBaseURL = "https://pub.example.com/x"
	client, err := NewClient(cfg, api)
	require.NoError(t, err)

	url, err := client.Copy(context.Background(), "pending/videos/abc.mp4", "videos/slug-12345678.mp4")

	require.NoError(t, err)
	assert.Equal(t, "https://pub.example.com/x/videos/slug-12345678.mp4", url)
	assert.NotContains(t, url, "AKID", "the public URL must not leak credentials")
	require.Equal(t, 1, api.copyCalls, "Copy must issue exactly one CopyObject call")
	require.NotNil(t, api.lastCopy)
	require.NotNil(t, api.lastCopy.Bucket, "the copy must target a bucket")
	assert.Equal(t, cfg.Bucket, *api.lastCopy.Bucket)
	require.NotNil(t, api.lastCopy.Key, "the copy must use the destination key")
	assert.Equal(t, "videos/slug-12345678.mp4", *api.lastCopy.Key)
	require.NotNil(t, api.lastCopy.CopySource, "the copy must reference the source object")
	assert.Equal(t, cfg.Bucket+"/pending/videos/abc.mp4", *api.lastCopy.CopySource)
}

func TestClient_Copy_RejectsEmptyKeys(t *testing.T) {
	tests := []struct {
		name string
		src  string
		dst  string
	}{
		{
			name: "empty source",
			src:  "",
			dst:  "videos/slug-12345678.mp4",
		},
		{
			name: "empty destination",
			src:  "pending/videos/abc.mp4",
			dst:  "",
		},
		{
			name: "both empty",
			src:  "",
			dst:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := newTestClient(t, validConfig(), api)

			url, err := client.Copy(context.Background(), tt.src, tt.dst)

			require.Error(t, err, "empty copy keys must be rejected")
			assert.ErrorIs(t, err, errs.ErrInvalidInput)
			assert.Empty(t, url)
			assert.Equal(t, 0, api.copyCalls, "an invalid copy must never reach object storage")
		})
	}
}

func TestClient_Copy_MapsNotFoundToErrObjectNotFound(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "NotFound",
			code: "NotFound",
		},
		{
			name: "NoSuchKey",
			code: "NoSuchKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{
				copyErr: &smithy.GenericAPIError{Code: tt.code, Message: "missing"},
			}
			client := newTestClient(t, validConfig(), api)

			url, err := client.Copy(context.Background(), "pending/videos/missing.mp4", "videos/slug-12345678.mp4")

			require.Error(t, err, "copying a missing source must be reported")
			assert.ErrorIs(t, err, errs.ErrObjectNotFound)
			assert.Empty(t, url)
			assert.Equal(t, 1, api.copyCalls)
		})
	}
}

func TestClient_Copy_WrapsOtherErrors(t *testing.T) {
	sentinel := errors.New("copy boom")
	api := &fakeAPI{copyErr: sentinel}
	client := newTestClient(t, validConfig(), api)

	url, err := client.Copy(context.Background(), "pending/videos/abc.mp4", "videos/slug-12345678.mp4")

	require.Error(t, err, "a real storage failure must not be swallowed")
	assert.Empty(t, url)
	assert.ErrorIs(t, err, sentinel, "the storage error must be wrapped, not replaced")
}

// TestClient_Inspect_GetSendsSingleByteRange guards the Range format for n=1.
func TestClient_Inspect_GetSendsSingleByteRange(t *testing.T) {
	api := &fakeAPI{
		headOut: &s3.HeadObjectOutput{ContentLength: aws.Int64(16)},
		getBody: []byte("0123456789ABCDEF"),
	}
	client := newTestClient(t, validConfig(), api)

	_, head, err := client.Inspect(context.Background(), "pending/videos/abc.mp4", 1)

	require.NoError(t, err)
	require.Len(t, head, 1)
	require.NotNil(t, api.lastGet.Range)
	assert.Equal(t, "bytes=0-0", *api.lastGet.Range)
}
