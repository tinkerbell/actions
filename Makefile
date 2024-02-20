BINS = archive2disk cexec grub2disk kexec oci2disk qemuimg2disk rootio slurp syslinux writefile

all: $(BINS)

.PHONY: $(BINS)
$(BINS):
	make -C $@

ci: bin/gofumpt
	./hack/ci-check.sh

include lint.mk
include rules.mk
