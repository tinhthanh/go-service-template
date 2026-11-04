package files_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/files"
	"github.com/tinhthanh/go-service-template/internal/server/servertest"
)

// upload sends a file as the client's lib/api.ts does: its bytes as the body, its name URI-encoded.
func upload(s *servertest.Server, who map[string]string, name, kind, body string) (int, map[string]any) {
	header := map[string]string{"X-File-Name": name, "Content-Type": kind}
	maps.Copy(header, who)

	a := s.Do(http.MethodPost, "/files", body, header)

	var out map[string]any
	_ = json.Unmarshal(a.Body, &out)

	return a.Status, out
}

func TestUploadFile_KeepsTheFileAsItCameAndAnswersItsAttachment(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	status, a := upload(s, s.As(servertest.Member), "notes%20%E2%80%94%20draft.txt", "text/plain", "the notes")

	require.Equal(t, http.StatusOK, status, a)
	assert.NotEmpty(t, a["id"])
	assert.Equal(t, "notes — draft.txt", a["name"], "the name as the client encoded it")
	assert.Equal(t, "text/plain", a["type"])
	assert.InDelta(t, len("the notes"), a["size"], 0)
}

func TestUploadFile_PastFiveMBIsRefusedOnTheField(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	status, a := upload(s, s.As(servertest.Member), "big.bin", "application/octet-stream", strings.Repeat("x", files.MaxSize+1))

	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, map[string]any{"error": "Files up to 5 MB, please.", "code": "invalid", "field": "attachment"}, a, "as the client's mock refuses it")

	status, _ = upload(s, s.As(servertest.Member), "fits.bin", "application/octet-stream", strings.Repeat("x", files.MaxSize))
	assert.Equal(t, http.StatusOK, status, "5 MB itself is kept")
}

func TestUploadFile_FarPastWhatTheServerTakesIs413(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	status, a := upload(s, s.As(servertest.Member), "huge.bin", "application/octet-stream", string(bytes.Repeat([]byte("x"), 9<<20)))

	assert.Equal(t, http.StatusRequestEntityTooLarge, status)
	assert.Equal(t, "too_large", a["code"])
}

func TestUploadFile_AViewerMayNotAndNobodyIs401(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	status, _ := upload(s, s.As(servertest.Viewer), "x.txt", "text/plain", "x")
	assert.Equal(t, http.StatusForbidden, status)

	status, _ = upload(s, nil, "x.txt", "text/plain", "x")
	assert.Equal(t, http.StatusUnauthorized, status)
}
