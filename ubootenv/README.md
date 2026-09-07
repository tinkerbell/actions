# UBoot Env

```
quay.io/tinkerbell/actions/ubootenv:latest
```

This action reads and modifies U-Boot environment variables stored in a `uboot.env` file
on a VFAT partition. It is a pure Go port of the U-Boot `fw_setenv` utility, supporting
the non-redundant VFAT file format (offset `0x0000`, default size `0x4000`).

Environment variables are provided as a JSON object via the `ENV_VARS` environment variable.
Setting a variable's value to an empty string (`""`) deletes it from the environment.

## Environment Variables

| Variable      | Description                                            | Required | Default          |
|---------------|--------------------------------------------------------|----------|------------------|
| `DEST_DISK`   | Block device / partition to mount (e.g. `/dev/sda1`)   | Yes      |                  |
| `FS_TYPE`     | Filesystem type for mounting                           | No       | `vfat`           |
| `ENV_FILE`    | Path to `uboot.env` relative to mount root            | No       | `/boot/uboot.env`|
| `ENV_VARS`    | JSON object of key/value pairs to set                  | Yes      |                  |

## Example Workflow Action

```yaml
actions:
  - name: "set u-boot environment"
    image: quay.io/tinkerbell/actions/ubootenv:latest
    timeout: 90
    environment:
      DEST_DISK: /dev/sda1
      FS_TYPE: vfat
      ENV_FILE: /boot/uboot.env
      ENV_VARS: '{"bootcmd":"run distro_bootcmd","bootdelay":"3"}'
```

## Deleting Variables

To delete a variable, set its value to an empty string:

```yaml
      ENV_VARS: '{"obsolete_var":""}'
```

## U-Boot Environment Format

The action expects the standard non-redundant U-Boot environment file format:

| Offset | Size          | Field  |
|--------|---------------|--------|
| 0x0000 | 4 bytes       | CRC32 (IEEE, little-endian, computed over data) |
| 0x0004 | env_size - 4  | Data: null-terminated `key=value` pairs, double-null terminated, zero-padded |

Default environment size: `0x4000` (16,384 bytes).
