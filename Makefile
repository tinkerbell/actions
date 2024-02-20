BINS = archive2disk cexec grub2disk kexec oci2disk qemuimg2disk rootio slurp syslinux writefile

all: $(BINS)

.PHONY: $(BINS)
$(BINS):
	make -C $@

ci: bin/gofumpt
	git ls-files '*.go' | xargs -I% sh -c 'sed -i "/^import (/,/^)/ { /^\s*$$/ d }" % && bin/gofumpt -w %'

tidy-all:
	for d in $(BINS); do (cd $$d; go mod tidy); done

include lint.mk
include rules.mk
