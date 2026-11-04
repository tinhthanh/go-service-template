package files

import (
	"context"

	"github.com/tinhthanh/go-service-template/internal/api"
)

// Handler answers the contract's uploadFile.
type Handler struct {
	svc *Service
}

// NewHandler -.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// UploadFile keeps the file the request carries.
func (h *Handler) UploadFile(ctx context.Context, req api.UploadFileRequestObject) (api.UploadFileResponseObject, error) {
	a, err := h.svc.Upload(ctx, req.Params.XFileName, req.ContentType, req.Body)
	if err != nil {
		return nil, err
	}

	return api.UploadFile200JSONResponse{Id: a.ID, Name: a.Name, Type: a.Type, Size: a.Size}, nil
}
