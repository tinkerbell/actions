```
quay.io/tinkerbell/actions/writefile:latest
```

This action will mount a block device and write a file to a destination path on
it's filesystem.

The below example will write a file to the filesystem on the block device `/dev/sda3`.

```yaml
actions:
- name: "expand ubuntu filesystem to root"
  image: quay.io/tinkerbell/actions/writefile:latest
  timeout: 90
  environment:
      DEST_DISK: /dev/sda3
      FS_TYPE: ext4
      DEST_PATH: /etc/myconfig/foo
      CONTENTS: hello-world
      UID: 0
      GID: 0
      MODE: 0600
      DIRMODE: 0700
```
