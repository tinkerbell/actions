# talosmeta action design

Date: 2026-09-07

## Goal

Add a Tinkerbell action that writes Talos Linux metal network configuration into
the META partition of a freshly imaged Talos disk. The action retrieves the
machine's Hardware data from the Tinkerbell metadata service the same way the
`rootio` action does, maps that data onto Talos' platform network configuration
document, and stores the YAML under META key `0xa`
(`MetalNetworkPlatformConfig`), which is what `talosctl meta write 0xa "$(cat
network.yaml)"` does on a running node.

This lets a Talos node come up with static addressing, routes, hostname, DNS
and NTP before it has fetched its machine configuration, which matters when the
Tinkerbell DHCP server stops serving the machine after provisioning.

## Inputs

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DEST_DISK` | yes | | Block device holding the Talos image, e.g. `/dev/sda`. |
| `MIRROR_HOST` | unless `NETWORK_CONFIG` is set | | Tinkerbell host serving `/metadata`. |
| `METADATA_SERVICE_PORT` | no | `7080` | Port of the consolidated Tinkerbell HTTP server. |
| `NETWORK_CONFIG` | no | | Raw Talos network YAML. When set, the metadata service is not consulted and the document is written verbatim. |
| `LINK_NAMING` | no | `predictable` | `predictable` derives the name Talos' udev will assign from sysfs; `kernel` keeps the name HookOS uses. |

## Data flow

1. Read environment.
2. Unless `NETWORK_CONFIG` is set, `GET http://MIRROR_HOST:PORT/metadata` and
   decode the Hardware spec shaped JSON: `interfaces[].dhcp` and
   `metadata.instance`.
3. Map to a Talos `PlatformConfigSpec` shaped document (see mapping).
4. Open `DEST_DISK`, read its GPT, find the partition named `META`.
5. Load the existing Talos ADV (first 512 KiB of the partition, two redundant
   copies) and the legacy syslinux ADV (last 1 KiB) with `siderolabs/go-adv`.
6. Set tag `0xa` to the YAML, keep every other tag, and flush both ADVs at the
   same offsets Talos uses. `fsync` the device.
7. Log the YAML that was written.

Writes go through the whole-disk device at the partition's byte offset, so the
action does not depend on partition device nodes existing inside the container.

## Mapping

Per Talos docs "Metal network configuration". Every emitted resource carries
`layer: platform`.

For every interface with `dhcp.ip.address`:

- Link: `{name, up: true}`. If `dhcp.vlan_id` is set, also emit a logical link
  `<name>.<vid>` with `kind: vlan`, `type: ether`, `parentName: <name>`,
  `vlan: {vlanID: <vid>, vlanProtocol: 802.1q}`; addresses and routes attach to
  the VLAN link.
- Address: `{address: <ip>/<prefix>, linkName, family: inet4|inet6, scope:
  global, flags: permanent}`. Prefix length comes from `dhcp.ip.netmask`
  (dotted quad or a bare prefix length). Missing netmask is an error.
- Default route when `dhcp.ip.gateway` is set: `{family, gateway, outLinkName,
  table: main, priority: 1024 (inet4) or 2048 (inet6), scope: global, type:
  unicast, protocol: static}`.
- One route per `dhcp.classless_static_routes` entry with `dst`.

Once:

- Hostname: `metadata.instance.hostname`, else the first `dhcp.hostname`.
  Domain from the first `dhcp.domain_name`.
- Resolvers: de-duplicated union of `dhcp.name_servers` as `dnsServers`.
- Time servers: de-duplicated union of `dhcp.time_servers`.
- `externalIPs`: `metadata.instance.ips[].address` where `public` is true.
- `operators: []`. `metadata` is omitted because the metal platform replaces it.

If no interface has a static address the action fails; there is nothing useful
to write and Talos' default DHCP behaviour should be left alone.

## Link names

Talos names interfaces with udev's predictable scheme (`enp1s0`, `eno1`,
`ens3`) unless booted with `net.ifnames=0`. HookOS uses kernel names (`eth0`).
The name written into META must be the one Talos will see.

Resolution order:

1. `dhcp.iface_name` when set. Operators can always pin the name here.
2. Match `dhcp.mac` against `/sys/class/net/*/address`.
   - `LINK_NAMING=kernel`: use that interface name.
   - `LINK_NAMING=predictable`: derive the udev name from sysfs: onboard
     (`eno<acpi_index|index>`), then PCI hotplug slot (`ens<slot>`), then PCI
     path (`enp<bus>s<slot>[f<func>]`, prefixed with `P<domain>` when the
     domain is non-zero). If the device is not PCI, fall back to the kernel
     name and log a warning.
3. Otherwise fail.

## Packages

- `talosmeta/main.go`: environment parsing and orchestration, `run(logger) error`
  like `writefile`.
- `talosmeta/hardware`: metadata client and JSON types.
- `talosmeta/talosnet`: Talos document types with YAML tags, `FromHardware`,
  link naming (`Namer` interface with a sysfs implementation rooted at a
  configurable path for tests).
- `talosmeta/meta`: META partition discovery and ADV read/write.

New dependencies: `github.com/siderolabs/go-adv`, `go.yaml.in/yaml/v3`.
`go-adv` requires Go 1.26.3, so the module's `go` directive moves to that.

## Errors

Every failure is fatal and logged with context. Partial writes are avoided by
building the full ADV bytes in memory before writing. Missing META partition,
unreachable metadata service, metadata without interfaces, unresolvable link
name and unparseable addresses each produce a distinct message.

## Testing

- `talosnet`: table tests for netmask parsing, family detection, VLAN handling
  and a golden YAML test against the structure in the Talos docs. Link naming
  tests use a fake sysfs tree.
- `hardware`: `httptest` server returning a Hardware spec.
- `meta`: create a small GPT image with `go-diskfs`, add a `META` partition,
  write tag `0xa`, read it back with `go-adv`, and re-run to prove other tags
  survive.

## Known gap

The `/metadata` endpoint in tinkerbell's tootles round-trips the Hardware spec
through `data.HackInstance`, which only keeps storage fields, so interface data
is dropped today. Extending `HackInstance` with `interfaces` and
`metadata.instance.hostname`/`ips` (the JSON tags already line up) unblocks
this action. Until then the action fails with a message pointing at this, and
`NETWORK_CONFIG` can be used as a passthrough.
