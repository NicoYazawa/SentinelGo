package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// stubTransport routes github.com / api.github.com requests to the test server.
type stubTransport struct{ server *httptest.Server }

func (s stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, _ := url.Parse(s.server.URL)
	u := *req.URL
	u.Scheme = target.Scheme
	u.Host = target.Host
	req.URL = &u
	return http.DefaultTransport.RoundTrip(req)
}

func TestGetAuthURL(t *testing.T) {
	g := NewGitHubOAuth("my-client", "secret", "http://cb", "http://fe", "")
	u := g.GetAuthURL("state-123")
	if !strings.Contains(u, "client_id=my-client") {
		t.Errorf("auth url missing client_id: %s", u)
	}
	if !strings.Contains(u, "state=state-123") {
		t.Errorf("auth url missing state: %s", u)
	}
	// parse to ensure it is valid
	if _, err := url.Parse(u); err != nil {
		t.Errorf("auth url invalid: %v", err)
	}
}

func TestGetFrontendURL(t *testing.T) {
	g := NewGitHubOAuth("a", "b", "c", "http://frontend", "")
	if g.GetFrontendURL() != "http://frontend" {
		t.Error("frontend url wrong")
	}
}

func TestNewGitHubOAuthProxy(t *testing.T) {
	g := NewGitHubOAuth("a", "b", "c", "d", "http://proxy:8080")
	if g.httpClient == nil || g.httpClient.Transport == nil {
		t.Error("proxy transport not set")
	}
}

func TestExchangeCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok-abc"})
	}))
	defer ts.Close()

	g := &GitHubOAuth{ClientID: "cid", ClientSecret: "sec", RedirectURI: "cb", httpClient: &http.Client{Transport: stubTransport{ts}}}
	tok, err := g.ExchangeCode("code-1")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "tok-abc" {
		t.Errorf("token wrong: %s", tok)
	}
}

func TestExchangeCodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification"})
	}))
	defer ts.Close()

	g := &GitHubOAuth{httpClient: &http.Client{Transport: stubTransport{ts}}}
	if _, err := g.ExchangeCode("code-1"); err == nil {
		t.Error("expected error from github")
	}
}

func TestGetUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(GitHubUser{ID: 42, Login: "octocat", Name: "Octo Cat", Email: "o@x.com", AvatarURL: "http://a"})
	}))
	defer ts.Close()

	g := &GitHubOAuth{httpClient: &http.Client{Transport: stubTransport{ts}}}
	u, err := g.GetUser("tok")
	if err != nil {
		t.Fatal(err)
	}
	if u.Login != "octocat" || u.ID != 42 {
		t.Errorf("user wrong: %+v", u)
	}
}

func TestGetUserDecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer ts.Close()

	g := &GitHubOAuth{httpClient: &http.Client{Transport: stubTransport{ts}}}
	if _, err := g.GetUser("tok"); err == nil {
		t.Error("expected decode error")
	}
}
