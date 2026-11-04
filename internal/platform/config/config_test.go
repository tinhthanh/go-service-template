package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/config"
)

const _db = "postgres://acme@localhost:5432/acme"

func TestLoad_TheDefaultsServeTheAPIUnderAPI(t *testing.T) {
	t.Setenv("DATABASE_URL", _db)

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.Addr)
	assert.Equal(t, "/api", cfg.APIPrefix)
	assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout)
	assert.Empty(t, cfg.CORSOrigins, "no origin may call with cookies until one is named")
	assert.True(t, cfg.CookieSecure, "cookies go over HTTPS only unless told otherwise")
	assert.Equal(t, 8*time.Hour, cfg.SessionLength)
}

func TestLoad_ADatabaseIsRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := config.Load()
	require.ErrorContains(t, err, "DATABASE_URL")
}

func TestLoad_OriginsAreAList(t *testing.T) {
	t.Setenv("DATABASE_URL", _db)
	t.Setenv("CORS_ORIGINS", "https://a.test,https://b.test")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, []string{"https://a.test", "https://b.test"}, cfg.CORSOrigins)
}

func TestLoad_AValueThatIsntOneIsRefusedByName(t *testing.T) {
	t.Setenv("DATABASE_URL", _db)
	t.Setenv("SHUTDOWN_TIMEOUT", "soon")

	_, err := config.Load()
	require.ErrorContains(t, err, "SHUTDOWN_TIMEOUT")
}
