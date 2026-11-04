package search

import (
	"context"

	"github.com/tinhthanh/go-service-template/internal/api"
)

// Handler answers the contract's search.
type Handler struct {
	svc *Service
}

// NewHandler -.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Search is what the letters find, best first, each kind on its own.
func (h *Handler) Search(ctx context.Context, req api.SearchRequestObject) (api.SearchResponseObject, error) {
	hits, err := h.svc.Find(ctx, req.Params.Q)
	if err != nil {
		return nil, err
	}

	out := make(api.Search200JSONResponse, 0, len(hits))

	for _, hit := range hits {
		h := api.SearchHit{Kind: api.SearchHitKind(hit.Kind), Id: hit.ID, Name: hit.Name}
		if hit.Detail != "" {
			h.Detail = &hit.Detail
		}

		if hit.Status != "" {
			h.Status = &hit.Status
		}

		out = append(out, h)
	}

	return out, nil
}
