package dto

import (
	"time"

	"github.com/maxaixs/mandat/backend/internal/service"
)

// UploadRequest announces a video or image file the editor is about to upload.
type UploadRequest struct {
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// UploadResponse is a presigned direct upload to object storage.
type UploadResponse struct {
	Key       string    `json:"key"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ToInput maps the request body onto the service request.
func (r UploadRequest) ToInput() service.UploadRequest {
	return service.UploadRequest{ContentType: r.ContentType, Size: r.Size}
}

// UploadToResponse maps a presigned upload onto its response payload.
func UploadToResponse(upload *service.PresignedUpload) *UploadResponse {
	return &UploadResponse{Key: upload.Key, UploadURL: upload.UploadURL, ExpiresAt: upload.ExpiresAt}
}
