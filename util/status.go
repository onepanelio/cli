package util

import (
	"context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
)

// NamespacesToCheck returns an array of namespaces to check depending on what's present in the yamlFile
func NamespacesToCheck(yamlFile *DynamicYaml) []string {
	namespaces := make([]string, 0)
	namespaces = append(namespaces, "application-system", "onepanel", "istio-system")

	if yamlFile.HasKey("certManager") {
		namespaces = append(namespaces, "cert-manager")
	}
	if yamlFile.HasKey("logging") {
		namespaces = append(namespaces, "kube-logging")
	}

	return namespaces
}

// IsApplicationControllerManagerRunning checks if the application-controller-manager pod is running
func IsApplicationControllerManagerRunning(c *kubernetes.Clientset) (bool, error) {
	pod, err := c.CoreV1().Pods("application-system").Get(context.Background(), "application-controller-manager-0", v1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}

	return pod.Status.Phase == "Running", nil
}

// NamespacesExist checks if the cluster has the input namespaces
func NamespacesExist(c *kubernetes.Clientset, namespaces ...string) (bool, error) {
	for _, namespace := range namespaces {
		_, err := c.CoreV1().Namespaces().Get(context.Background(), namespace, v1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return false, nil
			}

			return false, err
		}
	}

	return true, nil
}
