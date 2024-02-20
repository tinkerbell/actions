package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tinkerbell/hub/rootio/storage"
)

var metadata *storage.Metadata

// Release - this struct contains the release information populated when building rootio.
var Release struct {
	Version string
	Build   string
}

var rootioCmd = &cobra.Command{
	Use:   "rootio",
	Short: "This is a tool for managing storage for bare-metal servers",
}

func init() {
	rootioCmd.AddCommand(rootioWipe)
	rootioCmd.AddCommand(rootioFormat)
	rootioCmd.AddCommand(rootioPartition)
	rootioCmd.AddCommand(rootioMount)
	rootioCmd.AddCommand(rootioVersion)

	// Find configuration
	var err error
	if os.Getenv("TEST") != "" {
		// TEST MODE
		metadata, err = test()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		metadata, err = storage.RetrieveData()
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("Successfully parsed the MetaData, Found [%d] Disks\n", len(metadata.Instance.Storage.Disks))
}

// Execute - starts the command parsing process.
func Execute() {
	if err := rootioCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootioFormat = &cobra.Command{
	Use:   "format",
	Short: "Use rootio to format disks based upon metadata",
	Run: func(cmd *cobra.Command, args []string) {
		for fileSystem := range metadata.Instance.Storage.Filesystems {
			err := storage.FileSystemCreate(metadata.Instance.Storage.Filesystems[fileSystem])
			if err != nil {
				log.Error(err)
			}
		}
	},
}

var rootioMount = &cobra.Command{
	Use:   "mount",
	Short: "Use rootio to mount disks based upon metadata",
	Run: func(cmd *cobra.Command, args []string) {
		for fileSystem := range metadata.Instance.Storage.Filesystems {
			err := storage.Mount(metadata.Instance.Storage.Filesystems[fileSystem])
			if err != nil {
				log.Error(err)
			}
		}
	},
}

var rootioPartition = &cobra.Command{
	Use:   "partition",
	Short: "Use rootio to partition disks based upon metadata",
	Run: func(cmd *cobra.Command, args []string) {
		for disk := range metadata.Instance.Storage.Disks {
			err := storage.VerifyBlockDevice(metadata.Instance.Storage.Disks[disk].Device)
			if err != nil {
				log.Error(err)
			}
			err = storage.ExamineDisk(metadata.Instance.Storage.Disks[disk])
			if err != nil {
				log.Error(err)
			}

			if metadata.Instance.Storage.Disks[disk].WipeTable {
				err = storage.Wipe(metadata.Instance.Storage.Disks[disk])
				log.Infoln("Wiping")
				if err != nil {
					log.Error(err)
				}
			}
			log.Infoln("Partitioning")
			if os.Getenv("MBR") != "" {
				err = storage.MBRPartition(metadata.Instance.Storage.Disks[disk])
			} else {
				err = storage.Partition(metadata.Instance.Storage.Disks[disk])
			}
			if err != nil {
				log.Error(err)
			}
		}

		if len(metadata.Instance.Storage.VolumeGroups) > 0 {
			log.Infoln("Creating Volume Groups")
		}

		for _, vg := range metadata.Instance.Storage.VolumeGroups {
			if err := storage.CreateVolumeGroup(vg); err != nil {
				log.Error(err)
			}
		}
	},
}

var rootioWipe = &cobra.Command{
	Use:   "wipe",
	Short: "Use rootio to wipe disks based upon metadata",
	Run: func(cmd *cobra.Command, args []string) {
		for disk := range metadata.Instance.Storage.Disks {
			err := storage.VerifyBlockDevice(metadata.Instance.Storage.Disks[disk].Device)
			if err != nil {
				log.Error(err)
			}
			err = storage.ExamineDisk(metadata.Instance.Storage.Disks[disk])
			if err != nil {
				log.Error(err)
			}

			err = storage.Wipe(metadata.Instance.Storage.Disks[disk])
			log.Infoln("Wiping")
			if err != nil {
				log.Error(err)
			}
		}
	},
}

var rootioVersion = &cobra.Command{
	Use:   "version",
	Short: "Version and Release information about the rootio storage manager",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("rootio Release Information\n")
		fmt.Printf("Version:  %s\n", Release.Version)
		fmt.Printf("Build:    %s\n", Release.Build)
	},
}

func test() (*storage.Metadata, error) {
	// Open our jsonFile
	jsonFile, err := os.Open(os.Getenv("JSON_FILE"))
	// if we os.Open returns an error then handle it
	if err != nil {
		return nil, err
	}
	fmt.Println("Successfully Opened test data")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	// we initialize our Users array
	var w storage.Wrapper

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	if err := json.Unmarshal(byteValue, &w); err != nil {
		return nil, err
	}

	return &w.Metadata, nil
}
