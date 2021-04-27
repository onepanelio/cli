package util

import (
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

func NewConfig() (config *Config, err error) {
	config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()

	return
}

func GetBearerToken(in *restclient.Config, explicitKubeConfigPath string, serviceAccountName string) (token string, username string, err error) {
	if in == nil {
		return "", serviceAccountName, errors.Errorf("RestClient can't be nil")
	}

	kubeClient, err := kubernetes.NewForConfig(in)
	if err != nil {
		return "", serviceAccountName, errors.Errorf("Could not get kubeClient")
	}
	ns := "onepanel"
	secrets, err := kubeClient.CoreV1().Secrets(ns).List(v1.ListOptions{})
	if err != nil {
		return "", serviceAccountName, errors.Errorf("Could not get %s secrets.", ns)
	}
	search := `^` + serviceAccountName + `-token-`
	re := regexp.MustCompile(search)
	for _, secret := range secrets.Items {
		if re.Find([]byte(secret.ObjectMeta.Name)) != nil {
			return string(secret.Data["token"]), serviceAccountName, nil
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
