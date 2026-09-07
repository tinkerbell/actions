# taloscmdline action design

Date: 2026-09-07

## Goal

Add a Tinkerbell action that sets kernel command line arguments inside the
Talos Linux unified kernel image (UKI) on a freshly imaged disk, so that a node
boots with, for example,
`talos.config=http://<tinkerbell>:7080/2009-04-04/user-data` and fetches its
machine configuration from Tinkerbell's metadata service.

Talos 1.10+ metal images boot through systemd-boot on UEFI. The kernel command
line is not a text file: it is the `.cmdline` PE section of
`EFI/Linux/Talos-<version>.efi` on the FAT32 `EFI` partition. Talos re-reads
that section from the file for kexec reboots, so systemd-boot addons would be
lost on the first `talosctl reboot`; the section itself has to change.

## Inputs

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `DEST_DISK` | yes | | Whole disk holding the Talos image, e.g. `/dev/sda`. |
| `KERNEL_ARGS` | yes | | Whitespace separated arguments to set. `key=value` replaces every existing `key=...`; a bare flag is appended once. |
| `UKI_PATTERN` | no | `EFI/Linux/Talos-*.efi` | Glob, relative to the EFI partition root, of the UKIs to edit. |
| `STRIP_SIGNATURE` | no | `false` | Allow editing an Authenticode-signed UKI; the signature is removed. |

## Flow

1. Read the GPT of `DEST_DISK`, find the partition named `EFI`, derive its
   device node (`/dev/sda1`, `/dev/nvme0n1p1`, `/dev/disk/by-id/...-part1`).
2. Mount it as `vfat` on `/mountAction`; unmount on exit.
3. For each file matching the pattern: inspect the PE, merge `KERNEL_ARGS`
   into the current command line, skip when nothing changes, otherwise rewrite
   the UKI to a temporary file next to it and rename over the original.
4. Log the old and new command line for every UKI.

## UKI rewrite

Talos assembles a UKI by appending data sections (`.osrel`, `.cmdline`,
`.uname`, `.splash`, `.linux`, `.initrd`, profiles) to the systemd stub with
sequential virtual addresses aligned to `SectionAlignment` and raw sizes
aligned to `FileAlignment`. The rewrite keeps everything before `.cmdline`
byte-identical and re-lays out `.cmdline` and the sections after it:

- `.cmdline` gets `VirtualSize = len(cmdline)` and an aligned raw size, as
  Talos and ukify write it.
- Later sections keep their data and virtual sizes; virtual addresses and
  file offsets are recomputed sequentially with the same alignment.
- `SizeOfImage`, `SizeOfInitializedData` and `SizeOfCode` are recomputed;
  `CheckSum` is zeroed like Talos does. Anything after the last section (an
  Authenticode certificate table) is dropped and the security data directory
  cleared, only when `STRIP_SIGNATURE` is set; otherwise a signed UKI is an
  error.

Talos 1.10+ UKIs carry more than one `.cmdline`: one for the default boot and
one after the `.profile` sections for the "reset" boot entry. Every
`.cmdline` is rewritten, each merged independently, and the re-layout starts
at the first one.

Safety checks before writing: PE32+ only, at least one `.cmdline`, sections
from the first `.cmdline` on are data-only (no code, execute or write flags,
not `.reloc`), file order matches table order.

## Packages

- `taloscmdline/main.go`: environment, partition lookup, mount, glob, loop.
- `taloscmdline/kargs`: command line merge.
- `taloscmdline/uki`: `Inspect` and `Rewrite` for PE files; `ukitest`
  builds synthetic UKIs for tests.
- `taloscmdline/efipart`: GPT lookup by label and partition device naming.

No new module dependencies; `debug/pe` and `go-diskfs` are enough.

## Out of scope

GRUB. Dual-boot images also carry `grub/grub.cfg` on the XFS `BOOT`
partition for BIOS boots; the action only changes the UEFI path and says so in
its README.

## Testing

Synthetic UKIs (stub-like code section, `.reloc`, then the Talos data
sections) cover reading, growing, shrinking and same-size rewrites, signed
files with and without stripping, refusal of unsafe layouts, atomic
replacement, the merge semantics and partition naming. Validation against the
real Talos v1.14.0 metal image was done through a loop device and QEMU/OVMF
boots: the kernel reported the new command line both when the section kept
its raw size and when a 500-byte argument forced the later sections to move.
