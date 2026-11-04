package workspaces

import (
	"context"

	"github.com/tinhthanh/go-service-template/internal/api"
)

// Handler answers the contract's workspaces and createWorkspace.
type Handler struct {
	svc *Service
}

// NewHandler -.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Workspaces is the caller's workspaces.
func (h *Handler) Workspaces(ctx context.Context, _ api.WorkspacesRequestObject) (api.WorkspacesResponseObject, error) {
	ws, err := h.svc.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make(api.Workspaces200JSONResponse, 0, len(ws))
	for _, w := range ws {
		out = append(out, api.Workspace{Id: w.ID, Name: w.Name})
	}

	return out, nil
}

// CreateWorkspace makes one the caller belongs to.
func (h *Handler) CreateWorkspace(ctx context.Context, req api.CreateWorkspaceRequestObject) (api.CreateWorkspaceResponseObject, error) {
	w, err := h.svc.Create(ctx, req.Body.Name)
	if err != nil {
		return nil, err
	}

	return api.CreateWorkspace200JSONResponse{Id: w.ID, Name: w.Name}, nil
}
