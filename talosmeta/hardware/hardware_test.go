package hardware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleMetadata = `{
  "interfaces": [
    {
      "dhcp": {
        "mac": "52:54:00:12:34:01",
        "hostname": "node1",
        "domain_name": "example.com",
        "name_servers": ["10.0.0.2"],
        "time_servers": ["10.0.0.3"],
        "iface_name": "enp1s0",
        "vlan_id": "100",
        "ip": {"address": "10.0.80.10", "netmask": "255.255.255.0", "gateway": "10.0.80.1", "family": 4},
        "classless_static_routes": [{"destination_descriptor": "192.168.0.0/16", "router": "10.0.80.254"}]
      }
    }
  ],
  "metadata": {
    "instance": {
      "id": "i-1",
      "hostname": "node1.example.com",
      "ips": [{"address": "203.0.113.5", "family": 4, "public": true}],
      "storage": {"disks": [{"device": "/dev/sda"}]}
    }
  }
}`

func TestFetchDecodesInterfacesAndInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleMetadata))
	}))
	defer srv.Close()

	spec, err := Fetch(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if len(spec.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(spec.Interfaces))
	}
	dhcp := spec.Interfaces[0].DHCP
	if dhcp == nil {
		t.Fatal("expected dhcp block")
	}
	if dhcp.MAC != "52:54:00:12:34:01" || dhcp.IfaceName != "enp1s0" || dhcp.VLANID != "100" {
		t.Fatalf("unexpected dhcp: %+v", dhcp)
	}
	if dhcp.IP == nil || dhcp.IP.Address != "10.0.80.10" || dhcp.IP.Netmask != "255.255.255.0" || dhcp.IP.Gateway != "10.0.80.1" || dhcp.IP.Family != 4 {
		t.Fatalf("unexpected ip: %+v", dhcp.IP)
	}
	if len(dhcp.ClasslessStaticRoutes) != 1 || dhcp.ClasslessStaticRoutes[0].DestinationDescriptor != "192.168.0.0/16" || dhcp.ClasslessStaticRoutes[0].Router != "10.0.80.254" {
		t.Fatalf("unexpected routes: %+v", dhcp.ClasslessStaticRoutes)
	}
	if spec.Metadata == nil || spec.Metadata.Instance == nil {
		t.Fatal("expected metadata.instance")
	}
	inst := spec.Metadata.Instance
	if inst.ID != "i-1" || inst.Hostname != "node1.example.com" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if len(inst.IPs) != 1 || inst.IPs[0].Address != "203.0.113.5" || !inst.IPs[0].Public {
		t.Fatalf("unexpected instance ips: %+v", inst.IPs)
	}
}

func TestFetchRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no hardware found for source ip", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
}

func TestFetchRejectsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}
