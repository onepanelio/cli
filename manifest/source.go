package manifest

import (
	"github.com/onepanelio/cli/files"
	"log"
	"os"
)

type Source interface {
	// If keep root directory is true, all of the manifest files will be under a directory like the
	// release name for Github. If False, just the manifest files are moved over.
	MoveToDirectory(destinationPath string, keepRootDirectory bool) error
}


// Downloads a zip from Github and moves the contents to a directory.
type GithubSource struct {
	urlSource string
}

func CreateGithubSource(zipUrl string) (*GithubSource, error) {
	return &GithubSource{urlSource:zipUrl}, nil
}

func (g* GithubSource) MoveToDirectory(directoryPath string, keepRootDirectory bool) error {
	tempManifestsPath := ".temp_manifests"

	defer func () {
		_, err := files.DeleteIfExists(tempManifestsPath)
		if err != nil {
			log.Printf("[error] Deleting %v: %v", tempManifestsPath, err.Error())
		}
	}()

	if err := files.DownloadFile(tempManifestsPath, g.urlSource); err != nil {
		log.Printf("[error] Downloading %v: error %v", g.urlSource, err.Error())
		return err
	}

	unzippedFiles, err := files.Unzip(tempManifestsPath, directoryPath)
	if err != nil {
		return err
	}
	if keepRootDirectory || len(unzippedFiles) == 0 {
		return nil
	}

	rootFolder := unzippedFiles[0]

	if err := files.CopyDirChildren(rootFolder, directoryPath); err != nil {
		return err
	}


	err = os.RemoveAll(rootFolder)
	return err
}