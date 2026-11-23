// Package r2 stores cover images and article videos in Cloudflare R2 over the S3 API.
// It imports internal/errs for the application's sentinel errors, so the
// package is intentionally coupled to this module and not reusable elsewhere.
package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/pkg/imagetype"
)

// MaxUploadBytes caps a single cover upload.
const MaxUploadBytes = 10 << 20

// S3API is the minimal S3-API surface the client needs.
// The production S3 client satisfies it; tests inject a fake.
type S3API interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(
		ctx context.Context,
		in *s3.DeleteObjectInput,
		optFns ...func(*s3.Options),
	) (*s3.DeleteObjectOutput, error)
	HeadObject(ctx context.Context, in *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	CopyObject(ctx context.Context, in *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}

// Presigner signs direct-upload URLs. The production presign client satisfies it.
type Presigner interface {
	PresignPutObject(
		ctx context.Context,
		in *s3.PutObjectInput,
		optFns ...func(*s3.PresignOptions),
	) (*v4.PresignedHTTPRequest, error)
}

// Option customizes a Client.
type Option func(*Client)

// WithPresigner sets the signer of direct-upload URLs.
func WithPresigner(p Presigner) Option {
	return func(c *Client) { c.presigner = p }
}

// Client stores cover images and article videos in Cloudflare R2 over the S3 API.
type Client struct {
	cfg       Config
	api       S3API
	presigner Presigner
}

// NewClient validates cfg and returns a Client. A nil api builds the real
// S3 client and presigner pointed at the R2 endpoint; tests pass fakes instead.
func NewClient(cfg Config, api S3API, opts ...Option) (*Client, error) {
	const fc = "r2Client_NewClient"

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	client := &Client{cfg: cfg, api: api}
	if api == nil {
		s3Client := s3.NewFromConfig(aws.Config{
			Region:      "auto",
			Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		}, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
		client.api = s3Client
		client.presigner = s3.NewPresignClient(s3Client)
	}
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Upload stores body under key and returns its public URL.
// Content sniffing is authoritative: the declared content type is ignored
// and the key extension must agree with the sniffed image type.
func (c *Client) Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("%w: empty object key", errs.ErrInvalidInput)
	}
	if body == nil {
		return "", fmt.Errorf("%w: nil upload body", errs.ErrInvalidInput)
	}

	data, err := io.ReadAll(io.LimitReader(body, MaxUploadBytes+1))
	if err != nil {
		return "", fmt.Errorf("read upload body: %w", err)
	}
	if len(data) > MaxUploadBytes {
		return "", fmt.Errorf("%w: size exceeds %d bytes", errs.ErrPayloadTooLarge, MaxUploadBytes)
	}

	sniffed := imagetype.Sniff(data)
	if !strings.HasPrefix(sniffed, "image/") {
		return "", fmt.Errorf("%w: object %q is %s, not an image", errs.ErrInvalidInput, key, sniffed)
	}
	if err := checkExtMIME(key, sniffed); err != nil {
		return "", err
	}

	_, err = c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(sniffed),
	})
	if err != nil {
		return "", fmt.Errorf("put %s: %w", key, err)
	}

	return c.publicURL(key), nil
}

// Delete removes key. A missing key is not an error.
func (c *Client) Delete(ctx context.Context, key string) error {
	const fc = "r2Client_Delete"

	if key == "" {
		return fmt.Errorf("%s: %w: empty object key", fc, errs.ErrInvalidInput)
	}

	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil
		}

		return fmt.Errorf("%s: delete %s: %w", fc, key, err)
	}

	return nil
}

// PresignPut returns a URL that lets a browser PUT exactly size bytes of
// contentType under key until ttl expires.
func (c *Client) PresignPut(
	ctx context.Context,
	key string,
	contentType string,
	size int64,
	ttl time.Duration,
) (string, error) {
	const fc = "r2Client_PresignPut"

	if key == "" || size <= 0 {
		return "", fmt.Errorf("%s: %w: empty key or non-positive size", fc, errs.ErrInvalidInput)
	}
	if c.presigner == nil {
		return "", fmt.Errorf("%s: %w: presigner is not configured", fc, errs.ErrServiceUnavailable)
	}

	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.cfg.Bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("%s: presign %s: %w", fc, key, err)
	}

	return req.URL, nil
}

// Inspect returns the size of key and at most n of its leading bytes.
func (c *Client) Inspect(ctx context.Context, key string, n int) (int64, []byte, error) {
	const fc = "r2Client_Inspect"

	if key == "" || n <= 0 {
		return 0, nil, fmt.Errorf("%s: %w: empty object key or non-positive length", fc, errs.ErrInvalidInput)
	}

	head, err := c.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, nil, fmt.Errorf("%s: head %s: %w", fc, key, objectError(err))
	}

	obj, err := c.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=0-%d", n-1)),
	})
	if err != nil {
		return 0, nil, fmt.Errorf("%s: get %s: %w", fc, key, objectError(err))
	}
	defer obj.Body.Close()

	data, err := io.ReadAll(io.LimitReader(obj.Body, int64(n)))
	if err != nil {
		return 0, nil, fmt.Errorf("%s: read %s: %w", fc, key, err)
	}

	return aws.ToInt64(head.ContentLength), data, nil
}

// Copy duplicates srcKey into dstKey inside the bucket and returns the public URL of dstKey.
func (c *Client) Copy(ctx context.Context, srcKey string, dstKey string) (string, error) {
	const fc = "r2Client_Copy"

	if srcKey == "" || dstKey == "" {
		return "", fmt.Errorf("%s: %w: empty object key", fc, errs.ErrInvalidInput)
	}

	_, err := c.api.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(c.cfg.Bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(c.cfg.Bucket + "/" + srcKey),
	})
	if err != nil {
		return "", fmt.Errorf("%s: copy %s to %s: %w", fc, srcKey, dstKey, objectError(err))
	}

	return c.publicURL(dstKey), nil
}

// checkExtMIME rejects a key whose extension disagrees with sniffed content.
func checkExtMIME(key, sniffed string) error {
	dot := strings.LastIndex(key, ".")
	if dot < 0 || dot == len(key)-1 {
		return errs.NewValidationError("cover", "object key must carry an image extension")
	}
	ext := strings.ToLower(key[dot+1:])
	want, ok := imagetype.MIMEForExt(ext)
	if !ok {
		return errs.NewValidationError("cover", "unsupported image type "+ext)
	}
	if want != sniffed {
		return errs.NewValidationError("cover", "file extension "+ext+" does not match content "+sniffed)
	}

	return nil
}

// publicURL exposes the delivery address of key without credentials.
func (c *Client) publicURL(key string) string {
	return strings.TrimRight(c.cfg.PublicBase(), "/") + "/" + strings.TrimLeft(key, "/")
}

// objectError marks missing-object API errors with errs.ErrObjectNotFound.
func objectError(err error) error {
	if isNotFound(err) {
		return fmt.Errorf("%w: %w", errs.ErrObjectNotFound, err)
	}

	return err
}

// isNotFound reports missing-object API errors.
func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	return apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound"
}
