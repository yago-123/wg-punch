package types

type RegisterRequest struct {
	PeerID     string   `json:"peer_id" example:"peer1"`
	PublicKey  string   `json:"public_key" example:"abc123..."`
	Endpoint   string   `json:"endpoint" example:"203.0.113.1:51820"`
	AllowedIPs []string `json:"allowed_ips" example:"[\"10.0.0.1/32\"]"`
}

type PeerResponse struct {
	PeerID     string   `json:"peer_id" example:"peer1"`
	PublicKey  string   `json:"public_key" example:"abc123..."`
	Endpoint   string   `json:"endpoint" example:"203.0.113.1:51820"`
	AllowedIPs []string `json:"allowed_ips" example:"[\"10.0.0.1/32\"]"`
}
