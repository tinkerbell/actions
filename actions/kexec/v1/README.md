---
slug: kexec
name: kexec
tags: disk
maintainers: Dan Finneran <daniel.finneran@gmail.com>
description: "This action makes use of the kexec function that should be compiled into the
tinkie kernel. This provides a faster alternative to rebooting, and allows an action to 
effectively jump straight into the newly provisioned Operating System"
version: v1.0.0
createdAt: "2021-02-02T12:41:45.14Z"
---

The below example details all of the pieces of information that are required in order to kexec
into a newly provisioned Operating System. The `BLOCK_DEVICE` should point to where the kernel
/initrd on the newly provisioned operating system reside, and the `FS_TYPE` is required so that
the kexec action can mount the newly created Operating System and find it's kernel/initrd. The
paths should relate to the paths "inside" the newly provisioned OS along with the `CMD_LINE`
that will be required to boot the new OS successfully. To discover these things you may need to
examine the `/boot` folder in the newly written OS and look in the `/boot/grub/grub.cfg` to
understand the `CMD_LINE`.

```yaml
actions:
    - name: "kexec ubuntu"
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

`function not implemented` - If this error is reported and the `kexec` fails this is due to the kernel that tinkie is running, not supporting kexec.

No kexec occurs after a few seconds, but the action completes succesfully. Ensure that `pid: host` is part of the action, this is required so that
kexec to the new kernel occurs in the correct namespace.
