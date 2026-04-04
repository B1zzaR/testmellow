package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vpnplatform/internal/config"
)

type CreateUserRequest struct {
	Username           string       `json:"username"`
	TrafficLimitBytes  int64        `json:"trafficLimitBytes"`
	ExpireAt           time.Time    `json:"expireAt"`
	ActiveUserInbounds []InboundRef `json:"activeUserInbounds"`
}

type InboundRef struct {
	UUID string `json:"uuid"`
}

type UserResponse struct {
	UUID         string    `json:"uuid"`
	Username     string    `json:"username"`
	Status       string    `json:"status"`
	ExpireAt     time.Time `json:"expireAt"`
	SubscribeURL string    `json:"subscriptionUrl"`
}

type UpdateUserRequest struct {
	UUID               string       `json:"uuid,omitempty"`
	ExpireAt           *time.Time   `json:"expireAt,omitempty"`
	Status             *string      `json:"status,omitempty"`
	ActiveUserInbounds []InboundRef `json:"activeUserInbounds,omitempty"`
}

type Client struct {
	httpClient *http.Client
	cfg        config.RemnaConfig
}

func NewClient(cfg config.RemnaConfig) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
	}
}

// GetSquadInbounds returns the inbound UUIDs belonging to the configured squad.
// Returns nil if no squad UUID is configured.
func (c *Client) GetSquadInbounds(ctx context.Context) ([]InboundRef, error) {
	if c.cfg.SquadUUID == "" {
		return nil, nil
	}
	type squadInbound struct {
		UUID string `json:"uuid"`
	}
	type squadEntry struct {
		UUID     string         `json:"uuid"`
		Inbounds []squadInbound `json:"inbounds"`
	}
	type squadList struct {
		Total          int          `json:"total"`
		InternalSquads []squadEntry `json:"internalSquads"`
	}
	var raw struct {
		Response *squadList `json:"response"`
		Data     *squadList `json:"data"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/api/internal-squads", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("remnawave: GET /api/internal-squads returned %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("remnawave get squads decode: %w", err)
	}
	list := raw.Response
	if list == nil {
		list = raw.Data
	}
	if list == nil {
		return nil, fmt.Errorf("remnawave: empty squads response")
	}
	for _, s := range list.InternalSquads {
		if s.UUID == c.cfg.SquadUUID {
			refs := make([]InboundRef, len(s.Inbounds))
			for i, ib := range s.Inbounds {
				refs[i] = InboundRef{UUID: ib.UUID}
			}
			return refs, nil
		}
	}
	return nil, fmt.Errorf("remnawave: squad %q not found", c.cfg.SquadUUID)
}

func (c *Client) CreateUser(ctx context.Context, username string, expireAt time.Time) (*UserResponse, error) {
	inbounds, _ := c.GetSquadInbounds(ctx)
	req := CreateUserRequest{Username: username, TrafficLimitBytes: 0, ExpireAt: expireAt, ActiveUserInbounds: inbounds}
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

// SetUserInbounds updates the user's activeUserInbounds, which assigns squad membership.
func (c *Client) SetUserInbounds(ctx context.Context, remnaUUID string, inbounds []InboundRef) error {
	req := UpdateUserRequest{UUID: remnaUUID, ActiveUserInbounds: inbounds}
	if err := c.do(ctx, http.MethodPatch, "/api/users", req, nil); err != nil {
		return fmt.Errorf("remnawave set inbounds: %w", err)
	}
	return nil
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
	status := "disabled"
	if err := c.do(ctx, http.MethodPatch, "/api/users", UpdateUserRequest{UUID: remnaUUID, Status: &status}, nil); err != nil {
		return fmt.Errorf("remnawave disable user: %w", err)
	}
	return nil
}

func (c *Client) EnableUser(ctx context.Context, remnaUUID string) error {
	status := "active"
	if err := c.do(ctx, http.MethodPatch, "/api/users", UpdateUserRequest{UUID: remnaUUID, Status: &status}, nil); err != nil {
		return fmt.Errorf("remnawave enable user: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remnawave: %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	// Try direct unmarshal first; if the target field is zero, try the common
	// Remnawave envelope formats: {"response":{...}} and {"data":{...}}.
	if err := json.Unmarshal(respBody, out); err != nil {
		return err
	}
	// Check if the result looks empty by trying wrapped variants.
	// We do this by checking a raw map for an "response" or "data" key.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &envelope); err == nil {
		for _, key := range []string{"response", "data"} {
			if inner, ok := envelope[key]; ok {
				_ = json.Unmarshal(inner, out)
				break
			}
		}
	}
	return nil
}
