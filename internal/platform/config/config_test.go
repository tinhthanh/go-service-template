package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/config"
)

//nolint:paralleltest // it reads the environment the other tests set
func TestLoad_TheDefaultsServeTheAPIUnderAPI(t *testing.T) {
	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.Addr)
	assert.Equal(t, "/api", cfg.APIPrefix)
	assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout)
	assert.Empty(t, cfg.CORSOrigins, "no origin may call with cookies until one is named")
}

func TestLoad_OriginsAreAList(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "https://a.test,https://b.test")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, []string{"https://a.test", "https://b.test"}, cfg.CORSOrigins)
}

func TestLoad_AValueThatIsntOneIsRefusedByName(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "soon")

	_, err := config.Load()
	require.ErrorContains(t, err, "SHUTDOWN_TIMEOUT")
}
