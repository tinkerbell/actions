package talosnet

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/tinkerbell/actions/talosmeta/hardware"
)

// Namer resolves the interface name Talos will use for a MAC address.
type Namer interface {
	Name(mac string) (string, error)
}

// FromHardware maps Tinkerbell Hardware data onto a Talos platform network
// configuration. Every interface with a static address becomes a link, an
// address and, when a gateway is set, a default route. Hostname, resolvers,
// time servers and external IPs are collected once for the machine.
func FromHardware(spec *hardware.Spec, namer Namer) (*Config, error) {
	cfg := newConfig()

	var (
		hostname, domain         string
		nameServers, timeServers []string
		configured               int
	)

	for i, iface := range spec.Interfaces {
		d := iface.DHCP
		if d == nil || d.IP == nil || d.IP.Address == "" {
			continue
		}

		if err := addInterface(cfg, d, namer); err != nil {
			return nil, fmt.Errorf("interface %d (%s): %w", i, d.MAC, err)
		}

		if hostname == "" {
			hostname = d.Hostname
		}

		if domain == "" {
			domain = d.DomainName
		}

		nameServers = appendUnique(nameServers, d.NameServers...)
		timeServers = appendUnique(timeServers, d.TimeServers...)
		configured++
	}

	if configured == 0 {
		return nil, errors.New("no interface with a static address (interfaces[].dhcp.ip) found in the hardware data")
	}

	if spec.Metadata != nil && spec.Metadata.Instance != nil {
		inst := spec.Metadata.Instance

		if inst.Hostname != "" {
			hostname = inst.Hostname
		}

		for _, ip := range inst.IPs {
			if !ip.Public || ip.Address == "" {
				continue
			}

			addr, err := netip.ParseAddr(ip.Address)
			if err != nil {
				return nil, fmt.Errorf("instance ip: parsing address %q: %w", ip.Address, err)
			}

			cfg.ExternalIPs = appendUnique(cfg.ExternalIPs, addr.String())
		}
	}

	if hostname != "" {
		host, dom := splitFQDN(hostname)
		if domain == "" {
			domain = dom
		}

		cfg.Hostnames = append(cfg.Hostnames, Hostname{Hostname: host, Domainname: domain, Layer: layerPlatform})
	}

	if len(nameServers) > 0 {
		cfg.Resolvers = append(cfg.Resolvers, Resolver{DNSServers: nameServers, Layer: layerPlatform})
	}

	if len(timeServers) > 0 {
		cfg.TimeServers = append(cfg.TimeServers, TimeServer{Servers: timeServers, Layer: layerPlatform})
	}

	return cfg, nil
}

func addInterface(cfg *Config, d *hardware.DHCP, namer Namer) error {
	linkName, err := resolveLinkName(d, namer)
	if err != nil {
		return err
	}

	addr, err := netip.ParseAddr(d.IP.Address)
	if err != nil {
		return fmt.Errorf("parsing address %q: %w", d.IP.Address, err)
	}

	family := familyOf(addr)

	prefixLen, err := PrefixLength(d.IP.Netmask, familyNumber(addr))
	if err != nil {
		return fmt.Errorf("netmask %q: %w", d.IP.Netmask, err)
	}

	cfg.Links = append(cfg.Links, Link{Name: linkName, Up: true, Layer: layerPlatform})

	attachTo := linkName

	if d.VLANID != "" {
		vid, err := strconv.ParseUint(d.VLANID, 10, 12)
		if err != nil {
			return fmt.Errorf("parsing vlan_id %q: %w", d.VLANID, err)
		}

		attachTo = fmt.Sprintf("%s.%d", linkName, vid)

		cfg.Links = append(cfg.Links, Link{
			Name:       attachTo,
			Logical:    true,
			Up:         true,
			Kind:       linkKindVLAN,
			Type:       linkTypeEther,
			ParentName: linkName,
			VLAN:       &VLAN{ID: uint16(vid), Protocol: vlanProtocol},
			Layer:      layerPlatform,
		})
	}

	cfg.Addresses = append(cfg.Addresses, Address{
		Address:  netip.PrefixFrom(addr, prefixLen).String(),
		LinkName: attachTo,
		Family:   family,
		Scope:    scopeGlobal,
		Flags:    flagsPermanent,
		Layer:    layerPlatform,
	})

	if d.IP.Gateway != "" {
		gw, err := netip.ParseAddr(d.IP.Gateway)
		if err != nil {
			return fmt.Errorf("parsing gateway %q: %w", d.IP.Gateway, err)
		}

		cfg.Routes = append(cfg.Routes, Route{
			Family:      family,
			Gateway:     gw.String(),
			OutLinkName: attachTo,
			Table:       tableMain,
			Priority:    defaultRoutePriority(addr),
			Scope:       scopeGlobal,
			Type:        routeUnicast,
			Protocol:    protocolStatic,
			Layer:       layerPlatform,
		})
	}

	for _, r := range d.ClasslessStaticRoutes {
		dst, err := netip.ParsePrefix(r.DestinationDescriptor)
		if err != nil {
			return fmt.Errorf("parsing static route destination %q: %w", r.DestinationDescriptor, err)
		}

		gw, err := netip.ParseAddr(r.Router)
		if err != nil {
			return fmt.Errorf("parsing static route router %q: %w", r.Router, err)
		}

		cfg.Routes = append(cfg.Routes, Route{
			Family:      familyOf(dst.Addr()),
			Destination: dst.Masked().String(),
			Gateway:     gw.String(),
			OutLinkName: attachTo,
			Table:       tableMain,
			Scope:       scopeGlobal,
			Type:        routeUnicast,
			Protocol:    protocolStatic,
			Layer:       layerPlatform,
		})
	}

	return nil
}

func resolveLinkName(d *hardware.DHCP, namer Namer) (string, error) {
	if d.IfaceName != "" {
		return d.IfaceName, nil
	}

	if d.MAC == "" {
		return "", errors.New("resolving link name: neither iface_name nor mac is set")
	}

	name, err := namer.Name(d.MAC)
	if err != nil {
		return "", fmt.Errorf("resolving link name: %w", err)
	}

	return name, nil
}

// PrefixLength converts a netmask to a prefix length. The netmask may be a
// dotted quad (255.255.255.0), an IPv6 mask, or a bare prefix length with or
// without a leading slash. family is 4 or 6.
func PrefixLength(netmask string, family int64) (int, error) {
	netmask = strings.TrimPrefix(strings.TrimSpace(netmask), "/")
	if netmask == "" {
		return 0, errors.New("netmask is empty")
	}

	maxBits := 32
	if family == 6 {
		maxBits = 128
	}

	if n, err := strconv.Atoi(netmask); err == nil {
		if n < 0 || n > maxBits {
			return 0, fmt.Errorf("prefix length %d is out of range for IPv%d", n, family)
		}

		return n, nil
	}

	addr, err := netip.ParseAddr(netmask)
	if err != nil {
		return 0, fmt.Errorf("netmask %q is neither a prefix length nor an address", netmask)
	}

	if addr.BitLen() != maxBits {
		return 0, fmt.Errorf("netmask %q does not match IPv%d", netmask, family)
	}

	ones, bits := net.IPMask(addr.AsSlice()).Size()
	if bits == 0 {
		return 0, fmt.Errorf("netmask %q is not contiguous", netmask)
	}

	return ones, nil
}

func familyOf(addr netip.Addr) string {
	if addr.Is4() {
		return familyInet4
	}

	return familyInet6
}

func familyNumber(addr netip.Addr) int64 {
	if addr.Is4() {
		return 4
	}

	return 6
}

func defaultRoutePriority(addr netip.Addr) uint32 {
	if addr.Is4() {
		return priorityInet4
	}

	return priorityInet6
}

// splitFQDN separates a host name from its domain at the first dot.
func splitFQDN(name string) (host, domain string) {
	host, domain, _ = strings.Cut(name, ".")

	return host, domain
}

func appendUnique(list []string, values ...string) []string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		duplicate := false

		for _, existing := range list {
			if existing == v {
				duplicate = true

				break
			}
		}

		if !duplicate {
			list = append(list, v)
		}
	}

	return list
}
