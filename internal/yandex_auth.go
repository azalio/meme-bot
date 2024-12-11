package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type OAuth2Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type IAMTokenResponse struct {
	IAMToken string `json:"iamToken"`
}

// RefreshIAMToken refreshes the IAM token using the Yandex OAuth token.
func RefreshIAMToken(oauthToken string) (string, error) {
	// IAM token exchange endpoint
	iamTokenURL := "https://iam.api.cloud.yandex.net/iam/v1/tokens"

	// Create request body
	requestBody := map[string]string{
		"yandexPassportOauthToken": oauthToken,
	}
	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshalling request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", iamTokenURL, bytes.NewBuffer(requestBodyJSON))
	if err != nil {
		return "", fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Make HTTP request
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status code: %d", resp.StatusCode)
	}

	var iamTokenResponse IAMTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&iamTokenResponse)
	if err != nil {
		return "", fmt.Errorf("decoding IAM token response: %w", err)
	}

	return iamTokenResponse.IAMToken, nil
}

// GetIAMToken retrieves or refreshes the IAM token.  Uses OAuth token from .env
func GetIAMToken() (string, error) {
	oauthToken := os.Getenv("YANDEX_OAUTH_TOKEN")
	if oauthToken == "" {
		return "", fmt.Errorf("YANDEX_OAUTH_TOKEN environment variable not set")
	}

	iamToken, err := RefreshIAMToken(oauthToken)
	if err != nil {
		return "", fmt.Errorf("getting IAM token: %w", err)
	}

	return iamToken, nil

}
