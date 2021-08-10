module github.com/tinkerbell/hub/actions/kexec/v1

go 1.15

require (
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.1
	github.com/u-root/u-root v0.0.0-20210803232242-f0689d32e2e8 // indirect
	golang.org/x/sys v0.0.0-20210525143221-35b2ab0089ea
)

//replace github.com/u-root/u-root/pkg/boot/uefi => /go/src/github.com/u-root/u-root/pkg/boot/uefi
