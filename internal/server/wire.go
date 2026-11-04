package server

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tinhthanh/go-service-template/internal/platform/config"
	"github.com/tinhthanh/go-service-template/internal/projects"
	"github.com/tinhthanh/go-service-template/internal/search"
	"github.com/tinhthanh/go-service-template/internal/session"
	"github.com/tinhthanh/go-service-template/internal/tasks"
	"github.com/tinhthanh/go-service-template/internal/workspaces"
)

// Wire makes each feature from its store on the pool: the one place the
// features meet, and where one is handed what another provides.
func Wire(cfg *config.Config, pool *pgxpool.Pool) (Features, error) {
	sessions, err := session.New(session.NewPGStore(pool), session.Lengths{Session: cfg.SessionLength, Refresh: cfg.RefreshLength}, time.Now)
	if err != nil {
		return Features{}, err
	}

	return Features{
		Auth:       sessions,
		Session:    session.NewHandler(sessions, session.Cookies{Secure: cfg.CookieSecure, RefreshPath: cfg.APIPrefix + "/session/refresh"}),
		Workspaces: workspaces.NewHandler(workspaces.New(workspaces.NewPGStore(pool))),
		Projects:   projects.NewHandler(projects.New(projects.NewPGStore(pool))),
		Tasks:      tasks.NewHandler(tasks.New(tasks.NewPGStore(pool), time.Now)),
		Search:     search.NewHandler(search.New(search.NewPGStore(pool))),
	}, nil
}
