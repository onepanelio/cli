package github

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type Release struct {
	Url string `json:"url"`
	Name string `json:"name"`
	TagName string `json:"tag_name"`
	CreatedAt string `json:"created_at"`
	TarBallUrl string `json:"tarball_url"`
	ZipBallUrl string `json:"zipball_url"`
}

type Github struct {
	repoUrl string
}

func New(url string) (*Github, error) {
	return &Github{repoUrl: url}, nil
}

func (g *Github) GetRelease(url string) (release *Release, err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	release = &Release{}
	err = json.Unmarshal(data, release)

	return
}

func (g *Github) GetLatestRelease() (release *Release, err error) {
	return g.GetRelease(g.repoUrl + "/releases/latest")
}

func (g *Github) GetReleaseByTag(tag string) (release *Release, err error) {
	return g.GetRelease(g.repoUrl + "/releases/tags/" + tag)
}