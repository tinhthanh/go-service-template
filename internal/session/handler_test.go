package session_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/server/servertest"
)

func cookies(t *testing.T, a interface{ Cookies() []*http.Cookie }) map[string]*http.Cookie {
	t.Helper()

	byName := map[string]*http.Cookie{}
	for _, c := range a.Cookies() {
		byName[c.Name] = c
	}

	return byName
}

func TestSignIn_SetsCookiesNoScriptCanRead(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	a := s.Do(http.MethodPost, "/session", `{"email": "Member@ACME.test", "password": "`+servertest.Password+`"}`, nil)

	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	set := cookies(t, &a)
	require.Contains(t, set, "session")
	require.Contains(t, set, "refresh")

	for _, c := range set {
		assert.True(t, c.HttpOnly, c.Name)
		assert.True(t, c.Secure, c.Name)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite, c.Name)
	}

	assert.Equal(t, "/", set["session"].Path)
	assert.Equal(t, "/api/session/refresh", set["refresh"].Path, "the refresh cookie goes to the refresh path only")
	assert.JSONEq(t, `{"id": "u-member", "name": "Max Member", "email": "member@acme.test", "role": "member", "sessionEndsAt": "2026-11-04T10:00:00Z"}`, string(a.Body))
}

func TestSignIn_AWrongPasswordAndAnUnknownEmailAreRefusedAlike(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	wrong := s.Do(http.MethodPost, "/session", `{"email": "member@acme.test", "password": "nope"}`, nil)
	unknown := s.Do(http.MethodPost, "/session", `{"email": "nobody@acme.test", "password": "nope"}`, nil)

	assert.Equal(t, http.StatusUnauthorized, wrong.Status)
	assert.Equal(t, wrong.Body, unknown.Body, "the answer says nothing of who has an account")
	assert.Empty(t, wrong.Cookies())
	assert.Zero(t, s.Sessions.Sessions())
}

func TestSignIn_FromAnotherSiteIsRefused(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	// The browser sends the form; it can't add X-Requested-With without the server's CORS allowing it.
	a := s.Do(http.MethodPost, "/session", `{"email": "member@acme.test", "password": "`+servertest.Password+`"}`, map[string]string{"X-Requested-With": ""})

	assert.Equal(t, http.StatusForbidden, a.Status)
}

func TestMe_IsWhoTheCookieNames(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	a := s.Do(http.MethodGet, "/session", "", map[string]string{"Cookie": s.SignIn("owner@acme.test")})
	require.Equal(t, http.StatusOK, a.Status)

	var u struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	require.NoError(t, json.Unmarshal(a.Body, &u))
	assert.Equal(t, servertest.Owner, u.ID)
	assert.Equal(t, "owner", u.Role)
}

func TestMe_NobodySignedInIs401(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	assert.Equal(t, http.StatusUnauthorized, s.Do(http.MethodGet, "/session", "", nil).Status)
	assert.Equal(t, http.StatusUnauthorized, s.Do(http.MethodGet, "/session", "", map[string]string{"Cookie": "session=forged"}).Status)
}

func TestMe_ASessionThatHasEndedIs401(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	cookie := s.SignIn("member@acme.test")

	s.Clock().Advance(servertest.Lengths.Session + time.Second)

	assert.Equal(t, http.StatusUnauthorized, s.Do(http.MethodGet, "/session", "", map[string]string{"Cookie": cookie}).Status)
}

func TestRefreshSession_GivesNewCookiesAndTheOldOnesStop(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	old := s.SignIn("member@acme.test")

	s.Clock().Advance(servertest.Lengths.Session + time.Second)

	renewed := s.Do(http.MethodPost, "/session/refresh", "", map[string]string{"Cookie": old})
	require.Equal(t, http.StatusOK, renewed.Status, string(renewed.Body))
	assert.Contains(t, string(renewed.Body), `"sessionEndsAt":"2026-11-04T11:00:01Z"`)

	fresh := servertest.CookieHeader(renewed.Cookies())
	assert.Equal(t, http.StatusOK, s.Do(http.MethodGet, "/session", "", map[string]string{"Cookie": fresh}).Status)
	assert.Equal(t, http.StatusUnauthorized, s.Do(http.MethodPost, "/session/refresh", "", map[string]string{"Cookie": old}).Status, "a refresh token works once")
}

func TestRefreshSession_AfterTheRefreshHasEndedIs401(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	cookie := s.SignIn("member@acme.test")

	s.Clock().Advance(servertest.Lengths.Refresh + time.Second)

	assert.Equal(t, http.StatusUnauthorized, s.Do(http.MethodPost, "/session/refresh", "", map[string]string{"Cookie": cookie}).Status)
}

func TestSignOut_EndsTheSessionAndItsCookies(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	cookie := s.SignIn("member@acme.test")

	out := s.Do(http.MethodDelete, "/session", "", map[string]string{"Cookie": cookie})
	require.Equal(t, http.StatusNoContent, out.Status)

	for _, c := range out.Cookies() {
		assert.Empty(t, c.Value, c.Name)
		assert.True(t, c.Expires.Before(time.Now()), c.Name)
	}

	assert.Zero(t, s.Sessions.Sessions())
	assert.Equal(t, http.StatusUnauthorized, s.Do(http.MethodGet, "/session", "", map[string]string{"Cookie": cookie}).Status, "the cookie kept is no session")
}
