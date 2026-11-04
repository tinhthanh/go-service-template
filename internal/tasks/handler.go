package tasks

import (
	"context"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/tinhthanh/go-service-template/internal/api"
	"github.com/tinhthanh/go-service-template/internal/platform/httpx"
)

// Handler answers the contract's tasks, task, createTask, updateTask and deleteTask.
type Handler struct {
	svc *Service
}

// NewHandler -.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Tasks is a page of the caller's tasks that match, and how many of all of them are late.
func (h *Handler) Tasks(ctx context.Context, req api.TasksRequestObject) (api.TasksResponseObject, error) {
	p := req.Params
	page, err := h.svc.List(ctx, &Ask{
		Workspace: deref(p.Workspace), Q: deref(p.Q), Status: string(deref(p.Status)), Project: deref(p.Project),
		HideDone: deref(p.HideDone), Sort: deref(p.Sort), Cursor: deref(p.Cursor), Limit: deref(p.Limit),
	})
	if err != nil {
		return nil, err
	}

	out := api.Tasks200JSONResponse{Items: make([]api.Task, 0, len(page.Items)), Total: page.Total}
	out.Summary.Late = page.Late

	for i := range page.Items {
		out.Items = append(out.Items, contract(&page.Items[i]))
	}

	if page.Next != "" {
		out.Next = &page.Next
	}

	return out, nil
}

// Task is one of the caller's tasks.
func (h *Handler) Task(ctx context.Context, req api.TaskRequestObject) (api.TaskResponseObject, error) {
	t, err := h.svc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return api.Task200JSONResponse(contract(&t)), nil
}

// CreateTask makes a task.
func (h *Handler) CreateTask(ctx context.Context, req api.CreateTaskRequestObject) (api.CreateTaskResponseObject, error) {
	b := req.Body
	t, err := h.svc.Create(ctx, &Input{
		Title: b.Title, Status: string(deref(b.Status)), Workspace: deref(b.Workspace),
		Due: dayOf(b.Due), Project: b.Project, Attachment: b.Attachment,
	})
	if err != nil {
		return nil, err
	}

	return api.CreateTask200JSONResponse(contract(&t)), nil
}

// UpdateTask changes a task; null takes a due day, a project or a file off.
func (h *Handler) UpdateTask(ctx context.Context, req api.UpdateTaskRequestObject) (api.UpdateTaskResponseObject, error) {
	b := req.Body
	patch := Patch{
		Title: b.Title, Due: dayOf(b.Due), Project: b.Project, Attachment: b.Attachment,
		NoDue: httpx.SentNull(ctx, "due"), NoProject: httpx.SentNull(ctx, "project"), NoAttachment: httpx.SentNull(ctx, "attachment"),
	}

	if b.Status != nil {
		s := string(*b.Status)
		patch.Status = &s
	}

	t, err := h.svc.Update(ctx, req.Id, &patch)
	if err != nil {
		return nil, err
	}

	return api.UpdateTask200JSONResponse(contract(&t)), nil
}

// DeleteTask removes a task.
func (h *Handler) DeleteTask(ctx context.Context, req api.DeleteTaskRequestObject) (api.DeleteTaskResponseObject, error) {
	if err := h.svc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}

	return api.DeleteTask204Response{}, nil
}

func contract(t *Task) api.Task {
	out := api.Task{Id: t.ID, Workspace: t.Workspace, Title: t.Title, Status: api.TaskStatus(t.Status), UpdatedAt: t.UpdatedAt.UTC()}

	if t.Due != nil {
		out.Due = &openapi_types.Date{Time: *t.Due}
	}

	if t.Project != "" {
		out.Project = &struct {
			Id   string `json:"id"` //nolint:revive,staticcheck // the generated api.Task.Project type names it Id
			Name string `json:"name"`
		}{Id: t.Project, Name: t.ProjectName}
	}

	if a := t.Attachment; a != nil {
		out.Attachment = &api.Attachment{Id: a.ID, Name: a.Name, Type: a.Type, Size: a.Size}
	}

	return out
}

func dayOf(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}

	return &d.Time
}

func deref[T any](v *T) T {
	if v == nil {
		var zero T

		return zero
	}

	return *v
}
