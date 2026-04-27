package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/vpnplatform/internal/config"
)

// isTransientNetErr reports whether err is the kind of network-level error
// that's worth retrying: timeouts, connection-resets, DNS hiccups, etc.
// 4xx/5xx HTTP responses are NOT transient — those reach the caller as
// nil err + non-2xx status and are handled separately.
func isTransientNetErr(err error) bool {
	if err == nil {
		return false
	}
	var nerr net.Error
	if errors.As(err, &nerr) && (nerr.Timeout() || nerr.Temporary()) { //nolint:staticcheck // Temporary is fine here
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	// io.EOF on idle keep-alive connection — retry once.
	if errors.Is(err, io.EOF) {
		return true
	}
	return false
}

type CreateUserRequest struct {
	Username             string    `json:"username"`
	TrafficLimitBytes    int64     `json:"trafficLimitBytes"`
	ExpireAt             time.Time `json:"expireAt"`
	ActiveInternalSquads []string  `json:"activeInternalSquads,omitempty"`
}

// UserTraffic holds per-user traffic counters returned inside the user response.
type UserTraffic struct {
	UsedTrafficBytes         int64 `json:"usedTrafficBytes"`
	LifetimeUsedTrafficBytes int64 `json:"lifetimeUsedTrafficBytes"`
}

type UserResponse struct {
	UUID              string      `json:"uuid"`
	Username          string      `json:"username"`
	Status            string      `json:"status"`
	ExpireAt          time.Time   `json:"expireAt"`
	SubscribeURL      string      `json:"subscriptionUrl"`
	TrafficLimitBytes int64       `json:"trafficLimitBytes"`
	UserTraffic       UserTraffic `json:"userTraffic"`
}

type UpdateUserRequest struct {
	UUID                 string     `json:"uuid,omitempty"`
	ExpireAt             *time.Time `json:"expireAt,omitempty"`
	Status               *string    `json:"status,omitempty"`
	HwidDeviceLimit      *int       `json:"hwidDeviceLimit,omitempty"`
	ActiveInternalSquads []string   `json:"activeInternalSquads,omitempty"`
}

// HwidDevice represents a single HWID device from the Remnawave panel.
type HwidDevice struct {
	Hwid        string  `json:"hwid"`
	UserUUID    string  `json:"userUuid"`
	Platform    *string `json:"platform"`
	OsVersion   *string `json:"osVersion"`
	DeviceModel *string `json:"deviceModel"`
	UserAgent   *string `json:"userAgent"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type HwidDevicesResponse struct {
	Total   int          `json:"total"`
	Devices []HwidDevice `json:"devices"`
}

type DeleteHwidDeviceRequest struct {
	Hwid     string `json:"hwid"`
	UserUUID string `json:"userUuid"`
}

type Client struct {
	httpClient *http.Client
	cfg        config.RemnaConfig
	log        *zap.Logger
}

func NewClient(cfg config.RemnaConfig, log *zap.Logger) *Client {
	// Strip trailing slash to prevent double-slash in URLs.
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			// Do not follow redirects — API calls should never redirect.
			// If the panel is behind a CDN/proxy that redirects, we want to
			// know about it immediately rather than silently getting HTML.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cfg: cfg,
		log: log,
	}
}

// Ping performs a lightweight connectivity check against the Remnawave panel.
// Returns nil if the panel is reachable and authenticates the API token.
func (c *Client) Ping(ctx context.Context) error {
	// GET /api/users is a low-cost endpoint that requires auth.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/api/system/health", nil)
	if err != nil {
		return fmt.Errorf("remnawave ping: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("remnawave ping: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remnawave ping: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) CreateUser(ctx context.Context, username string, expireAt time.Time) (*UserResponse, error) {
	// activeInternalSquads expects SQUAD UUIDs, not inbound UUIDs.
	var squads []string
	if c.cfg.SquadUUID != "" {
		squads = []string{c.cfg.SquadUUID}
	}
	req := CreateUserRequest{Username: username, TrafficLimitBytes: 0, ExpireAt: expireAt, ActiveInternalSquads: squads}
	var resp UserResponse
	if err := c.do(ctx, http.MethodPost, "/api/users", req, &resp); err != nil {
		return nil, fmt.Errorf("remnawave create user: %w", err)
	}
	// Add all users (including the new one) to the configured squad via the
	// bulk-actions endpoint which triggers an async event on the server.
	if c.cfg.SquadUUID != "" && resp.UUID != "" {
		_ = c.AddAllUsersToSquad(ctx, c.cfg.SquadUUID)
	}
	return &resp, nil
}

func (c *Client) GetUser(ctx context.Context, remnaUUID string) (*UserResponse, error) {
	var resp UserResponse
	if err := c.do(ctx, http.MethodGet, "/api/users/"+remnaUUID, nil, &resp); err != nil {
		return nil, fmt.Errorf("remnawave get user: %w", err)
	}
	return &resp, nil
}

// GetUserByUsername looks up a Remnawave user by their username via the
// dedicated endpoint GET /api/users/by-username/{username}.
func (c *Client) GetUserByUsername(ctx context.Context, username string) (*UserResponse, error) {
	var resp UserResponse
	if err := c.do(ctx, http.MethodGet, "/api/users/by-username/"+username, nil, &resp); err != nil {
		return nil, fmt.Errorf("remnawave get user by username: %w", err)
	}
	if resp.UUID == "" {
		return nil, fmt.Errorf("remnawave: user %q not found", username)
	}
	return &resp, nil
}

// AddAllUsersToSquad triggers Remnawave to add ALL users to the given squad (async event).
// This is the correct API to ensure a newly created user inherits the squad's inbounds.
func (c *Client) AddAllUsersToSquad(ctx context.Context, squadUUID string) error {
	path := "/api/internal-squads/" + squadUUID + "/bulk-actions/add-users"
	if err := c.do(ctx, http.MethodPost, path, struct{}{}, nil); err != nil {
		return fmt.Errorf("remnawave add users to squad: %w", err)
	}
	return nil
}

func (c *Client) UpdateExpiry(ctx context.Context, remnaUUID string, newExpiry time.Time) error {
	req := UpdateUserRequest{UUID: remnaUUID, ExpireAt: &newExpiry}
	if err := c.do(ctx, http.MethodPatch, "/api/users", req, nil); err != nil {
		return fmt.Errorf("remnawave update expiry: %w", err)
	}
	return nil
}

func (c *Client) DisableUser(ctx context.Context, remnaUUID string) error {
	status := "DISABLED"
	if err := c.do(ctx, http.MethodPatch, "/api/users", UpdateUserRequest{UUID: remnaUUID, Status: &status}, nil); err != nil {
		return fmt.Errorf("remnawave disable user: %w", err)
	}
	return nil
}

func (c *Client) EnableUser(ctx context.Context, remnaUUID string) error {
	status := "ACTIVE"
	if err := c.do(ctx, http.MethodPatch, "/api/users", UpdateUserRequest{UUID: remnaUUID, Status: &status}, nil); err != nil {
		return fmt.Errorf("remnawave enable user: %w", err)
	}
	return nil
}

// UpdateHwidDeviceLimit sets the HWID device limit for a user in the Remnawave panel.
func (c *Client) UpdateHwidDeviceLimit(ctx context.Context, remnaUUID string, limit int) error {
	if err := c.do(ctx, http.MethodPatch, "/api/users", UpdateUserRequest{UUID: remnaUUID, HwidDeviceLimit: &limit}, nil); err != nil {
		return fmt.Errorf("remnawave update hwid device limit: %w", err)
	}
	return nil
}

// GetUserHwidDevices returns the list of HWID-tracked devices for the given Remnawave user.
func (c *Client) GetUserHwidDevices(ctx context.Context, remnaUUID string) (*HwidDevicesResponse, error) {
	var resp HwidDevicesResponse
	if err := c.do(ctx, http.MethodGet, "/api/hwid/devices/"+remnaUUID, nil, &resp); err != nil {
		return nil, fmt.Errorf("remnawave get user hwid devices: %w", err)
	}
	return &resp, nil
}

// DeleteUserHwidDevice removes a specific HWID device for a user.
func (c *Client) DeleteUserHwidDevice(ctx context.Context, hwid, remnaUUID string) error {
	req := DeleteHwidDeviceRequest{Hwid: hwid, UserUUID: remnaUUID}
	if err := c.do(ctx, http.MethodPost, "/api/hwid/devices/delete", req, nil); err != nil {
		return fmt.Errorf("remnawave delete user hwid device: %w", err)
	}
	return nil
}

// Marshalled request body cached so we can rebuild the request reader on
// each retry without losing data. The HTTP request consumes the body on
// first attempt, so a bare `bytes.NewReader(b)` would arrive empty on retry.
func (c *Client) do(ctx context.Context, method, path string, body, out interface{}) error {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyBytes = b
	}
	url := c.cfg.BaseURL + path

	const maxRetries = 2
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil {
			break
		}
		if !isTransientNetErr(lastErr) || attempt == maxRetries {
			c.log.Error("remnawave request failed",
				zap.String("method", method), zap.String("url", url),
				zap.Int("attempt", attempt+1), zap.Error(lastErr))
			return lastErr
		}
		// Backoff 200ms, 400ms; bounded by ctx.
		backoff := time.Duration(200*(1<<attempt)) * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		// Truncate body in logs — error responses may echo the request and
		// leak UUIDs / structured fields into managed log destinations
		// (Datadog/Loki). 500 chars is plenty for diagnostics.
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500] + "...(truncated)"
		}
		c.log.Warn("remnawave error response",
			zap.String("method", method),
			zap.String("url", url),
			zap.Int("status", resp.StatusCode),
			zap.String("body_preview", preview))
		return fmt.Errorf("remnawave: %s %s returned %d: %s", method, path, resp.StatusCode, preview)
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	// Remnawave wraps responses in {"response": {...}} envelope.
	// Try to unwrap first; if no envelope, unmarshal directly.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &envelope); err == nil {
		for _, key := range []string{"response", "data"} {
			if inner, ok := envelope[key]; ok {
				if err := json.Unmarshal(inner, out); err != nil {
					c.log.Warn("remnawave: failed to decode envelope inner",
						zap.String("key", key),
						zap.String("url", url),
						zap.Error(err))
					return fmt.Errorf("remnawave decode %s envelope: %w", key, err)
				}
				return nil
			}
		}
	}
	// No envelope — unmarshal directly.
	if err := json.Unmarshal(respBody, out); err != nil {
		c.log.Warn("remnawave: failed to decode response",
			zap.String("url", url),
			zap.String("body", string(respBody)),
			zap.Error(err))
		return fmt.Errorf("remnawave decode response: %w", err)
	}
	return nil
}
