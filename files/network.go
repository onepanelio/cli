package files

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// DownloadFile will download a url to a local file.
// The network request attaches the "onepanelio" user-agent to the request headers
// This is important for certain sites like Github, otherwise you get a 403.
func DownloadFile(filepath string, url string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "onepanelio")

	// Get the data
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		return fmt.Errorf("[error] getting manifests. Response code %v", resp.StatusCode)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
