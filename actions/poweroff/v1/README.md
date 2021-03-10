---
slug: poweroff
name: poweroff
tags: network
maintainers: Marky Jackson <marky.r.jackson@gmail.com>
description: "This action makes use of the kexec function that should be compiled into the
tinkie kernel for powering off. This provides a faster alternative to rebooting, and allows an action to
effectively jump straight into the newly provisioned Operating System"
version: v1.0.0
createdAt: "2021-02-02T12:41:45.14Z"
---

Need to add more here.

```yaml
actions:
    - name: "poweroff"
      image: quay.io/tinkerbell-actions/kexec:v1.0.0
      timeout: 90
      pid: host
      environment:
          BLOCK_DEVICE: /dev/sda3
          FS_TYPE: ext4
          KERNEL_PATH: /boot/vmlinuz
          INITRD_PATH: /boot/initrd
          CMD_LINE: "root=/dev/sda3 ro"
```

Troubleshooting:

Need to add more here.
