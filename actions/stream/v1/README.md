---
slug: stream 
name: stream
tags: disk
maintainers: Dan Finneran <daniel.finneran@gmail.com>
description: "This action will stream a remote disk image (raw) to a block device, and
is mainly used to write cloud images to a disk."
version: v1.0.0
createdAt: "2021-01-20T12:41:45.14Z"
---

The below example will use the same action in three ways, to "partition" followed by "formating" and
finally mounting the disks.

```yaml
    actions:
      - name: "stream ubuntu"
        image: thebsdbox/strea:0.0
        timeout: 90
        environment:
          IMG_URL: 192.168.1.2/ubuntu.img
          DEST_DISK: /dev/sda3
          COMPRESSED: true
```