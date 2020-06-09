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

	insecure, err := strconv.ParseBool(yamlFile.GetValue("application.insecure").Value)
	if err != nil {
		log.Fatal("insecure is not a bool")
	}

	if !insecure {
		httpScheme = "https://"
	}

	return fmt.Sprintf("%v%v%v", httpScheme, fqdn, fqdnExtra), nil
}
