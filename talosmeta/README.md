```
quay.io/tinkerbell/actions/talosmeta:latest
```

This action writes [Talos Linux](https://www.talos.dev) metal platform network
configuration into the `META` partition of a disk that already holds a Talos
image. It retrieves the machine's Hardware data from the Tinkerbell metadata
service the same way the `rootio` action does, maps it onto the Talos network
configuration document, and stores the YAML under META key `0xa`
(`MetalNetworkPlatformConfig`). This is what
`talosctl meta write 0xa "$(cat network.yaml)"` does on a running node, done
offline before the first boot.

Talos applies the document before it fetches its machine configuration, so a
node gets a static address, default route, hostname, DNS and NTP even when the
Tinkerbell DHCP server no longer serves it after provisioning.

```yaml
actions:
- name: "stream-talos-image"
  image: quay.io/tinkerbell/actions/oci2disk:latest
  timeout: 600
  environment:
    DEST_DISK: /dev/sda
    IMG_URL: "192.168.1.2:5000/talos/metal-amd64:v1.14.0"
- name: "write-talos-network-config"
  image: quay.io/tinkerbell/actions/talosmeta:latest
  timeout: 60
  environment:
    DEST_DISK: /dev/sda
    MIRROR_HOST: 192.168.1.2
```

## Environment Variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DEST_DISK` | yes | | Block device holding the Talos image, for example `/dev/sda`. The partition named `META` is located through the GPT, so partition device nodes are not needed. |
| `MIRROR_HOST` | unless `NETWORK_CONFIG` or `HARDWARE_SPEC` is set | | Host of the Tinkerbell metadata service. |
| `METADATA_SERVICE_PORT` | no | `7080` | Port of the consolidated Tinkerbell HTTP server that serves `/metadata`. |
| `HARDWARE_SPEC` | no | | The Hardware `spec` (or just its `interfaces` and `metadata.instance`) as JSON. Takes precedence over the metadata service; a Workflow template can render it from `.hardware.spec`, see below. |
| `NETWORK_CONFIG` | no | | A complete Talos network configuration document. When set nothing else is consulted and the YAML is written verbatim, exactly like `talosctl meta write 0xa`. |
| `LINK_NAMING` | no | `predictable` | How interface names are derived when the Hardware does not set `iface_name`; see below. |

Re-running the action replaces key `0xa` and leaves every other META key intact.

## Passing Hardware data from the Workflow template

Tinkerbell exposes the Hardware object to Workflow templates as `.hardware`,
so the template can hand the relevant part of the spec straight to the action
without any metadata service round trip:

```yaml
- name: "write-talos-network-config"
  image: quay.io/tinkerbell/actions/talosmeta:latest
  timeout: 120
  environment:
    DEST_DISK: /dev/sda
    HARDWARE_SPEC: {{ dict "interfaces" (dig "interfaces" (list) .hardware.spec) "metadata" (dict "instance" (dict "hostname" (dig "metadata" "instance" "hostname" "" .hardware.spec) "ips" (dig "metadata" "instance" "ips" (list) .hardware.spec))) | toJson | quote }}
```

`dig` keeps the render working when a field is absent, and `quote` produces
a YAML double-quoted scalar. Only `interfaces` and `metadata.instance` are
needed, so the machine configuration in `spec.userData` never leaves the
Hardware object.

## Mapping

The document follows the Talos
[metal network configuration](https://docs.siderolabs.com/talos/latest/platform-specific-installations/bare-metal-platforms/metal-network-configuration)
format. Every entry is written on the `platform` layer, so the machine
configuration can still override it.

For each `spec.interfaces[]` entry with `dhcp.ip.address`:

- a link `{name, up: true}`; with `dhcp.vlan_id` set, also a logical VLAN
  link `<name>.<vid>` that carries the address and routes,
- an address `<address>/<prefix>` with `family`, `scope: global` and
  `flags: permanent`; the prefix length comes from `dhcp.ip.netmask` (dotted
  quad or a bare length),
- a default route through `dhcp.ip.gateway` (`table: main`, priority `1024`
  for IPv4 and `2048` for IPv6),
- one route per `dhcp.classless_static_routes` entry.

For the machine:

- `hostnames` from `metadata.instance.hostname`, falling back to the first
  `dhcp.hostname`; the domain comes from `dhcp.domain_name` or the FQDN,
- `resolvers` from the union of `dhcp.name_servers`,
- `timeServers` from the union of `dhcp.time_servers`,
- `externalIPs` from `metadata.instance.ips[]` entries marked `public`.

Interfaces without a static address are left alone so Talos keeps using DHCP
on them. The action fails when no interface has a static address.

Example output for one interface:

```yaml
addresses:
    - address: 10.0.80.10/24
      linkName: enp1s0
      family: inet4
      scope: global
      flags: permanent
      layer: platform
links:
    - name: enp1s0
      up: true
      layer: platform
routes:
    - family: inet4
      gateway: 10.0.80.1
      outLinkName: enp1s0
      table: main
      priority: 1024
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
externalIPs: []
```

## Interface names

Talos names interfaces with udev's predictable scheme (`enp1s0`, `eno1`,
`ens3`) unless it boots with `net.ifnames=0`, while HookOS uses kernel names
(`eth0`). The name written to META has to be the one Talos will see.

1. `dhcp.iface_name` is used verbatim when set. Set it in the Hardware to pin
   the name.
2. Otherwise the interface carrying `dhcp.mac` is looked up in sysfs.
   - `LINK_NAMING=predictable` (default) derives the udev name: onboard
     index (`eno<n>`), then PCI hotplug slot (`ens<n>`), then PCI path
     (`enp<bus>s<slot>[f<function>]`). Non-PCI devices keep the kernel name
     and a warning is logged.
   - `LINK_NAMING=kernel` keeps the HookOS name, for Talos images booted with
     `net.ifnames=0`.

## Verifying on the node

```
talosctl -n <node> get meta 0x0a -o yaml
talosctl -n <node> get addresses
```

## Metadata service requirements

The document is built from `interfaces[].dhcp` and `metadata.instance` in the
JSON returned by `/metadata`. Tinkerbell's tootles `/metadata` endpoint
currently reduces the Hardware spec to the storage fields the `rootio` action
needs, so the interfaces are missing and the action stops with a message
saying so. Extending tootles' `HackInstance` type with `interfaces` and
`metadata.instance.hostname`/`ips` (the JSON tags already match the Hardware
CRD) makes the data available. Until then, `NETWORK_CONFIG` can carry a
document rendered by the workflow template.
