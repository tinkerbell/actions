---
slug: cexec
name: cexec
tags: command
maintainers: Dan Finneran <daniel.finneran@gmail.com>
description: "This action can be used in a variety of ways, however its core functionality
is the management of disks. The action parses the metadata and will partition, format and
mount disks"
version: v1.0.0
createdAt: "2021-01-20T12:41:45.14Z"
---

The `cexec` action performs *execution* either within a [chroot](https://en.wikipedia.org/wiki/Chroot) environment
or within the base filesystem. The primary use-case is being able to provision 
files/an Operating System to disk and then being able to execute something that
perhaps resides within that filesystem.

```yaml
actions:
    - name: "Install Grub"
      image: quay.io/tinkerbell-actions/cexec:v1.0.0
      timeout: 90
      environment:
          BLOCK_DEVICE: /dev/sda3
          FS_TYPE: ext4
          CHROOT: y
          CMD_LINE: "grub-install --root-directory=/boot /dev/sda"
```

In order to execute multiple commands (seperated by a semi-colon) we will
need to leverage a shell. We do this by passing `sh -c` as a `DEFAULT_INTERPRETER`.
This interpreter will then parse your commands.

```yaml
actions:
    - name: "Update packages"
      image: quay.io/tinkerbell-actions/cexec:v1.0.0
      timeout: 90
      environment:
          BLOCK_DEVICE: /dev/sda3
          FS_TYPE: ext4
          CHROOT: y
          DEFAULT_INTERPRETER: "/bin/sh -c"
          CMD_LINE: "apt-get -y update; apt-get -y upgrade"
```
