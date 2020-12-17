package artifacthub

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"gopkg.in/yaml.v2"
)

// Manifest represents the ArtifactHub manifest file that gets used to populate
// the website.
type Manifest struct {
	Version          string `yaml:"version"`
	Name             string `yaml:"name"`
	DisplayName      string `yaml:"displayName"`
	CreatedAt        string `yaml:"createdAt"`
	Description      string `yaml:"description"`
	LogoPath         string `yaml:"logoPath"`
	Digest           string `yaml:"digest,omitempty"`
	License          string `yaml:"license,omitempty"`
	HomeURL          string `yaml:"homeURL,omitempty"`
	AppVersion       string `yaml:"appVersion,omitempty"`
	ContainersImages []struct {
		Name  string `yaml:"name,omitempty"`
		Image string `yaml:"image,omitempty"`
		//Whitelisted string `yaml:"whitelisted,omitempty"`
	} `yaml:"containersImages,omitempty"`
	//ContainsSecurityUpdates string   `yaml:"containsSecurityUpdates,omitempty"`
	//Operator                string   `yaml:"operator,omitempty"`
	//Deprecated              string   `yaml:"deprecated,omitempty"`
	//Prerelease              string   `yaml:"prerelease,omitempty"`
	Keywords []string `yaml:"keywords,omitempty"`
	Links    []struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
	} `yaml:"links,omitempty"`
	Readme string `yaml:"readme,omitempty"`
	//Install     string   `yaml:"install,omitempty"`
	//Changes     []string `yaml:"changes,omitempty"`
	//Maintainers []struct {
	//Name  string `yaml:"name"`
	//Email string `yaml:"email"`
	//} `yaml:"maintainers,omitempty"`
	Provider struct {
		Name string `yaml:"name"`
	} `yaml:"provider"`
	//Ignore []string `yaml:"ignore"`
}

func PopulateFromActionMarkdown(file io.Reader, m *Manifest) error {
	md := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)
	readme, err := ioutil.ReadAll(file)
	if err != nil {
		return errors.Wrap(err, "error reading the README.md proposal")
	}
	var buf bytes.Buffer
	context := parser.NewContext()
	if err := md.Convert(readme, &buf, parser.WithContext(context)); err != nil {
		return errors.Wrap(err, "error converting the README.md proposal")
	}
	metaData := meta.Get(context)

	m.Name = metaData["slug"].(string)
	m.DisplayName = metaData["name"].(string)
	m.Readme = buf.String()
	m.Version = metaData["version"].(string)
	m.AppVersion = m.Version
	m.Keywords = strings.Split(metaData["tags"].(string), ",")
	m.Description = metaData["description"].(string)

	m.ContainersImages = []struct {
		Name  string `yaml:"name,omitempty"`
		Image string `yaml:"image,omitempty"`
		//Whitelisted string `yaml:"whitelisted,omitempty"`
	}{
		{
			Name:  fmt.Sprintf("quay.io/tinkerbell-actions/%s:%s", m.Name, m.Version),
			Image: fmt.Sprintf("quay.io/tinkerbell-actions/%s:%s", m.Name, m.Version),
		},
	}

	if _, err := time.Parse("January 2, 2006", metaData["createdAt"].(string)); err != nil {
		println(fmt.Sprintf("action: %s error converting createdAt right format is \"January 2, 2016\" got %s", m.Name, metaData["createdAt"].(string)))
	} else {
		m.CreatedAt = metaData["createdAt"].(string)
	}

	return nil
}

func WriteToFile(manifest *Manifest, dst string) error {
	b, err := yaml.Marshal(manifest)
	if err != nil {
		return errors.Wrap(err, "error marshalling manifest to yaml")
	}
	dstFile, err := os.OpenFile(dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrap(err, "error creating manifest file")
	}
	_, err = dstFile.Write(b)
	if err != nil {
		return errors.Wrap(err, "error writing manifest to file")
	}
	return nil
}
