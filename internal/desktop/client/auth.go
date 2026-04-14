package client

import (
	"context"
	"fmt"
)

// Login authenticates with username/password. The session cookie is stored
// automatically by the cookie jar and sent on all subsequent requests.
func (c *Client) Login(ctx context.Context, username, password string) error {
	body := map[string]string{"username": username, "password": password}
	resp, err := c.requestRaw(ctx, "POST", "/auth/login", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("login failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Logout invalidates the current session.
func (c *Client) Logout(ctx context.Context) error {
	resp, err := c.requestRaw(ctx, "POST", "/auth/logout", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetMe returns the currently authenticated user.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var u User
	if err := c.request(ctx, "GET", "/users/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}
