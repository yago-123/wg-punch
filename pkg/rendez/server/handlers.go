package server

import (
	"net"
	"net/http"

	"github.com/yago-123/wg-punch/pkg/rendez"

	"github.com/gin-gonic/gin"
	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/rendez/store"
)

type Handler struct {
	store store.Store
}

func NewHandler(s store.Store) *Handler {
	return &Handler{store: s}
}

// RegisterHandler godoc
// @Summary      Register a peer
// @Description  Registers a peer's public key, endpoint, and allowed IPs for NAT traversal
// @Tags         rendezvous
// @Accept       json
// @Produce      json
// @Param        registerRequest body RegisterRequest true "Peer registration info"
// @Success      200  {string}  string "ok"
// @Failure      400  {string}  string "invalid request body or endpoint or CIDR"
// @Failure      500  {string}  string "failed to register peer"
// @Router       /register [post]
func (h *Handler) RegisterHandler(c *gin.Context) {
	var req rendez.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "invalid request body")
		return
	}

	// Convert endpoint string to UDP address
	udpAddr, err := net.ResolveUDPAddr("udp", req.Endpoint)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid endpoint")
		return
	}

	var allowed []net.IPNet
	for _, cidr := range req.AllowedIPs {
		// Parse allowed IP CIDR
		_, ipnet, errCIDR := net.ParseCIDR(cidr)
		if errCIDR != nil {
			c.String(http.StatusBadRequest, "invalid CIDR in allowed_ips")
			return
		}
		allowed = append(allowed, *ipnet)
	}

	info := peer.Info{
		PublicKey:  req.PublicKey,
		Endpoint:   udpAddr,
		AllowedIPs: allowed,
	}

	if errStore := h.store.Register(req.PeerID, info); errStore != nil {
		c.String(http.StatusInternalServerError, "failed to register peer")
		return
	}

	c.String(http.StatusOK, "ok")
}

// LookupHandler godoc
// @Summary      Look up a peer
// @Description  Fetch peer information by PeerID
// @Tags         rendezvous
// @Produce      json
// @Param        peer_id path string true "Peer ID"
// @Success      200 {object} PeerResponse
// @Failure      404 {string} string "peer not found"
// @Router       /peer/{peer_id} [get]
func (h *Handler) LookupHandler(c *gin.Context) {
	peerID := c.Param("peer_id")

	info, ok := h.store.Lookup(peerID)
	if !ok {
		c.String(http.StatusNotFound, "peer not found")
		return
	}

	resp := rendez.PeerResponse{
		PeerID:     peerID,
		PublicKey:  info.PublicKey,
		Endpoint:   info.Endpoint.String(),
		AllowedIPs: make([]string, len(info.AllowedIPs)),
	}
	for i, ipnet := range info.AllowedIPs {
		resp.AllowedIPs[i] = ipnet.String()
	}

	c.JSON(http.StatusOK, resp)
}
