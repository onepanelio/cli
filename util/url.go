package util

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

func GetWildCardDNS(url string) string {
	url = strings.ReplaceAll(url, "/", "")
	parts := strings.Split(url, ".")
	url = strings.Join(parts[1:], ".")

	return fmt.Sprintf("*.%v", url)
}

func GetDeployedWebURL(yamlFile *DynamicYaml) (string, error) {
	httpScheme := "http://"
	fqdn := yamlFile.GetValue("application.fqdn").Value
	fqdnExtra := ""

	if yamlFile.HasKey("application.local") {
		applicationUIPort := yamlFile.GetValue("application.local.uiHTTPPort").Value
		fqdnExtra = fmt.Sprintf(":%v", applicationUIPort)
	} else {
		applicationUIPath := yamlFile.GetValue("application.cloud.uiPath").Value

		fqdnExtra = fmt.Sprintf("%v", applicationUIPath)

		insecure, err := strconv.ParseBool(yamlFile.GetValue("application.cloud.insecure").Value)
		if err != nil {
			log.Fatal("insecure is not a bool")
		}

		if !insecure {
			httpScheme = "https://"
		}
	}

	return fmt.Sprintf("%v%v%v", httpScheme, fqdn, fqdnExtra), nil
}
