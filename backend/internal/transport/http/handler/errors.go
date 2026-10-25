package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/service"
)

// ErrResp is an application error classified for the client:
// the HTTP status plus the client-facing message.
type ErrResp struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// errResps maps sentinel errors onto their client-facing responses.
var errResps = map[error]ErrResp{
	errs.ErrNewsNotFound:       {Status: http.StatusNotFound, Message: "News article not found"},
	errs.ErrInvalidInput:       {Status: http.StatusBadRequest, Message: "Invalid input"},
	errs.ErrConflict:           {Status: http.StatusConflict, Message: "Resource conflict"},
	errs.ErrPayloadTooLarge:    {Status: http.StatusRequestEntityTooLarge, Message: "Request payload too large"},
	errs.ErrServiceUnavailable: {Status: http.StatusServiceUnavailable, Message: "Service unavailable"},
}

// respInternal is the fallback response for unexpected errors.
var respInternal = ErrResp{Status: http.StatusInternalServerError, Message: "Internal server error"}

// errResp classifies an application error into an ErrResp.
func errResp(err error) ErrResp {
	var validationErr *service.ValidationErr
	if errors.As(err, &validationErr) {
		return ErrResp{Status: http.StatusBadRequest, Message: validationErr.Reason}
	}

	for sentinel, resp := range errResps {
		if errors.Is(err, sentinel) {
			return resp
		}
	}

	return respInternal
}

// WriteError maps an application error onto a JSON error response.
func WriteError(c *gin.Context, err error) {
	resp := errResp(err)

	c.AbortWithStatusJSON(resp.Status, gin.H{"error": resp})
}
