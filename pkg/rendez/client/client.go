package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/yago-123/wg-punch/pkg/rendez/types"
)

const RendezvousClientTimeout = 5 * time.Second

type Rendezvous interface {
	Register(ctx context.Context, req types.RegisterRequest) error
	Discover(ctx context.Context, peerID string) (*types.PeerResponse, *net.UDPAddr, error)
	WaitForPeer(ctx context.Context, peerID string, interval time.Duration) (*types.PeerResponse, *net.UDPAddr, error)
}

type Client struct {
	baseURL string
	client  *http.Client
}

func NewRendezvous(baseURL string) Rendezvous {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: RendezvousClientTimeout},
	}
}

// Register registers a peer with the rendez server
func (c *Client) Register(ctx context.Context, req types.RegisterRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal register request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/register", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send register request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register failed with status: %s", resp.Status)
	}

	return nil
}

// Discover retrieves the peer information from the rendezvous server
func (c *Client) Discover(ctx context.Context, peerID string) (*types.PeerResponse, *net.UDPAddr, error) {
	url := fmt.Sprintf("%s/peer/%s", c.baseURL, peerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create discover request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("send discover request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("discover failed with status: %s", resp.Status)
	}

	var peerResp types.PeerResponse
	if errJSON := json.NewDecoder(resp.Body).Decode(&peerResp); errJSON != nil {
		return nil, nil, fmt.Errorf("decode response: %w", errJSON)
	}

	// Convert endpoint into UDP address
	udpAddr, err := net.ResolveUDPAddr("udp", peerResp.Endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid endpoint in response: %w", err)
	}

	return &peerResp, udpAddr, nil
}

func (c *Client) WaitForPeer(ctx context.Context, peerID string, interval time.Duration) (*types.PeerResponse, *net.UDPAddr, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-ticker.C:
			res, addr, err := c.Discover(ctx, peerID)
			if err == nil && res != nil && addr != nil {

				udpAddr, errUDP := net.ResolveUDPAddr("udp", res.Endpoint)
				if errUDP != nil {
					return nil, nil, fmt.Errorf("invalid endpoint in response: %w", err)
				}
				return &types.PeerResponse{
					PublicKey:  res.PublicKey,
					AllowedIPs: res.AllowedIPs,
					Endpoint:   addr.String(),
				}, udpAddr, nil
			}
		}
	}
}
