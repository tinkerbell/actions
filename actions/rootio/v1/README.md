---
slug: rootio
name: rootio
tags: disk
maintainers: Dan Finneran <daniel.finneran@gmail.com>
description: "This action can be used in a variety of ways, however its core functionality
is the management of disks. The action parses the metadata and will partition, format and
mount disks"
version: v1.0.0
createdAt: "2021-01-20T12:41:45.14Z"
---

The below example will use the same action in three ways, to "partition" followed by "formating" and
finally mounting the disks. All of the different commands below `partition`/`format`/`mount`, will 
have their actions determined by the contents of the storage section in the metadata refer to the
[Customer Partitioning and RAID](https://metal.equinix.com/developers/docs/servers/custom-partitioning-raid/) documentation for more information.

```yaml
    actions:
      - name: "disk-wipe-partition"
        image: quay.io/tinkerbell-actions/rootio:v1.0.0
        timeout: 90
        command: [ "partition" ]
        environment:
          MIRROR_HOST: 192.168.1.2
      - name: "format"
        image: quay.io/tinkerbell-actions/rootio:v1.0.0
        timeout: 90
        command: [ "format" ]
        environment:
          MIRROR_HOST: 192.168.1.2
      - name: "mount"
        image: quay.io/tinkerbell-actions/rootio:v1.0.0
        timeout: 90
        command: [ "mount" ]
        environment:
          MIRROR_HOST: 192.168.1.2
```