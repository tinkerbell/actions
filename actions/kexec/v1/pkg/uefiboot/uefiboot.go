package uefiboot

import (
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/u-root/u-root/pkg/boot"
	"github.com/u-root/u-root/pkg/boot/uefi"
)

var (
	UEFIDebug       string
	UEFIBase        string
	UEFISerialAddr  string
	UEFISerialWidth string
	UEFISerialHertz string
	UEFISerialBaud  string
)

func Boot() {
	debug, _ := strconv.ParseBool(UEFIDebug)

	// Set default configuration

	var imageBase uint64
	imageBase = 0x800000

	uefiConfig := uefi.SerialPortConfig{
		Type:       uefi.SerialPortTypeIO,
		BaseAddr:   0x3f8,
		RegWidth:   1,
		InputHertz: 1843200,
		Baud:       115200,
	}

	// Parse environment variables to see if defaults need overwriting

	if UEFIBase != "" {
		base, err := strconv.ParseUint(UEFIBase, 16, 64)
		if err == nil {
			imageBase = base
		}
	}

	if UEFISerialAddr != "" {
		addr, err := strconv.ParseUint(UEFISerialAddr, 16, 64)
		if err == nil {
			uefiConfig.BaseAddr = uint32(addr)
		}
	}

	if UEFISerialWidth != "" {
		width, err := strconv.ParseUint(UEFISerialWidth, 10, 8)
		if err == nil {
			uefiConfig.RegWidth = uint32(width)
		}
	}

	if UEFISerialHertz != "" {
		hertz, err := strconv.ParseUint(UEFISerialHertz, 10, 8)
		if err == nil {
			uefiConfig.InputHertz = uint32(hertz)
		}
	}

	if UEFISerialBaud != "" {
		baud, err := strconv.ParseUint(UEFISerialBaud, 10, 8)
		if err == nil {
			uefiConfig.Baud = uint32(baud)
		}
	}

	fv, err := uefi.New("/UEFIPAYLOAD.fd")
	if err != nil {
		log.Fatal(err)
	}

	fv.ImageBase = uintptr(imageBase)
	fv.SerialConfig = uefiConfig

	log.Infof("Debug %t, Image Base %x, Base %x, Baud %d", debug, imageBase, uefiConfig.BaseAddr, uefiConfig.Baud)
	if err := fv.Load(debug); err != nil {
		log.Fatalf("Loading: %v", err)
	}

	if err := boot.Execute(); err != nil {
		log.Fatalf("Kexec: %v", err)
	}
	// If we reach here then :shrug:
	log.Infoln("Dunno.. are we booting into a new OS?")
	return

}
