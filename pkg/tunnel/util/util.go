package util

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"net"
	"os"
)

// AssignAddressToIface assigns the internal IP address to the WireGuard interface in CIDR notation in order to allow
// communications between peers
// todo(): move addrCIDR to a native type like Addr?
func AssignAddressToIface(iface, addrCIDR string) error {
	// Lookup interface link by name
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", iface, err)
	}

	// Parse address CIDR to assign to the interface
	addr, err := netlink.ParseAddr(addrCIDR)
	if err != nil {
		return fmt.Errorf("failed to parse address %s: %w", addrCIDR, err)
	}

	// todo(): move this into a separate function
	// Check if the address already exists on the interface
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("failed to list addresses on %s: %w", iface, err)
	}

	for _, a := range existingAddrs {
		if a.IP.Equal(addr.IP) && a.Mask.String() == addr.Mask.String() {
			return nil // already exists, don't reassign
		}
	}

	// Assign address to the interface
	if errAddr := netlink.AddrAdd(link, addr); errAddr != nil {
		return fmt.Errorf("failed to assign address: %w", errAddr)
	}

	return nil
}

// AddPeerRoutes adds the allowed IPs of the peer to the WireGuard interface so that the kernel can route packets
func AddPeerRoutes(iface string, allowedIPs []net.IPNet) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", iface, err)
	}

	for _, ipNet := range allowedIPs {
		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       &ipNet,
		}

		// Try to add the route, but don't fail if it already exists
		if errRoute := netlink.RouteAdd(route); errRoute != nil && !os.IsExist(errRoute) {
			return fmt.Errorf("failed to add route %s: %w", ipNet.String(), errRoute)
		}
	}

	return nil
}
