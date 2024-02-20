ifeq ($(origin GOBIN), undefined)
GOBIN := ${PWD}/bin
export GOBIN
PATH := ${GOBIN}:${PATH}
export PATH
endif

toolsBins := $(addprefix bin/,$(notdir $(shell awk -F'"' '/^\s*_/ {print $$2}' tools/tools.go)))

# installs cli tools defined in tools/tools.go
$(toolsBins): tools/go.mod tools/go.sum tools/tools.go
$(toolsBins): CMD=$(shell awk -F'"' '/$(@F)"/ {print $$2}' tools/tools.go)
$(toolsBins):
	go install $(CMD)
