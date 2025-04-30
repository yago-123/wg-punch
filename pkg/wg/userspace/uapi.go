package userspacewg

import (
	"encoding/hex"
	"fmt"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ConvertWgTypesToUAPI converts WireGuard configuration to a UAPI string format
func ConvertWgTypesToUAPI(cfg wgtypes.Config) (string, error) {
	var b strings.Builder

	if cfg.PrivateKey != nil {
		b.WriteString(fmt.Sprintf("private_key=%s\n", hex.EncodeToString(cfg.PrivateKey[:])))
	}

	if cfg.ListenPort != nil {
		b.WriteString(fmt.Sprintf("listen_port=%d\n", *cfg.ListenPort))
	}

	if cfg.ReplacePeers {
		b.WriteString("replace_peers=true\n")
	}

	for _, peer := range cfg.Peers {
		b.WriteString(fmt.Sprintf("public_key=%s\n", hex.EncodeToString(peer.PublicKey[:])))

		if peer.Endpoint != nil {
			b.WriteString(fmt.Sprintf("endpoint=%s\n", peer.Endpoint.String()))
		}

		if peer.ReplaceAllowedIPs {
			b.WriteString("replace_allowed_ips=true\n")
		}

		for _, ipNet := range peer.AllowedIPs {
			b.WriteString(fmt.Sprintf("allowed_ip=%s\n", ipNet.String()))
		}

		if peer.PersistentKeepaliveInterval != nil {
			b.WriteString(fmt.Sprintf("persistent_keepalive_interval=%d\n", int(peer.PersistentKeepaliveInterval.Seconds())))
		}

		b.WriteString("\n") // Separate peers
	}

	return b.String(), nil
}
