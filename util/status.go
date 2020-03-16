package util

import (
	"errors"
	"fmt"
	"strings"
)

func DeploymentStatus() (ready bool, err error) {
	//True is a required namespace
	namespacesToCheck := make(map[string]bool)
	namespacesToCheck["application-system"] = true
	namespacesToCheck["cert-manager"] = true
	namespacesToCheck["istio-system"] = true
	namespacesToCheck["kube-logging"] = false
	namespacesToCheck["onepanel"] = true
	var stdout, stderr string
	for namespace, required := range namespacesToCheck {
		flags := make(map[string]interface{})
		var extraArgs []string
		stdout, stderr, err = KubectlGet("pod", "", namespace, extraArgs, flags)
		if err != nil {
			return false, err
		}
		if stderr != "" {
			if strings.Contains(stderr, "No resources found") {
				if required {
					return false, errors.New(stderr)
				}
				fmt.Println(stderr)
				continue
			}
			return false, errors.New(stderr)
		}
		//lines := strings.Split(stdout,"\n")
		print(stdout)
	}

	return false, nil
}
