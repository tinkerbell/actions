package image

// This file probes the block geometry of the destination device. It is Linux
// only: image2disk writes to Linux block devices and nothing else.

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// defaultBlockSize is used when the destination cannot report a block size,
// which happens for regular files. 4096 is the modern physical sector size and
// a safe multiple of the 512-byte logical size older devices report.
const defaultBlockSize = 4096

// maxChunkSize caps the pipeline chunk size. Beyond this a single pwrite stops
// getting faster and only costs latency and memory.
const maxChunkSize = 64 << 20

var (
	// errNotBlockDevice reports that the destination does not answer block
	// ioctls, which is the normal case for a regular file.
	errNotBlockDevice = errors.New("not a block device")

	// ErrImageTooLarge reports that the decoded image does not fit the destination.
	ErrImageTooLarge = errors.New("image is larger than the destination device")
)

// Geometry describes the block geometry of a write destination.
type Geometry struct {
	// LogicalBlockSize is the smallest addressable unit (BLKSSZGET).
	LogicalBlockSize int
	// PhysicalBlockSize is the hardware sector size (BLKPBSZGET).
	PhysicalBlockSize int
	// MinIO is the smallest efficient IO size (BLKIOMIN).
	MinIO int
	// OptIO is the optimal IO size, such as a RAID stripe width (BLKIOOPT).
	// It is zero when the device reports none.
	OptIO int
	// SizeBytes is the device capacity (BLKGETSIZE64), or the file size for a
	// regular file. Zero means unknown, in which case no bounds check is possible.
	SizeBytes int64
	// Rotational reports spinning media (BLKROTATIONAL). Concurrent writes hurt
	// on rotational devices because seek ordering matters.
	Rotational bool
	// IsBlockDevice is false for regular files.
	IsBlockDevice bool
}

// ProbeGeometry inspects f and returns its block geometry. A destination that
// does not answer block ioctls, such as a regular file, yields conservative
// defaults rather than an error.
func ProbeGeometry(f *os.File) (Geometry, error) {
	st, statErr := f.Stat()

	rc, err := f.SyscallConn()
	if err != nil {
		return Geometry{}, fmt.Errorf("syscall conn for %s: %w", f.Name(), err)
	}

	var (
		g     Geometry
		inner error
	)
	if cerr := rc.Control(func(fd uintptr) { inner = probeFD(int(fd), &g) }); cerr != nil {
		return g, fmt.Errorf("control fd for %s: %w", f.Name(), cerr)
	}

	if errors.Is(inner, errNotBlockDevice) {
		// A regular file: fall back to its current size.
		g = Geometry{}
		if statErr == nil {
			g.SizeBytes = st.Size()
		}
		return g.normalize(), nil
	}
	if inner != nil {
		return g, inner
	}
	return g.normalize(), nil
}

// probeFD issues the BLK* ioctls against fd and fills in g.
func probeFD(fd int, g *Geometry) error {
	// BLKSSZGET is the canary. A regular file answers ENOTTY; some filesystems
	// answer EINVAL and a seccomp sandbox can answer ENOSYS.
	ssz, err := ioctlUint32(fd, unix.BLKSSZGET)
	switch {
	case err == nil:
		g.LogicalBlockSize = int(ssz)
		g.IsBlockDevice = true
	case errors.Is(err, unix.ENOTTY), errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOSYS):
		return errNotBlockDevice
	default:
		return fmt.Errorf("ioctl BLKSSZGET: %w", err)
	}

	// The rest are advisory: a device that will not answer is not an error.
	if v, e := ioctlUint32(fd, unix.BLKPBSZGET); e == nil {
		g.PhysicalBlockSize = int(v)
	}
	if v, e := ioctlUint32(fd, unix.BLKIOMIN); e == nil {
		g.MinIO = int(v)
	}
	if v, e := ioctlUint32(fd, unix.BLKIOOPT); e == nil {
		g.OptIO = int(v)
	}
	if v, e := ioctlUint16(fd, unix.BLKROTATIONAL); e == nil {
		g.Rotational = v != 0
	}

	sz, err := ioctlUint64(fd, unix.BLKGETSIZE64)
	if err != nil {
		return fmt.Errorf("ioctl BLKGETSIZE64: %w", err)
	}
	g.SizeBytes = int64(sz)

	return nil
}

// normalize fills in derived defaults so no field is zero where zero is invalid.
func (g Geometry) normalize() Geometry {
	if g.LogicalBlockSize <= 0 {
		g.LogicalBlockSize = defaultBlockSize
	}
	if g.PhysicalBlockSize < g.LogicalBlockSize {
		g.PhysicalBlockSize = g.LogicalBlockSize
	}
	if g.MinIO < g.PhysicalBlockSize {
		g.MinIO = g.PhysicalBlockSize
	}
	return g
}

// Align returns the byte alignment that chunk offsets and sizes must respect.
func (g Geometry) Align() int {
	a := max(g.PhysicalBlockSize, g.MinIO, defaultBlockSize)
	// Only adopt OptIO when it is a whole multiple of the block size. Real RAID
	// arrays report non-power-of-two stripe widths (786432 for a 3-disk stripe),
	// and aligning to one of those would misalign every block below it.
	if g.OptIO > a && g.OptIO%a == 0 {
		a = g.OptIO
	}
	return a
}

// AlignChunkSize rounds target down to a whole multiple of Align, clamped to
// one alignment unit at the low end and maxChunkSize at the high end.
func (g Geometry) AlignChunkSize(target int) int {
	a := g.Align()
	if target > maxChunkSize {
		target = maxChunkSize
	}
	if n := (target / a) * a; n >= a {
		return n
	}
	return a
}

// DiffBlockSize is the granularity at which the diff writer compares.
func (g Geometry) DiffBlockSize() int {
	return max(g.PhysicalBlockSize, defaultBlockSize)
}

// RereadPartitionTable asks the kernel to re-scan f's partition table, which is
// the equivalent of partprobe. It fails with EINVAL when f is a partition
// rather than a whole disk; callers treat that as non-fatal.
func RereadPartitionTable(f *os.File) error {
	rc, err := f.SyscallConn()
	if err != nil {
		return fmt.Errorf("syscall conn: %w", err)
	}
	var inner error
	if cerr := rc.Control(func(fd uintptr) {
		inner = unix.IoctlSetInt(int(fd), unix.BLKRRPART, 0)
	}); cerr != nil {
		return fmt.Errorf("control fd: %w", cerr)
	}
	if inner != nil {
		return fmt.Errorf("ioctl BLKRRPART: %w", inner)
	}
	return nil
}

// ioctlPtr issues an ioctl whose argument is a pointer to a result. It mirrors
// the unexported helper of the same name in golang.org/x/sys/unix; the uintptr
// conversion is written inside the Syscall argument list, which is the only
// form the unsafe.Pointer rules permit.
func ioctlPtr(fd int, req uint, arg unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// ioctlUint16 issues an ioctl that writes an unsigned short, such as BLKROTATIONAL.
func ioctlUint16(fd int, req uint) (uint16, error) {
	var v uint16
	err := ioctlPtr(fd, req, unsafe.Pointer(&v))
	return v, err
}

// ioctlUint32 issues an ioctl that writes an int or unsigned int. The BLK* size
// ioctls all use put_int/put_uint, so the destination must be four bytes wide:
// unix.IoctlGetInt would hand the kernel a pointer to an eight-byte Go int.
func ioctlUint32(fd int, req uint) (uint32, error) {
	var v uint32
	err := ioctlPtr(fd, req, unsafe.Pointer(&v))
	return v, err
}

// ioctlUint64 issues an ioctl that writes a u64, such as BLKGETSIZE64.
// golang.org/x/sys/unix has no IoctlGetUint64 helper, hence the raw call.
func ioctlUint64(fd int, req uint) (uint64, error) {
	var v uint64
	err := ioctlPtr(fd, req, unsafe.Pointer(&v))
	return v, err
}
