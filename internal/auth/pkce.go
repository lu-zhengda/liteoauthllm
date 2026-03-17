package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// GeneratePKCE returns a random verifier and its S256 challenge per RFC 7636.
func GeneratePKCE() (verifier string, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}

	verifier = base64.RawURLEncoding.EncodeToString(buf)

	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])

	return verifier, challenge, nil
}

// GenerateState returns a cryptographically random state parameter for OAuth.
func GenerateState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// WaitForCallback starts a local HTTP server on the given port and waits for
// the OAuth redirect. It validates the state parameter and returns the code.
func WaitForCallback(port int, expectedState string) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		if state != expectedState {
			errCh <- fmt.Errorf("state mismatch")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}

		// Check for OAuth error response
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			errCh <- fmt.Errorf("OAuth error: %s — %s", errMsg, desc)
			fmt.Fprintf(w, "<html><body><h2>Login failed</h2><p>%s: %s</p></body></html>", errMsg, desc)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback (query: %s)", r.URL.RawQuery)
			http.Error(w, "no code", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Login successful!</h2><p>You can close this tab.</p></body></html>")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		codeCh <- code
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	go srv.ListenAndServe() //nolint:errcheck

	select {
	case code := <-codeCh:
		// Give the browser a moment to receive the response before shutting down
		time.Sleep(500 * time.Millisecond)
		srv.Close()
		return code, nil
	case err := <-errCh:
		srv.Close()
		return "", err
	case <-time.After(5 * time.Minute):
		srv.Close()
		return "", fmt.Errorf("login timed out after 5 minutes — please try again")
	}
}

// ExchangeCode exchanges an authorization code for tokens using the token
// endpoint, presenting the PKCE verifier to prove possession.
func ExchangeCode(tokenURL, clientID, code, verifier, redirectURI string) (Token, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return Token{}, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Token{}, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return Token{}, fmt.Errorf("parsing token response: %w", err)
	}

	return Token{
		Version:      1,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix(),
	}, nil
}
