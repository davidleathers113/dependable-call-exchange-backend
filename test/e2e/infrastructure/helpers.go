package infrastructure

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// APIClient provides helpers for making API requests
type APIClient struct {
	BaseURL string
	Token   string
	t       *testing.T
}

// NewAPIClient creates a new API client
func NewAPIClient(t *testing.T, baseURL string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		t:       t,
	}
}

// SetToken sets the authentication token
func (c *APIClient) SetToken(token string) {
	c.Token = token
}

// Post makes a POST request
func (c *APIClient) Post(path string, payload interface{}) *http.Response {
	body, err := json.Marshal(payload)
	require.NoError(c.t, err)

	req, err := http.NewRequest("POST", c.BaseURL+path, bytes.NewBuffer(body))
	require.NoError(c.t, err)

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Get makes a GET request
func (c *APIClient) Get(path string) *http.Response {
	req, err := http.NewRequest("GET", c.BaseURL+path, nil)
	require.NoError(c.t, err)

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Put makes a PUT request
func (c *APIClient) Put(path string, payload interface{}) *http.Response {
	body, err := json.Marshal(payload)
	require.NoError(c.t, err)

	req, err := http.NewRequest("PUT", c.BaseURL+path, bytes.NewBuffer(body))
	require.NoError(c.t, err)

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Delete makes a DELETE request
func (c *APIClient) Delete(path string) *http.Response {
	req, err := http.NewRequest("DELETE", c.BaseURL+path, nil)
	require.NoError(c.t, err)

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Patch makes a PATCH request
func (c *APIClient) Patch(path string, payload interface{}) *http.Response {
	body, err := json.Marshal(payload)
	require.NoError(c.t, err)

	req, err := http.NewRequest("PATCH", c.BaseURL+path, bytes.NewBuffer(body))
	require.NoError(c.t, err)

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// DecodeResponse decodes JSON response
func (c *APIClient) DecodeResponse(resp *http.Response, v interface{}) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)

	if resp.StatusCode >= 400 {
		c.t.Logf("Error response: %s", string(body))
	}

	err = json.Unmarshal(body, v)
	require.NoError(c.t, err)
}

// WebSocketClient provides WebSocket testing utilities
type WebSocketClient struct {
	URL  string
	Conn *websocket.Conn
	t    *testing.T
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(t *testing.T, wsURL string) *WebSocketClient {
	return &WebSocketClient{
		URL: wsURL,
		t:   t,
	}
}

// Connect establishes WebSocket connection
func (w *WebSocketClient) Connect(clientID string) error {
	u, err := url.Parse(w.URL)
	if err != nil {
		return err
	}

	q := u.Query()
	q.Set("client_id", clientID)
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	w.Conn = conn
	return nil
}

// Close closes the WebSocket connection
func (w *WebSocketClient) Close() {
	if w.Conn != nil {
		w.Conn.Close()
	}
}

// Send sends a message
func (w *WebSocketClient) Send(msg interface{}) error {
	return w.Conn.WriteJSON(msg)
}

// Receive receives a message
func (w *WebSocketClient) Receive(v interface{}) error {
	return w.Conn.ReadJSON(v)
}
