package util

import (
	"errors"
	//"strings"
)

func DeploymentStatus() (ready bool, err error) {
	//todo - necessary namespaces vs optional
	namespacesToCheck := []string{"application-system", "cert-manager", "istio-system", "kube-logging", "onepanel"}
	var stdout, stderr string
	for _, namespace := range namespacesToCheck {
		flags := make(map[string]interface{})
		var extraArgs []string
		stdout, stderr, err = KubectlGet("pod", "", namespace, extraArgs, flags)
		if err != nil {
			return false, err
		}
		if stderr != "" {
			return false, errors.New(stderr)
		}
		//lines := strings.Split(stdout,"\n")
		print(stdout)
	}

	return false, nil
}
