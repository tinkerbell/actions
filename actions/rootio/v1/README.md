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
finally mounting the disks.

```yaml
    actions:
      - name: "disk-wipe-partition"
        image: thebsdbox/rootio:0.0
        timeout: 90
        command: [ "partition" ]
        environment:
          MIRROR_HOST: 192.168.1.2
      - name: "format"
        image: thebsdbox/rootio:0.0
        timeout: 90
        command: [ "format" ]
        environment:
          MIRROR_HOST: 192.168.1.2
      - name: "mount"
        image: thebsdbox/rootio:0.0
        timeout: 90
        command: [ "mount" ]
        environment:
          MIRROR_HOST: 192.168.1.2
```