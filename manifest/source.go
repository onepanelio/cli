package manifest

import (
	"fmt"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/github"
	"log"
	"os"
	"strings"
)

type Source interface {
	// If keep root directory is true, all of the manifest files will be under a directory like the
	// release name for Github. If False, just the manifest files are moved over.
	MoveToDirectory(destinationPath string, keepRootDirectory bool) error
	// Get the resulting manifest path. Should only be called after MoveToDirectory
	GetManifestPath() (string, error)
}

type GithubSource struct {
	tag string // The tag of the release. latest is also accepted.
	overrideCache bool // if true, will override the local cached files.
	release *github.Release
	moved bool // true if MoveToDirectory has been called
	destination string // the directory to move the manifest files to
}

func CreateGithubSource(tag string, overrideCache bool) (*GithubSource, error) {
	source := &GithubSource{
		tag:tag,
		overrideCache:overrideCache,
		moved: false,
	}

	return source, nil
}

func (g* GithubSource) getTagDownloadUrl() (string, error) {
	if g.release == nil {
		githubApi, err := github.New("https://api.github.com/repos/onepanelio/manifests")
		if err != nil {
			return "", err
		}

		release := &github.Release{}

		if g.tag == "latest" {
			release, err = githubApi.GetLatestRelease()
			if err != nil {
				return "", err
			}
		} else {
			release, err = githubApi.GetReleaseByTag(g.tag)
			if err != nil {
				return "", err
			}
		}

		g.release = release
	}

	return g.release.ZipBallUrl, nil
}

func (g *GithubSource) getManifestPath(directoryPath string) string {
	return directoryPath + string(os.PathSeparator) + g.release.TagName
}

func (g* GithubSource) GetManifestPath() (string, error) {
	if !g.moved {
		return "", fmt.Errorf("files not yet moved. Unable to get manifest path")
	}

	return g.getManifestPath(g.destination), nil
}

func (g* GithubSource) MoveToDirectory(directoryPath string, keepRootDirectory bool) error {
	g.destination = directoryPath

	tempManifestsPath := ".temp_manifests"

	defer func () {
		_, err := files.DeleteIfExists(tempManifestsPath)
		if err != nil {
			log.Printf("[error] Deleting %v: %v", tempManifestsPath, err.Error())
		}
	}()

	sourceUrl, err := g.getTagDownloadUrl()
	if err != nil {
		return err
	}

	finalManifestPath := g.getManifestPath(directoryPath)

	cacheExists, err := files.Exists(finalManifestPath)
	if err != nil {
		return err
	}

	if !g.overrideCache && cacheExists {
		g.moved = true
		return nil
	}

	if err := os.RemoveAll(finalManifestPath); err != nil {
		return err
	}

	if err := files.DownloadFile(tempManifestsPath, sourceUrl); err != nil {
		log.Printf("[error] Downloading %v: error %v", sourceUrl, err.Error())
		return err
	}

	unzippedFiles, err := files.Unzip(tempManifestsPath, directoryPath)
	if err != nil {
		return err
	}

	if len(unzippedFiles) == 0 {
		return nil
	}

	if keepRootDirectory {
		if err := os.Rename(unzippedFiles[0], directoryPath + string(os.PathSeparator) +  g.release.TagName); err != nil {
			return err
		}
		return nil
	}

	rootFolder := unzippedFiles[0]

	if err := files.CopyDirChildren(rootFolder, directoryPath); err != nil {
		return err
	}

	err = os.RemoveAll(rootFolder)

	g.moved = true

	return err
}

type DirectorySource struct {
	sourceDirectory string
	overrideCache bool // if true, will override the local cached files.
	moved bool // true if MoveToDirectory has been called
	destination string // the directory to move the manifest files to
}

func CreateDirectorySource(sourceDirectory string, overrideCache bool) (*DirectorySource, error) {
	source := &DirectorySource{
		sourceDirectory: sourceDirectory,
		overrideCache:overrideCache,
		moved: false,
	}

	return source, nil
}

func (d* DirectorySource) getManifestPath(directoryPath string) string {
	lastPathSeparatorIndex := strings.LastIndex(d.sourceDirectory, string(os.PathSeparator))
	if lastPathSeparatorIndex < 0  {
		return directoryPath + string(os.PathSeparator) + d.sourceDirectory
	}

	destinationDirectoryName := d.sourceDirectory[lastPathSeparatorIndex:]

	return directoryPath + string(os.PathSeparator) + destinationDirectoryName
}

func (d* DirectorySource) GetManifestPath() (string, error) {
	if !d.moved {
		return "", fmt.Errorf("files not yet moved. Unable to get manifest path")
	}

	return d.getManifestPath(d.destination), nil
}

// TODO remove keep root directory
func (d* DirectorySource) MoveToDirectory(directoryPath string, keepRootDirectory bool) error {
	d.destination = directoryPath

	finalManifestPath := d.getManifestPath(directoryPath)

	cacheExists, err := files.Exists(finalManifestPath)
	if err != nil {
		return err
	}

	if !d.overrideCache && cacheExists {
		d.moved = true
		return nil
	}

	if err := os.RemoveAll(finalManifestPath); err != nil {
		return err
	}

	if err := files.CopyDir(d.sourceDirectory, finalManifestPath); err != nil {
		return err
	}

	d.moved = true

	return err
}
