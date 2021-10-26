package util

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"os"
	"regexp"
	"time"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

type Config = restclient.Config

const onepanelEnabledLabelKey = "onepanel.io/enabled"

func ListOnepanelEnabledNamespaces(c *kubernetes.Clientset) (namespaces []string, err error) {
	namespaceList, err := c.CoreV1().Namespaces().List(context.Background(), v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", onepanelEnabledLabelKey, "true"),
	})

	if err != nil {
		return
	}

	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	return
}

// NewConfig creates a new, default, configuration for kubernetes
func NewConfig() (config *Config, err error) {
	config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()

	return
}

// NewKubernetesClient creates a kubernetes client with the config returned from NewConfig
func NewKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func GetBearerToken(in *restclient.Config, explicitKubeConfigPath string, serviceAccountName string) (token string, username string, err error) {
	if in == nil {
		return "", serviceAccountName, errors.Errorf("RestClient can't be nil")
	}

	kubeClient, err := kubernetes.NewForConfig(in)
	if err != nil {
		return "", serviceAccountName, errors.Errorf("Could not get kubeClient")
	}

	namespaces := []string{"onepanel"}
	moreNamespaces, err := ListOnepanelEnabledNamespaces(kubeClient)
	if err != nil {
		return "", "", err
	}
	namespaces = append(namespaces, moreNamespaces...)

	for _, namespace := range namespaces {
		secrets, err := kubeClient.CoreV1().Secrets(namespace).List(context.Background(), v1.ListOptions{})
		if err != nil {
			return "", serviceAccountName, errors.Errorf("Could not get %s secrets.", namespace)
		}

		search := `^` + serviceAccountName + `-token-`
		re := regexp.MustCompile(search)
		for _, secret := range secrets.Items {
			if re.Find([]byte(secret.ObjectMeta.Name)) != nil {
				return string(secret.Data["token"]), serviceAccountName, nil
			}
		}
	}

	return "", serviceAccountName, errors.Errorf("could not find a token")
}

func RefreshTokenIfExpired(restConfig *restclient.Config, explicitPath, curentToken string) (string, error) {
	if restConfig.AuthProvider != nil {
		timestr := restConfig.AuthProvider.Config["expiry"]
		if timestr != "" {
			t, err := time.Parse(time.RFC3339, timestr)
			if err != nil {
				return "", errors.Errorf("Invalid expiry date in Kubeconfig. %v", err)
			}
			if time.Now().After(t) {
				err = RefreshAuthToken(restConfig)
				if err != nil {
					return "", err
				}
				config := ReloadKubeConfig(explicitPath)
				restConfig, err = config.ClientConfig()
				if err != nil {
					return "", err
				}
				return restConfig.AuthProvider.Config["access-token"], nil
			}
		}
	}
	return curentToken, nil
}

func ReloadKubeConfig(explicitPath string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = explicitPath
	overrides := clientcmd.ConfigOverrides{}
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

func RefreshAuthToken(in *restclient.Config) error {
	tc, err := in.TransportConfig()
	if err != nil {
		return err
	}

	auth, err := restclient.GetAuthProvider(in.Host, in.AuthProvider, in.AuthConfigPersister)
	if err != nil {
		return err
	}

	rt, err := transport.New(tc)
	if err != nil {
		return err
	}
	rt = auth.WrapTransport(rt)
	req := http.Request{Header: map[string][]string{}}

	_, _ = rt.RoundTrip(&req)
	return nil
}
