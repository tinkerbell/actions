# Tinkerbell `cidataio` Action

This action creates a `cidata` partition and writes cloud-init data to it.

## Description

The `cidataio` action is a Go-based tool designed to run in a Tinkerbell workflow. It prepares a disk that has just been flashed with an OS image (e.g., Talos "nocloud") by adding a new partition for cloud-init data.

It performs the following steps:
1. Finds the target disk from the `DEST_DISK` environment variable.
2. Creates a new partition using all remaining free space on the disk.
3. Formats this new partition as `vfat` with the label `cidata`.
4. Mounts the partition.
5. Writes the contents of `USER_DATA`, `META_DATA`, and `VENDOR_DATA` environment variables to `user-data`, `meta-data`, and `vendor-data` files, respectively.
6. Unmounts the partition.

## Environment Variables

* **`DEST_DISK`** (Required): The block device to operate on (e.g., `/dev/sda`, `/dev/nvme0n1`).
* **`USER_DATA`** (Optional): The content for the `user-data` file.
* **`META_DATA`** (Optional): The content for the `meta-data` file.
* **`NETWORK_CONFIG`** (Optional): The content for the `network-config` file.

## Example Workflow YAML

This action is typically run immediately after `image2disk`.

```yaml
actions:
  - name: "stream talos nocloud image"
    image: quay.io/tinkerbell/actions/image2disk:latest
    timeout: 9600
    environment:
      DEST_DISK: {{ index .Hardware.Disks 0 }}
      IMG_URL: "..."
      COMPRESSED: "true"

  - name: "create cidata partition and write files"
    image: ghcr.io/tinkerbell/cidataio:latest
    timeout: 120
    environment:
      DEST_DISK: {{ index .Hardware.Disks 0 }}
      USER_DATA: |
        # user-data content here
      META_DATA: |
        local-hostname: my-node-1
```
