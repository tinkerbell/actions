package types

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type ExportedCacher struct {
	ID                                 string                   `json:"id"`
	Metadata                           Metadata                 `jsonm:"metadata"`
}

type Metadata struct {
	Arch                               string                   `json:"arch"`
	State                              string                   `json:"state"`
	EFIBoot                            bool                     `json:"efi_boot"`
	Instance                           Instance                 `json:"instance,omitempty"`
	PreinstalledOperatingSystemVersion interface{}              `json:"preinstalled_operating_system_version"`
	NetworkPorts                       []map[string]interface{} `json:"network_ports"`
	PlanSlug                           string                   `json:"plan_slug"`
	Facility                           string                   `json:"facility_code"`
	Hostname                           string                   `json:"hostname"`
	BondingMode                        int                      `json:"bonding_mode"`
}

type Instance struct {
	ID       string `json:"id,omitempty"`
	State    string `json:"state,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	AllowPXE bool   `json:"allow_pxe,omitempty"`
	Rescue   bool   `json:"rescue,omitempty"`

	IPAddresses []map[string]interface{} `json:"ip_addresses,omitempty"`
	OS          OperatingSystem         `json:"operating_system_version,omitempty"`
	UserData    string                   `json:"userdata,omitempty"`

	CryptedRootPassword string `json:"crypted_root_password,omitempty"`

	Storage      Storage `json:"storage,omitempty"`
	SSHKeys      []string `json:"ssh_keys,omitempty"`
	NetworkReady bool     `json:"network_ready,omitempty"`
}

type OperatingSystem struct {
	Slug     string `json:"slug"`
	Distro   string `json:"distro"`
	Version  string `json:"version"`
	ImageTag string `json:"image_tag"`
	OsSlug   string `json:"os_slug"`
}



type File struct {
	Path     string `json:"path"`
	Contents string `json:"contents,omitempty"`
	Mode     int    `json:"mode,omitempty"`
	UID      int    `json:"uid,omitempty"`
	GID      int    `json:"gid,omitempty"`
}

//type Filesystem struct {
//	Mount struct {
//		Device string             `json:"device"`
//		Format string             `json:"format"`
//		Files  []*File            `json:"files,omitempty"`
//		Create *FilesystemOptions `json:"create,omitempty"`
//		Point  string             `json:"point"`
//	} `json:"mount"`
//}

type FilesystemOptions struct {
	Force   bool     `json:"force,omitempty"`
	Options []string `json:"options,omitempty"`
}

type Raid struct {
	Name    string   `json:"name"`
	Level   string   `json:"level"`
	Devices []string `json:"devices"`
	Spares  int      `json:"spares,omitempty"`
}

type Storage struct {
	Disks       []Disk       `json:"disks,omitempty"`
	RAID        []Raid       `json:"raid,omitempty"`
	Filesystems []Filesystem `json:"filesystems,omitempty"`
}

// Metadata struct
// This is an auto generated struct taken from a metadata request
type Metadata_Instance struct {
	CryptedRootPassword    string `json:"crypted_root_password"`
	Hostname               string `json:"hostname"`
	OperatingSystemVersion struct {
		Distro     string `json:"distro"`
		OsCodename string `json:"os_codename"`
		OsSlug     string `json:"os_slug"`
		Version    string `json:"version"`
	} `json:"operating_system_version"`
	Storage struct {
		Disks       []Disk       `json:"disks"`
		Filesystems []Filesystem `json:"filesystems"`
	} `json:"storage"`
}

//Filesystem defines the organisation of a filesystem
type Filesystem struct {
		Mount struct {
		Create struct {
			Options []string `json:"options"`
		} `json:"create"`
		Device string `json:"device"`
		Format string `json:"format"`
		Point  string `json:"point"`
	} `json:"mount"`
}

//Disk defines the configuration for a disk
type Disk struct {
	Device     string       `json:"device"`
	Partitions []Partitions `json:"partitions"`
	WipeTable  bool         `json:"wipe_table"`
}

//Partitions details the architecture
type Partitions struct {
	Label  string `json:"label"`
	Number int    `json:"number"`
	Size   uint64 `json:"size"`
}

//RetreieveData -
func RetreieveData() (*Instance, error) {
	metadataURL := os.Getenv("MIRROR_HOST")
	if metadataURL == "" {
		return nil, fmt.Errorf("Unable to discover the metadata server from environment variable [MIRROR_HOST]")
	}

	metadataClient := http.Client{
		Timeout: time.Second * 60, // Timeout after 60 seconds (seems massively long is this dial-up?)
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:50061/metadata", metadataURL), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "bootkit")

	res, getErr := metadataClient.Do(req)
	if getErr != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, err
	}

	var exportedcacher ExportedCacher
	//var mdata Metadata

	jsonErr := json.Unmarshal(body, &exportedcacher)
	if jsonErr != nil {
		return nil, jsonErr
	}

	return &exportedcacher.Metadata.Instance, nil
}
