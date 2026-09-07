// Package talosnet builds the Talos Linux metal platform network
// configuration document from Tinkerbell Hardware data.
//
// The document layout mirrors network.PlatformConfigSpec from
// github.com/siderolabs/talos/pkg/machinery, which is what the metal platform
// decodes from META key 0xa. Only the fields this action emits are modelled;
// Talos ignores absent fields.
package talosnet

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

const (
	layerPlatform  = "platform"
	familyInet4    = "inet4"
	familyInet6    = "inet6"
	scopeGlobal    = "global"
	flagsPermanent = "permanent"
	tableMain      = "main"
	routeUnicast   = "unicast"
	protocolStatic = "static"
	linkKindVLAN   = "vlan"
	linkTypeEther  = "ether"
	vlanProtocol   = "802.1q"

	// Talos gives IPv4 and IPv6 default routes from platform config these
	// priorities (see the metal network configuration example in the docs).
	priorityInet4 = 1024
	priorityInet6 = 2048
)

// Config is the platform network configuration document.
type Config struct {
	Addresses   []Address    `yaml:"addresses"`
	Links       []Link       `yaml:"links"`
	Routes      []Route      `yaml:"routes"`
	Hostnames   []Hostname   `yaml:"hostnames"`
	Resolvers   []Resolver   `yaml:"resolvers"`
	TimeServers []TimeServer `yaml:"timeServers"`
	Operators   []Operator   `yaml:"operators"`
	ExternalIPs []string     `yaml:"externalIPs"`
}

// Address mirrors network.AddressSpecSpec.
type Address struct {
	Address  string `yaml:"address"`
	LinkName string `yaml:"linkName"`
	Family   string `yaml:"family"`
	Scope    string `yaml:"scope"`
	Flags    string `yaml:"flags"`
	Layer    string `yaml:"layer"`
}

// Link mirrors the subset of network.LinkSpecSpec used here.
type Link struct {
	Name       string `yaml:"name"`
	Logical    bool   `yaml:"logical,omitempty"`
	Up         bool   `yaml:"up"`
	Kind       string `yaml:"kind,omitempty"`
	Type       string `yaml:"type,omitempty"`
	ParentName string `yaml:"parentName,omitempty"`
	VLAN       *VLAN  `yaml:"vlan,omitempty"`
	Layer      string `yaml:"layer"`
}

// VLAN mirrors network.VLANSpec.
type VLAN struct {
	ID       uint16 `yaml:"vlanID"`
	Protocol string `yaml:"vlanProtocol"`
}

// Route mirrors network.RouteSpecSpec.
type Route struct {
	Family      string `yaml:"family"`
	Destination string `yaml:"dst,omitempty"`
	Gateway     string `yaml:"gateway"`
	OutLinkName string `yaml:"outLinkName"`
	Table       string `yaml:"table"`
	Priority    uint32 `yaml:"priority,omitempty"`
	Scope       string `yaml:"scope"`
	Type        string `yaml:"type"`
	Protocol    string `yaml:"protocol"`
	Layer       string `yaml:"layer"`
}

// Hostname mirrors network.HostnameSpecSpec.
type Hostname struct {
	Hostname   string `yaml:"hostname"`
	Domainname string `yaml:"domainname,omitempty"`
	Layer      string `yaml:"layer"`
}

// Resolver mirrors network.ResolverSpecSpec.
type Resolver struct {
	DNSServers []string `yaml:"dnsServers"`
	Layer      string   `yaml:"layer"`
}

// TimeServer mirrors network.TimeServerSpecSpec.
type TimeServer struct {
	Servers []string `yaml:"timeServers"`
	Layer   string   `yaml:"layer"`
}

// Operator mirrors network.OperatorSpecSpec. No operators are emitted; the
// type exists so the document carries an explicit empty list like Talos does.
type Operator struct {
	Operator string `yaml:"operator"`
	LinkName string `yaml:"linkName"`
	Layer    string `yaml:"layer"`
}

func newConfig() *Config {
	return &Config{
		Addresses:   []Address{},
		Links:       []Link{},
		Routes:      []Route{},
		Hostnames:   []Hostname{},
		Resolvers:   []Resolver{},
		TimeServers: []TimeServer{},
		Operators:   []Operator{},
		ExternalIPs: []string{},
	}
}

// Marshal renders the document as YAML in the layout Talos expects.
func (c *Config) Marshal() ([]byte, error) {
	out, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshalling network configuration: %w", err)
	}

	return out, nil
}

// Parse decodes a YAML document. It is used to validate operator-supplied
// configuration before it is written to META.
func Parse(doc []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(doc, &cfg); err != nil {
		return nil, fmt.Errorf("parsing network configuration: %w", err)
	}

	return &cfg, nil
}
