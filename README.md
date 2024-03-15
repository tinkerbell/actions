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

Actions are released on a per revision basis. With each PR merged, all Actions are built and pushed
to quay.io tagged with the Git revision. The `latest` tag is updated to point to the new image.

We try not to make changes that would break Actions, but we do not provide a backward compatibility
guarantee. We recommend using the static Git revision tag for most deployments.

Our release process may provide stronger compatibility guarantees in the future.

## Community Actions

[Actions](https://tinkerbell.org/docs/concepts/templates/#action) are one of the best parts of Tinkerbell. These reusable building blocks allow us to evolve the way we provision and interact with machines. And sharing Actions is a great way to participate in this evolution. The Actions below are built and maintained by community members, like you! To add your own Action to the list, raise a PR. If you find an Action that's no longer maintained, please raise an issue or PR to have it removed.

A couple recommendations for making your Action as community friendly as possible:

- Host your Action in a container registry that's publicly accessible. Here's an [example Github Action](docs/example-publish.yaml) that builds and pushes an image to `ghcr.io`.
- Include a README with usage instructions and examples.

### Actions List

- [waitdaemon](https://github.com/jacobweinstock/waitdaemon) - Run an Action that always reports successful. Useful for reboot, poweroff, or kexec Actions.
