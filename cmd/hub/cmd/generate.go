package cmd

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/tinkerbell/actions/pkg/artifacthub"
)

type generateOptions struct {
	context string
	output  string
}

var generateOpts = &generateOptions{}

var generateCmd = &cobra.Command{
	Use:   "generate [--context .]",
	Short: "Generate the static website",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate(generateOpts)
	},
}

func init() {
	generateCmd.PersistentFlags().StringVar(&generateOpts.context, "context", ".", "base path for the proposals repository in your local file system")
	generateCmd.PersistentFlags().StringVar(&generateOpts.output, "output", "./artifacthub-manifests/actions", "where the generate website will be stored")

	rootCmd.AddCommand(generateCmd)
}

func runGenerate(opts *generateOptions) error {
	actionsPath := path.Join(opts.context, "actions")
	info, err := os.Stat(actionsPath)
	if os.IsNotExist(err) {
		return errors.Wrap(err, "we expect an actions directory inside the repository.")
	}
	if !info.IsDir() {
		return errors.New("the expected actions directory has to be a directory, not a file")
	}

	actionsDir, err := ioutil.ReadDir(actionsPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(generateOpts.output); os.IsNotExist(err) {
		if err := os.Mkdir(generateOpts.output, 0700); err != nil {
			return err
		}
	}

	// This is the manifest with all the pre-populated information. Ideally
	// this has to be populated from an outside configuration file.
	manifest := &artifacthub.Manifest{
		Provider: struct {
			Name string `yaml:"name"`
		}{
			Name: "tinkerbell-community",
		},
		HomeURL:  "https://github.com/tinkerbell/actions",
		LogoPath: "./../../logo.png",
		License:  "Apache-2",
		Links: []struct {
			Name string `yaml:"name"`
			URL  string `yaml:"url"`
		}{
			{
				Name: "website",
				URL:  "https://tinkerbell.org/",
			},
			{
				Name: "support",
				URL:  "https://github.com/tinkerbell/actions/issues",
			},
		},
	}

	// ./actions/disk-wipe/v1
	//              ↑ _____ we are here
	for _, actionPath := range actionsDir {
		versionDir, err := ioutil.ReadDir(path.Join(actionsPath, actionPath.Name()))
		if err != nil {
			return err
		}
		// ./actions/disk-wipe/v1
		//                      ↑ _____ we are here
		for _, v := range versionDir {
			readmeFile, err := os.Open(path.Join(actionsPath, actionPath.Name(), v.Name(), "README.md"))
			if err != nil {
				return errors.Wrap(err, "error reading the README.md proposal")
			}

			if err := artifacthub.PopulateFromActionMarkdown(readmeFile, manifest); err != nil {
				return errors.Wrap(err, "error converting the README.md to an ArtifactHub manifest")
			}

			if err := artifacthub.WriteToFile(manifest, generateOpts.output); err != nil {
				return errors.Wrap(err, "error writing manifest to a file")
			}
		}
	}
	return nil
}
