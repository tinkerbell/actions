---
slug: stream 
name: stream
tags: disk
maintainers: Dan Finneran <daniel.finneran@gmail.com>
description: "This action will stream a remote disk image (raw) to a block device, and
is mainly used to write cloud images to a disk. It is recommended to use the `qemu-img`
tool to convert disk images into raw, it is also possible to compress the raw images 
with tar+gzip to prevent wasted disk space"
version: v1.0.0
createdAt: "2021-01-20T12:41:45.14Z"
---

The below example will stream a raw ubuntu cloud image (converted by qemu-img) and write
it to the block storage disk `/dev/sda`. The raw image is uncompressed in this example.

```yaml
    actions:
      - name: "stream ubuntu"
        image: thebsdbox/stream:0.0
        timeout: 90
        environment:
          IMG_URL: 192.168.1.2/ubuntu.raw
          DEST_DISK: /dev/sda
          COMPRESSED: false
```


The below example will stream a compressed raw ubuntu cloud image (converted by qemu-img) 
and then compressed with tar/gzip to reduce local space.

```
qemu-img convert ubuntu.img ubuntu.raw
tar -cvzf ubuntu.tar.gz ubuntu.raw
```

```yaml
    actions:
      - name: "stream ubuntu"
        image: thebsdbox/stream:0.0
        timeout: 90
        environment:
          IMG_URL: 192.168.1.2/ubuntu.tar.gz
          DEST_DISK: /dev/sda
          COMPRESSED: true
```