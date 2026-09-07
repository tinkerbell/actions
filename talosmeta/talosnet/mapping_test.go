package talosnet

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/tinkerbell/actions/talosmeta/hardware"
)

type staticNamer map[string]string

func (s staticNamer) Name(mac string) (string, error) {
	if name, ok := s[mac]; ok {
		return name, nil
	}
	return "", errors.New("unknown mac " + mac)
}

func iface(mac, address, netmask, gateway string, family int64) hardware.Interface {
	return hardware.Interface{DHCP: &hardware.DHCP{
		MAC: mac,
		IP:  &hardware.IP{Address: address, Netmask: netmask, Gateway: gateway, Family: family},
	}}
}

func decodeYAML(t *testing.T, doc []byte) any {
	t.Helper()
	var out any
	if err := yaml.Unmarshal(doc, &out); err != nil {
		t.Fatalf("invalid yaml: %v\n%s", err, doc)
	}
	return out
}

func assertYAML(t *testing.T, cfg *Config, want string) {
	t.Helper()
	got, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if !reflect.DeepEqual(decodeYAML(t, got), decodeYAML(t, []byte(want))) {
		t.Fatalf("unexpected document\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFromHardwareStaticIPv4Interface(t *testing.T) {
	in := iface("52:54:00:12:34:01", "10.0.80.10", "255.255.255.0", "10.0.80.1", 4)
	in.DHCP.Hostname = "node1"
	in.DHCP.DomainName = "example.com"
	in.DHCP.NameServers = []string{"10.0.0.2", "10.0.0.2"}
	in.DHCP.TimeServers = []string{"10.0.0.3"}
	in.DHCP.ClasslessStaticRoutes = []hardware.ClasslessStaticRoute{{DestinationDescriptor: "192.168.0.0/16", Router: "10.0.80.254"}}

	spec := &hardware.Spec{
		Interfaces: []hardware.Interface{in},
		Metadata: &hardware.Metadata{Instance: &hardware.Instance{
			IPs: []hardware.InstanceIP{{Address: "203.0.113.5", Family: 4, Public: true}, {Address: "10.0.80.10", Family: 4}},
		}},
	}

	cfg, err := FromHardware(spec, staticNamer{"52:54:00:12:34:01": "eth0"})
	if err != nil {
		t.Fatalf("FromHardware() error: %v", err)
	}

	assertYAML(t, cfg, `
addresses:
    - address: 10.0.80.10/24
      linkName: eth0
      family: inet4
      scope: global
      flags: permanent
      layer: platform
links:
    - name: eth0
      up: true
      layer: platform
routes:
    - family: inet4
      gateway: 10.0.80.1
      outLinkName: eth0
      table: main
      priority: 1024
      scope: global
      type: unicast
      protocol: static
      layer: platform
    - family: inet4
      dst: 192.168.0.0/16
      gateway: 10.0.80.254
      outLinkName: eth0
      table: main
      scope: global
      type: unicast
      protocol: static
      layer: platform
hostnames:
    - hostname: node1
      domainname: example.com
      layer: platform
resolvers:
    - dnsServers:
        - 10.0.0.2
      layer: platform
timeServers:
    - timeServers:
        - 10.0.0.3
      layer: platform
operators: []
externalIPs:
    - 203.0.113.5
`)
}

func TestFromHardwareIPv6UsesPrefixLengthAndHigherPriority(t *testing.T) {
	spec := &hardware.Spec{Interfaces: []hardware.Interface{iface("52:54:00:12:34:01", "2001:db8::10", "64", "2001:db8::1", 6)}}

	cfg, err := FromHardware(spec, staticNamer{"52:54:00:12:34:01": "eth0"})
	if err != nil {
		t.Fatalf("FromHardware() error: %v", err)
	}

	assertYAML(t, cfg, `
addresses:
    - address: 2001:db8::10/64
      linkName: eth0
      family: inet6
      scope: global
      flags: permanent
      layer: platform
links:
    - name: eth0
      up: true
      layer: platform
routes:
    - family: inet6
      gateway: 2001:db8::1
      outLinkName: eth0
      table: main
      priority: 2048
      scope: global
      type: unicast
      protocol: static
      layer: platform
hostnames: []
resolvers: []
timeServers: []
operators: []
externalIPs: []
`)
}

func TestFromHardwareVLANAttachesAddressToVLANLink(t *testing.T) {
	in := iface("52:54:00:12:34:01", "10.0.80.10", "24", "", 4)
	in.DHCP.VLANID = "100"
	spec := &hardware.Spec{Interfaces: []hardware.Interface{in}}

	cfg, err := FromHardware(spec, staticNamer{"52:54:00:12:34:01": "eth0"})
	if err != nil {
		t.Fatalf("FromHardware() error: %v", err)
	}

	assertYAML(t, cfg, `
addresses:
    - address: 10.0.80.10/24
      linkName: eth0.100
      family: inet4
      scope: global
      flags: permanent
      layer: platform
links:
    - name: eth0
      up: true
      layer: platform
    - name: eth0.100
      logical: true
      up: true
      kind: vlan
      type: ether
      parentName: eth0
      vlan:
        vlanID: 100
        vlanProtocol: 802.1q
      layer: platform
routes: []
hostnames: []
resolvers: []
timeServers: []
operators: []
externalIPs: []
`)
}

func TestFromHardwarePrefersInstanceHostnameAndIfaceName(t *testing.T) {
	in := iface("52:54:00:12:34:01", "10.0.80.10", "24", "", 4)
	in.DHCP.Hostname = "dhcp-name"
	in.DHCP.IfaceName = "enp1s0"
	spec := &hardware.Spec{
		Interfaces: []hardware.Interface{in},
		Metadata:   &hardware.Metadata{Instance: &hardware.Instance{Hostname: "instance-name"}},
	}

	cfg, err := FromHardware(spec, staticNamer{})
	if err != nil {
		t.Fatalf("FromHardware() error: %v", err)
	}
	if len(cfg.Hostnames) != 1 || cfg.Hostnames[0].Hostname != "instance-name" {
		t.Fatalf("expected instance hostname, got %+v", cfg.Hostnames)
	}
	if len(cfg.Links) != 1 || cfg.Links[0].Name != "enp1s0" {
		t.Fatalf("expected iface_name to win, got %+v", cfg.Links)
	}
}

func TestFromHardwareSkipsInterfacesWithoutStaticIP(t *testing.T) {
	spec := &hardware.Spec{Interfaces: []hardware.Interface{
		{DHCP: &hardware.DHCP{MAC: "52:54:00:12:34:02"}},
		{},
		iface("52:54:00:12:34:01", "10.0.80.10", "24", "", 4),
	}}

	cfg, err := FromHardware(spec, staticNamer{"52:54:00:12:34:01": "eth0"})
	if err != nil {
		t.Fatalf("FromHardware() error: %v", err)
	}
	if len(cfg.Addresses) != 1 || len(cfg.Links) != 1 {
		t.Fatalf("expected a single address and link, got %+v / %+v", cfg.Addresses, cfg.Links)
	}
}

func TestFromHardwareErrors(t *testing.T) {
	tests := []struct {
		name string
		spec *hardware.Spec
		want string
	}{
		{"no interfaces", &hardware.Spec{}, "no interface"},
		{"no static ip", &hardware.Spec{Interfaces: []hardware.Interface{{DHCP: &hardware.DHCP{MAC: "52:54:00:12:34:01"}}}}, "no interface"},
		{"missing netmask", &hardware.Spec{Interfaces: []hardware.Interface{iface("52:54:00:12:34:01", "10.0.80.10", "", "", 4)}}, "netmask"},
		{"bad address", &hardware.Spec{Interfaces: []hardware.Interface{iface("52:54:00:12:34:01", "nope", "24", "", 4)}}, "address"},
		{"bad gateway", &hardware.Spec{Interfaces: []hardware.Interface{iface("52:54:00:12:34:01", "10.0.80.10", "24", "nope", 4)}}, "gateway"},
		{"unknown mac", &hardware.Spec{Interfaces: []hardware.Interface{iface("52:54:00:ff:ff:ff", "10.0.80.10", "24", "", 4)}}, "link name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromHardware(tt.spec, staticNamer{"52:54:00:12:34:01": "eth0"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestPrefixLength(t *testing.T) {
	tests := []struct {
		netmask string
		family  int64
		want    int
		wantErr bool
	}{
		{"255.255.255.0", 4, 24, false},
		{"255.255.255.255", 4, 32, false},
		{"0.0.0.0", 4, 0, false},
		{"24", 4, 24, false},
		{"/24", 4, 24, false},
		{"64", 6, 64, false},
		{"ffff:ffff:ffff:ffff::", 6, 64, false},
		{"255.255.0.255", 4, 0, true},
		{"33", 4, 0, true},
		{"129", 6, 0, true},
		{"", 4, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.netmask, func(t *testing.T) {
			got, err := PrefixLength(tt.netmask, tt.family)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PrefixLength(%q) error = %v, wantErr %v", tt.netmask, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("PrefixLength(%q) = %d, want %d", tt.netmask, got, tt.want)
			}
		})
	}
}

func TestParseFailsOnInvalidYAML(t *testing.T) {
	if _, err := Parse([]byte("addresses: [")); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseRoundTrip(t *testing.T) {
	doc := []byte("hostnames:\n    - hostname: node1\n      layer: platform\n")
	cfg, err := Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(cfg.Hostnames) != 1 || cfg.Hostnames[0].Hostname != "node1" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestFromHardwareSplitsFQDNHostname(t *testing.T) {
	spec := &hardware.Spec{
		Interfaces: []hardware.Interface{iface("52:54:00:12:34:01", "10.0.80.10", "24", "", 4)},
		Metadata:   &hardware.Metadata{Instance: &hardware.Instance{Hostname: "node1.example.com"}},
	}

	cfg, err := FromHardware(spec, staticNamer{"52:54:00:12:34:01": "eth0"})
	if err != nil {
		t.Fatalf("FromHardware() error: %v", err)
	}
	if len(cfg.Hostnames) != 1 || cfg.Hostnames[0].Hostname != "node1" || cfg.Hostnames[0].Domainname != "example.com" {
		t.Fatalf("expected node1 / example.com, got %+v", cfg.Hostnames)
	}
}
