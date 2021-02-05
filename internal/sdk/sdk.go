package sdk

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	url      string
	username string
	password string
	Client   *http.Client
}

// The timeout for the REST client requests.
const timeout = 60

func NewClient(url string, username string, password string) *Client {
	return NewClientWithTransport(url, username, password, &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},)
}

func NewClientWithTransport(url string, username string, password string, transport *http.Transport) *Client {
	return &Client{
		url:      strings.TrimSuffix(url, "/"),
		username: username,
		password: password,
		Client: &http.Client{
			Timeout: timeout * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) Close() error {
	c.Client.CloseIdleConnections()
	return nil
}

func (c *Client) request(ctx context.Context, method string, path string, requestBody interface{}, responseBody interface{}) error {
	url := fmt.Sprintf("%s%s", c.url, path)

	requestBodyReader := &bytes.Buffer{}
	if requestBody != nil {
		if err := json.NewEncoder(requestBodyReader).Encode(requestBody); err != nil {
			return fmt.Errorf("unable to encode request body %s %s: %w", method, path, err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, requestBodyReader)
	if err != nil {
		return fmt.Errorf("unable to perform request %s %s: %w", method, path, err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json;charset=utf-8")
	}

	res, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to perform request %s %s - %w", method, path, err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("unable to perform request %s %s: %d - %w", method, path, res.StatusCode, err)
		}
		return fmt.Errorf("unable to perform request %s %s: %d - %s", method, path, res.StatusCode, body)
	}

	if responseBody != nil {
		if err := json.NewDecoder(res.Body).Decode(responseBody); err != nil {
			return fmt.Errorf("unable to decode response %s %s - %w", method, path, err)
		}
	}

	return nil
}
