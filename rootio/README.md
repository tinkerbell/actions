```
quay.io/tinkerbell/actions/rootio:latest
```

This action can be used in a variety of ways, however its core functionality
is the management of disks. The action parses the metadata and will partition, format and
mount disks

The below example will use the same action in three ways, to "partition" followed by "formating" and
finally mounting the disks. All of the different commands below `partition`/`format`/`mount`, will
have their actions determined by the contents of the storage section in the metadata refer to the
[Customer Partitioning and RAID](https://deploy.equinix.com/developers/docs/metal/storage/custom-partitioning-raid/) documentation for more information.

```yaml
actions:
- name: "disk-wipe-partition"
  image: quay.io/tinkerbell/actions/rootio:latest
  timeout: 90
  command: ["partition"]
  environment:
    MIRROR_HOST: 192.168.1.2
- name: "format"
  image: quay.io/tinkerbell/actions/rootio:latest
  timeout: 90
  command: ["format"]
  environment:
    MIRROR_HOST: 192.168.1.2
- name: "mount"
  image: quay.io/tinkerbell/actions/rootio:latest
  timeout: 90
  command: ["mount"]
  environment:
    MIRROR_HOST: 192.168.1.2
```

**Advanced**

For certain use-cases we may need to use [MBR]() support, examples being
VMware vSphere and it's installer. In order to support this, we can pass
an environment variable to rootio for `partition` options.

```yaml
actions:
- name: "disk-wipe-partition"
  image: quay.io/tinkerbell/actions/rootio:latest
  timeout: 90
  command: ["partition"]
  environment:
    MIRROR_HOST: 192.168.1.2
    MBR: true
```

This also supports an extended version of CPR:

```json
"storage": {
    "disks": [
      {
        "device": "/dev/sdb",
        "partitions": [
          {
            "label": "FAT32_ACTIVE",
            "number": 1,
            "size": 0
          }
        ],
        "wipe_table": true
      }
    ],
    "filesystems": [
      {
        "mount": {
          "create": {
            "options": ["-L", "ROOT"]
          },
          "device": "/dev/sdb1",
          "format": "vfat",
          "point": "/"
        }
      }
    ]
  }
```

Where labels `FAT32/Linux` can be appended with `_ACTIVE` to make them a
bootable partition.

**Software RAID**

The `partition` command can also assemble Linux software RAID (mdadm)
arrays once the underlying partitions exist. Declare them under
`storage.raid` in the metadata. Arrays are created after partitioning
and before volume groups, so LVM PVs can sit on top of `/dev/mdX`.

Supported levels: `0`, `1`, `4`, `5`, `6`, `10`, `linear`.

```yaml
actions:
- name: "disk-wipe-partition"   # partitions disks + assembles /dev/md0
  image: quay.io/tinkerbell/actions/rootio:latest
  timeout: 90
  command: ["partition"]
  environment:
    MIRROR_HOST: 192.168.1.2
- name: "format"                # mkfs.ext4 /dev/md0
  image: quay.io/tinkerbell/actions/rootio:latest
  timeout: 90
  command: ["format"]
  environment:
    MIRROR_HOST: 192.168.1.2
- name: "mount"
  image: quay.io/tinkerbell/actions/rootio:latest
  timeout: 90
  command: ["mount"]
  environment:
    MIRROR_HOST: 192.168.1.2
```

```json
"storage": {
  "disks": [
    {
      "device": "/dev/sdb",
      "wipe_table": true,
      "partitions": [{"label": "LINUX", "number": 1, "size": 0}]
    },
    {
      "device": "/dev/sdc",
      "wipe_table": true,
      "partitions": [{"label": "LINUX", "number": 1, "size": 0}]
    }
  ],
  "raid": [
    {
      "name": "md0",
      "level": "1",
      "devices": ["/dev/sdb1", "/dev/sdc1"]
    }
  ],
  "filesystems": [
    {
      "mount": {
        "device": "/dev/md0",
        "format": "ext4",
        "point": "/",
        "create": {"options": ["-L", "ROOT"]}
      }
    }
  ]
}
```

On re-runs, the `partition` and `wipe` commands stop any active array
named in metadata and zero the mdadm superblock from its member devices
before repartitioning, so the action is idempotent against a previously
deployed host.
