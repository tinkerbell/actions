---
slug: qemuimg2disk
name: qemuimg2disk
tags: disk
maintainers: Nahum Shalman <nshalman@equinix.com>
description: "This action will stream a remote disk image to a block device, and
is mainly used to write cloud images to a disk."
version: v1.0.0
createdAt: "2021-10-06T15:38:02.00Z"
---

Under the hood this action uses qemu-img which in turn
is built atop libcurl. This has two nice benefits:

1. Full SSL support allows images to be streamed from anywhere on the internet
2. Support for the QCOW2 image format which is used by some OS vendors as a standard format

This action could be used as part of a workflow to stream the latest Ubuntu image
directly from Canonical onto block storage disk `/dev/sda` like so:

```yaml
actions:
    - name: "stream ubuntu"
      image: quay.io/tinkerbell-actions/qemuimg2disk:v1.0.0
      timeout: 300
      environment:
          IMG_URL: https://cloud-images.ubuntu.com/daily/server/focal/current/focal-server-cloudimg-amd64.img
          DEST_DISK: /dev/sda
```
