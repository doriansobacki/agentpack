package entra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// graphBaseURL is Microsoft Graph's production base URL.
const graphBaseURL = "https://graph.microsoft.com"

// memberOfPath lists the signed-in user's direct group memberships. Requires
// the User.Read delegated permission at minimum; tenants with strict consent
// policies may additionally need GroupMember.Read.All.
const memberOfPath = "/v1.0/me/memberOf?$select=id,displayName"

// fetchGraphGroups reads the user's group memberships from Microsoft Graph,
// following @odata.nextLink pagination, and returns object IDs and display
// names. Non-group directory objects in the response (e.g. directory roles)
// are included too; unmatched manifest keys are harmless.
func fetchGraphGroups(ctx context.Context, baseURL, accessToken string) (ids, names []string, err error) {
	url := baseURL + memberOfPath
	for url != "" {
		var page struct {
			NextLink string `json:"@odata.nextLink"`
			Value    []struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"value"`
		}
		if err := graphGet(ctx, url, accessToken, &page); err != nil {
			return nil, nil, err
		}
		for _, obj := range page.Value {
			if obj.ID != "" {
				ids = append(ids, obj.ID)
			}
			if obj.DisplayName != "" {
				names = append(names, obj.DisplayName)
			}
		}
		url = page.NextLink
	}
	return ids, names, nil
}

// graphGet performs one authenticated Graph request and decodes the JSON body.
func graphGet(ctx context.Context, url, accessToken string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GET %s: %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
