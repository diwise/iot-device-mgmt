package client

import (
	"context"
	"fmt"
	"net/http"
)

func (dmc *devManagementClient) doRequestWithTokenRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	if dmc.clientCredentials != nil {
		token, err := dmc.refreshToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
		}

		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	drainAndCloseResponseBody(resp)
	dmc.invalidateTokenCache()

	if dmc.clientCredentials != nil {
		token, err := dmc.refreshToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
		}

		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err = dmc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)
	return resp, nil
}
