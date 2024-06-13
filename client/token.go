package client

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
)

func GetToken(authUrl string, key string, uid string, uname string, room string, role string, autoJoin bool) (string, error) {
	resp, err := http.Get(fmt.Sprintf(
		"%s/token?key=%s&uid=%s&uname=%s&room=%s&role=%s&nonce=%d&autojoin=%t",
		authUrl,
		url.QueryEscape(key),
		url.QueryEscape(uid),
		url.QueryEscape(uname),
		url.QueryEscape(room),
		url.QueryEscape(role),
		rand.Intn(100000),
		autoJoin,
	))
	if err != nil {
		return "", fmt.Errorf("unable to get token, %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unable to get token, got status code %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to get token, %w", err)
	}
	token := string(bodyBytes)
	return token, nil
}