# Actions

This repository is a suite of reusable Tinkerbell Actions that are used to compose Tinkerbell Workflows.

| Name | Description |
| --- | --- |
| [archive2disk](/archive2disk/)    | Write archives to a block device |
| [cexec](/cexec/)                  | chroot and execute binaries |
| [grub2disk](/grub2disk/)          | Write grub configs to a block device |
| [image2disk](/image2disk/)        | Write images to a block device |
| [kexec](/kexec/)                  | kexec to a Linux Kernel |
| [oci2disk](/oci2disk/)            | Stream OCI compliant images from a registry and write to a block device |
| [qemuimg2disk](/qemuimg2disk/)    | Stream images and write to a block device |
| [rootio](/rootio/)                | Manage disks (partition, format etc)
| [slurp](/slurp/)                  | Stream a block device to a remote server |
| [syslinux](/syslinux/)            | Install the syslinux bootloader to a block device |
| [writefile](/writefile/)          | Write a file to a file system on a block device |

## Releases

Actions are released on a per revision basis. With each PR merged, all actions are built and pushed
to quay.io tagged with the Git revision. The `latest` tag is updated to point to the new image.

We try not to make changes that would break actions, but we do not provide a backward compatibility
guarantee. We recommend using the static Git revision tag for most deployments.

Our release process may provide stronger compatibility guarantees in the future.

## Community actions

These actions are built and maintained by third parties. To add your own action to the list, raise
a PR. If you find an action that's no longer maintained, please raise an issue.

| Name | Description | Repo |
| --- | --- | --- |
