package sdk

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

type Client struct {
	url      string
	username string
	password string
	client   *http.Client
	log      hclog.Logger
}

// The timeout for the REST client requests.
const timeout = 60

func NewClient(log hclog.Logger) *Client {
	return &Client{
		client: &http.Client{
			Timeout: timeout * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},

				// Values copied from http.DefaultTransport
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		log: log,
	}
}

func (c *Client) Initialise(url string, username string, password string) {
	c.url = strings.TrimSuffix(url, "/")
	c.username = username
	c.password = password
}

func (c *Client) Close() error {
	c.client.CloseIdleConnections()
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

	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to perform request %s %s: %w", method, path, err)
	}

	defer exhaustCloseWithLogOnError(c.log, res.Body)

	if res.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("unable to perform request %s %s (%d): %w", method, path, res.StatusCode, err)
		}
		return &HttpError{
			method: method,
			path:   path,
			status: res.StatusCode,
			body:   strings.TrimSpace(string(body)),
		}
	}

	if responseBody != nil {
		if err := json.NewDecoder(res.Body).Decode(responseBody); err != nil {
			return fmt.Errorf("unable to decode response %s %s: %w", method, path, err)
		}
	}

	return nil
}

// exhaustCloseWithLogOnError completely drains an io.ReadCloser, such as the body of an http.Response. Draining and
// closing the response body is important to allow the connection to be reused.
//
// From the documentation of the body field on http.Response:
//   The default HTTP client's Transport may not reuse HTTP/1.x "keep-alive" TCP connections if the Body is not read to completion and closed.
func exhaustCloseWithLogOnError(log hclog.Logger, r io.ReadCloser) {
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		log.Warn("failed to exhaust reader, performance may be impacted", "err", err)
	}

	err := r.Close()
	if err == nil || errors.Is(err, os.ErrClosed) {
		return
	}

	log.Warn("failed to close reader", "err", err)
}
