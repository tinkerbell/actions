// Package hardware retrieves the Tinkerbell Hardware data for the machine an
// action runs on. It follows the pattern of the rootio action: an HTTP GET of
// /metadata on the Tinkerbell metadata service, which answers with the
// Hardware spec of the machine that made the request.
package hardware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Spec mirrors the parts of the Tinkerbell Hardware spec that describe
// networking. Field names and JSON tags follow the Hardware CRD.
type Spec struct {
	Interfaces []Interface `json:"interfaces,omitempty"`
	Metadata   *Metadata   `json:"metadata,omitempty"`
}

// Interface is one network interface of the machine.
type Interface struct {
	DHCP        *DHCP `json:"dhcp,omitempty"`
	DisableDHCP bool  `json:"disableDhcp,omitempty"`
}

// DHCP holds the addressing Tinkerbell hands out for an interface.
type DHCP struct {
	MAC                   string                 `json:"mac,omitempty"`
	Hostname              string                 `json:"hostname,omitempty"`
	DomainName            string                 `json:"domain_name,omitempty"`
	NameServers           []string               `json:"name_servers,omitempty"`
	TimeServers           []string               `json:"time_servers,omitempty"`
	IfaceName             string                 `json:"iface_name,omitempty"`
	IP                    *IP                    `json:"ip,omitempty"`
	VLANID                string                 `json:"vlan_id,omitempty"`
	ClasslessStaticRoutes []ClasslessStaticRoute `json:"classless_static_routes,omitempty"`
}

// IP is a static address assignment.
type IP struct {
	Address string `json:"address,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	Family  int64  `json:"family,omitempty"`
}

// ClasslessStaticRoute is a DHCP option 121 route.
type ClasslessStaticRoute struct {
	DestinationDescriptor string `json:"destination_descriptor"`
	Router                string `json:"router"`
}

// Metadata is the Hardware metadata block.
type Metadata struct {
	Instance *Instance `json:"instance,omitempty"`
}

// Instance describes the provisioned instance.
type Instance struct {
	ID       string       `json:"id,omitempty"`
	Hostname string       `json:"hostname,omitempty"`
	IPs      []InstanceIP `json:"ips,omitempty"`
}

// InstanceIP is an address listed on the instance.
type InstanceIP struct {
	Address    string `json:"address,omitempty"`
	Netmask    string `json:"netmask,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
	Family     int64  `json:"family,omitempty"`
	Public     bool   `json:"public,omitempty"`
	Management bool   `json:"management,omitempty"`
}

const (
	metadataPath = "/metadata"
	userAgent    = "talosmeta"
	maxBody      = 8 << 20
)

// Fetch retrieves the Hardware spec from the metadata service rooted at
// baseURL, for example http://192.168.1.2:7080.
func Fetch(ctx context.Context, client *http.Client, baseURL string) (*Spec, error) {
	url := strings.TrimRight(baseURL, "/") + metadataPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building metadata request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("reading metadata response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata service %s returned %s: %s", url, res.Status, strings.TrimSpace(string(body)))
	}

	var spec Spec
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("decoding metadata from %s: %w", url, err)
	}

	return &spec, nil
}
