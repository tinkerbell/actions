module github.com/tinkerbell/actions

go 1.15

require (
	github.com/Azure/go-ansiterm v0.0.0-20210608223527-2377c96fe795 // indirect
	github.com/containerd/console v1.0.1
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.5
	github.com/mattn/godown v0.0.0-20201027140031-2c7783b24de7
	github.com/moby/buildkit v0.8.3
	github.com/moby/term v0.0.0-20201110203204-bea5bbe245bf // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	github.com/yuin/goldmark v1.2.1
	github.com/yuin/goldmark-meta v1.0.0
	go.uber.org/zap v1.16.0
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
)

replace (
	// containerd: Forked from 0edc412565dcc6e3d6125ff9e4b009ad4b89c638 (20201117) with:
	// - `Adjust overlay tests to expect "index=off"`        (#4719, for ease of cherry-picking #5076)
	// - `overlay: support "userxattr" option (kernel 5.11)` (#5076)
	// - `docker: avoid concurrent map access panic`         (#4855)
	github.com/containerd/containerd => github.com/AkihiroSuda/containerd v1.1.1-0.20210312044057-48f85a131bb8
	// protobuf: corresponds to containerd
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.5
	github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe
	github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
	// genproto: corresponds to containerd
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63
)
