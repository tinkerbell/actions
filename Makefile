BINS = archive2disk cexec grub2disk kexec oci2disk qemuimg2disk rootio slurp syslinux writefile

all: $(BINS)

.PHONY: $(BINS)
$(BINS):
	make -C $@

include rules.mk
include lint.mk

formatters: $(toolBins)
	git ls-files '*.go' | xargs -I% sh -c 'sed -i "/^import (/,/^)/ { /^\s*$$/ d }" % && bin/gofumpt -w %'
	git ls-files '*.go' | xargs -I% bin/goimports -w %

tidy-all:
	(cd tools; go mod tidy)
	for d in $(BINS); do (cd $$d; go mod tidy); done
