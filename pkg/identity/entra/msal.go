package entra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

// client abstracts the token-acquiring operations the provider needs, keeping
// the MSAL specifics behind one seam so unit tests can substitute a fake (and
// future IdP providers can copy the shape).
type client interface {
	// AcquireSilent returns a token from the cache, refreshing it if needed.
	// It errors when no cached account can satisfy the request.
	AcquireSilent(ctx context.Context, scopes []string) (authResult, error)
	// AcquireDeviceCode runs the device authorization grant. prompt is called
	// once with the user-facing instruction (verification URL + user code)
	// before blocking until the user completes sign-in or ctx is done.
	AcquireDeviceCode(ctx context.Context, scopes []string, prompt func(message string)) (authResult, error)
}

// authResult is the subset of an authentication result the provider consumes.
type authResult struct {
	// AccessToken authorizes Microsoft Graph calls.
	AccessToken string
	// IDTokenRaw is the compact-serialized ID token carrying the user's
	// email and groups claims.
	IDTokenRaw string
}

// msalClient implements client on MSAL for Go's public-client application.
type msalClient struct {
	app public.Client
}

// newMSALClient builds an MSAL public client for the tenant, persisting its
// token cache at cachePath.
func newMSALClient(tenantID, clientID, cachePath string) (client, error) {
	app, err := public.New(clientID,
		public.WithAuthority("https://login.microsoftonline.com/"+tenantID),
		public.WithCache(&fileCache{path: cachePath}),
	)
	if err != nil {
		return nil, err
	}
	return &msalClient{app: app}, nil
}

// AcquireSilent implements client.
func (c *msalClient) AcquireSilent(ctx context.Context, scopes []string) (authResult, error) {
	accounts, err := c.app.Accounts(ctx)
	if err != nil {
		return authResult{}, err
	}
	if len(accounts) == 0 {
		return authResult{}, fmt.Errorf("no account in the token cache")
	}
	res, err := c.app.AcquireTokenSilent(ctx, scopes, public.WithSilentAccount(accounts[0]))
	if err != nil {
		return authResult{}, err
	}
	return authResult{AccessToken: res.AccessToken, IDTokenRaw: res.IDToken.RawToken}, nil
}

// AcquireDeviceCode implements client.
func (c *msalClient) AcquireDeviceCode(ctx context.Context, scopes []string, prompt func(message string)) (authResult, error) {
	code, err := c.app.AcquireTokenByDeviceCode(ctx, scopes)
	if err != nil {
		return authResult{}, err
	}
	prompt(code.Result.Message)
	res, err := code.AuthenticationResult(ctx)
	if err != nil {
		return authResult{}, err
	}
	return authResult{AccessToken: res.AccessToken, IDTokenRaw: res.IDToken.RawToken}, nil
}

// fileCache persists MSAL's token cache as a plain file with owner-only
// permissions, so sync stays non-interactive after the first login.
//
// TODO: encrypt at rest via the OS keychain/DPAPI where available; plain
// 0600 is the portable baseline until that follow-up lands.
type fileCache struct {
	path string
}

// Replace implements cache.ExportReplace: it loads the persisted cache into
// MSAL. A missing file is not an error — it just means no one signed in yet.
func (c *fileCache) Replace(_ context.Context, cache cache.Unmarshaler, _ cache.ReplaceHints) error {
	data, err := os.ReadFile(c.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return cache.Unmarshal(data)
}

// Export implements cache.ExportReplace: it writes MSAL's cache to disk.
func (c *fileCache) Export(_ context.Context, cache cache.Marshaler, _ cache.ExportHints) error {
	data, err := cache.Marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}
