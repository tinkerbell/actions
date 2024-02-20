The `grub2disk` mounts and writes a grub onto a block device. This lets you see a grub menu during the boot process and select OS of your choice and which is deployed in the system. As a part of the process, [chroot](https://en.wikipedia.org/wiki/Chroot) operation is also performed on `GRUB_INSTALL_PATH`. For this reason respective `volumes` as described in the below template is required.

```yaml
version: "0.1"
name: centos-install
global_timeout: 1800
tasks:
  - name: "centos-install"
    worker: "{{.device_1}}"
    volumes:
      - /dev:/dev
      - /proc:/proc
      - /sys:/sys
      - /mnt:/mnt
      - /statedir:/statedir
      - /etc:/etc
      - /bin:/bin
      - /sbin:/sbin
      - /var:/var
      - /usr:/usr
      - /lib:/lib
      - /tmp:/tmp
      - /boot:/boot
    actions:
      - name: "grub_2_disk"
        image: grub2disk
        timeout: 180
        environment:
          GRUB_INSTALL_PATH: /dev/sda
          GRUB_DISK: /dev/sda1
          FS_TYPE: ext4
```

Below are the details which are needed for this action

```shel
GRUB_INSTALL_PATH: /dev/sda <path-onto-which-grub-will-be-written>
GRUB_DISK: /dev/sda1 <block-device-which-will-be-mounted-&-chrooted>
FS_TYPE: etx4 <filesystem-type-of-grub-disk>
```

Once the process is completed. You can reboot the system and jump to boot menu. Following are sample grub menu, which should be visible.

![sample grub 1](sample_grub_menu_1.png)

![sample grub 2](sample_grub_menu_1.png)
