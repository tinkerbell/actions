```
quay.io/tinkerbell/actions/cexec:latest
```

The `cexec` Action performs *execution* either within a [chroot](https://en.wikipedia.org/wiki/Chroot) environment
or within the base filesystem. The primary use-case is being able to provision
files/an Operating System to disk and then being able to execute something that
perhaps resides within that filesystem.

## Examples

```yaml
actions:
- name: "Install Grub"
  image: quay.io/tinkerbell/actions/cexec:latest
  timeout: 90
  environment:
      BLOCK_DEVICE: /dev/sda3
      FS_TYPE: ext4
      CHROOT: y
      CMD_LINE: "grub-install --root-directory=/boot /dev/sda"
```

In order to execute multiple commands (separated by a semi-colon) we will
need to leverage a shell. We do this by passing `sh -c` as a `DEFAULT_INTERPRETER`.
This interpreter will then parse your commands.

```yaml
actions:
- name: "Update packages"
  image: quay.io/tinkerbell/actions/cexec:latest
  timeout: 90
  environment:
      BLOCK_DEVICE: /dev/sda3
      FS_TYPE: ext4
      CHROOT: y
      DEFAULT_INTERPRETER: "/bin/sh -c"
      CMD_LINE: "apt-get -y update; apt-get -y upgrade"
      UPDATE_RESOLV_CONF: true
      DEBIAN_FRONTEND: noninteractive
```

### Environment variables and CLI flags

All options can be set either via environment variables or CLI flags.
CLI flags take precedence over environment variables, which take precedence over default values.

| Env variable | Flag | Type | Default Value | Required | Description |
|--------------|------|------|---------------|----------|-------------|
| `BLOCK_DEVICE` | `--block-device` | string | "" | yes | The block device to mount. |
| `FS_TYPE` | `--fs-type` | string | "" | yes | The filesystem type of the block device. |
| `CHROOT` | `--chroot` | string | "" | no | If set to `y` (or a non empty string), the Action will execute the given command within a chroot environment. This option is DEPRECATED. Future versions will always chroot. |
| `CMD_LINE` | `--cmd-line` | string | "" | yes | The command to execute. |
| `DEFAULT_INTERPRETER` | `--default-interpreter` | string | "" | no | The default interpreter to use when executing commands. This is useful when you need to execute multiple commands. |
| `UPDATE_RESOLV_CONF` | `--update-resolv-conf` | boolean | false | no | If set to `true`, the cexec Action will update the `/etc/resolv.conf` file within the chroot environment with the `/etc/resolv.conf` from the host. |
| `JSON_OUTPUT` | `--json-output` | boolean | true | no | If set to `true`, the cexec Action will log output in JSON format. The defaults to `true`. If set to `false`, the cexec Action will log output in plain text format. |

Any environment variables you set on the Action will be available to the command you execute.
For example, if you set `DEBIAN_FRONTEND: noninteractive` as an environment variable, it will be available to the command you execute.

### Exit codes

The following exit codes or statuses are returned by the `cexec` Action:

| Code | Description |
|------|-------------|
| 0 | The cexec Action was executed successfully. |
| 10 | The was a failure parsing cli flags and/or env variables. |
| 20 | Required cli flags and/or env variables were not specified. |
| 30 | The cexec Action failed to execute successfully. |
