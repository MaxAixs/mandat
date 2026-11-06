// Package r2 stores cover images in Cloudflare R2 over the S3 API.
// It imports internal/errs for the application's sentinel errors, so the
// package is intentionally coupled to this module and not reusable elsewhere.
package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/maxaixs/mandat/backend/internal/errs"
)

// MaxUploadBytes caps a single cover upload.
const MaxUploadBytes = 10 << 20

// S3API is the minimal S3-API surface the client needs.
// The production S3 client satisfies it; tests inject a fake.
type S3API interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// Client stores cover images in Cloudflare R2 over the S3 API.
type Client struct {
	cfg Config
	api S3API
}

// NewClient validates cfg and returns a Client. A nil api builds the real
// S3 client pointed at the R2 endpoint; tests pass a fake instead.
func NewClient(cfg Config, api S3API) (*Client, error) {
	const fc = "r2Client_NewClient"

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if api == nil {
		api = s3.NewFromConfig(aws.Config{
			Region:      "auto",
			Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		}, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	return &Client{cfg: cfg, api: api}, nil
}

// mimeForExt maps accepted key extensions onto sniffed MIME types.
var mimeForExt = map[string]string{
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
	"gif":  "image/gif",
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

	sniffed := sniffMIME(data)
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

// sniffMIME reports the content type of data from its leading bytes.
func sniffMIME(data []byte) string {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}

	return http.DetectContentType(head)
}

// checkExtMIME rejects a key whose extension disagrees with sniffed content.
func checkExtMIME(key, sniffed string) error {
	dot := strings.LastIndex(key, ".")
	if dot < 0 || dot == len(key)-1 {
		return errs.NewValidationError("cover", "object key must carry an image extension")
	}
	ext := strings.ToLower(key[dot+1:])
	want, ok := mimeForExt[ext]
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

// isNotFound reports missing-object API errors.
func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	return apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound"
}
