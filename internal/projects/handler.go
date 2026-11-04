package projects

import (
	"context"

	"github.com/tinhthanh/go-service-template/internal/api"
)

// Handler answers the contract's projects, project, createProject, updateProject, deleteProject and projectTags.
type Handler struct {
	svc *Service
}

// NewHandler -.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Projects is a page of the caller's projects that match.
func (h *Handler) Projects(ctx context.Context, req api.ProjectsRequestObject) (api.ProjectsResponseObject, error) {
	p := req.Params
	page, err := h.svc.List(ctx, &Ask{
		Workspace: deref(p.Workspace), Q: deref(p.Q), Status: string(deref(p.Status)), Tags: deref(p.Tags),
		Sort: deref(p.Sort), Cursor: deref(p.Cursor), Limit: deref(p.Limit),
	})
	if err != nil {
		return nil, err
	}

	out := api.Projects200JSONResponse{Items: make([]api.Project, 0, len(page.Items)), Total: page.Total}
	for i := range page.Items {
		out.Items = append(out.Items, contract(&page.Items[i]))
	}

	if page.Next != "" {
		out.Next = &page.Next
	}

	return out, nil
}

// Project is one of the caller's projects.
func (h *Handler) Project(ctx context.Context, req api.ProjectRequestObject) (api.ProjectResponseObject, error) {
	pr, err := h.svc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.Project200JSONResponse(contract(&pr)), nil
}

// CreateProject makes a project.
func (h *Handler) CreateProject(ctx context.Context, req api.CreateProjectRequestObject) (api.CreateProjectResponseObject, error) {
	pr, err := h.svc.Create(ctx, Input{Name: req.Body.Name, Description: req.Body.Description, Workspace: deref(req.Body.Workspace)})
	if err != nil {
		return nil, err
	}

	return api.CreateProject200JSONResponse(contract(&pr)), nil
}

// UpdateProject changes a project.
func (h *Handler) UpdateProject(ctx context.Context, req api.UpdateProjectRequestObject) (api.UpdateProjectResponseObject, error) {
	patch := Patch{Name: req.Body.Name, Description: req.Body.Description, Tags: req.Body.Tags}
	if req.Body.Status != nil {
		s := string(*req.Body.Status)
		patch.Status = &s
	}

	pr, err := h.svc.Update(ctx, req.Id, patch)
	if err != nil {
		return nil, err
	}

	return api.UpdateProject200JSONResponse(contract(&pr)), nil
}

// DeleteProject removes a project.
func (h *Handler) DeleteProject(ctx context.Context, req api.DeleteProjectRequestObject) (api.DeleteProjectResponseObject, error) {
	if err := h.svc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}

	return api.DeleteProject204Response{}, nil
}

// ProjectTags is every tag in use.
func (h *Handler) ProjectTags(ctx context.Context, req api.ProjectTagsRequestObject) (api.ProjectTagsResponseObject, error) {
	tags, err := h.svc.Tags(ctx, deref(req.Params.Workspace))
	if err != nil {
		return nil, err
	}

	return api.ProjectTags200JSONResponse(tags), nil
}

func contract(p *Project) api.Project {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}

	return api.Project{
		Id: p.ID, Workspace: p.Workspace, Name: p.Name, Description: p.Description, Status: api.ProjectStatus(p.Status),
		Owner: p.Owner, Tags: tags, UpdatedAt: p.UpdatedAt.UTC(),
	}
}

func deref[T any](v *T) T {
	if v == nil {
		var zero T

		return zero
	}

	return *v
}
