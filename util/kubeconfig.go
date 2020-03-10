package util

import (
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"k8s.io/client-go/plugin/pkg/client/auth/exec"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

type Config = restclient.Config

func NewConfig() (config *Config) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		panic(err)
	}

	return
}

func GetBearerToken(in *restclient.Config, explicitKubeConfigPath string) (string, error) {

	if len(in.BearerToken) > 0 {
		return in.BearerToken, nil
	}

	if in == nil {
		return "", errors.Errorf("RestClient can't be nil")
	}
	if in.ExecProvider != nil {
		tc, err := in.TransportConfig()
		if err != nil {
			return "", err
		}

		auth, err := exec.GetAuthenticator(in.ExecProvider)
		if err != nil {
			return "", err
		}

		//This function will return error because of TLS Cert missing,
		// This code is not making actual request. We can ignore it.
		_ = auth.UpdateTransportConfig(tc)

		rt, err := transport.New(tc)
		if err != nil {
			return "", err
		}
		req := http.Request{Header: map[string][]string{}}

		_, _ = rt.RoundTrip(&req)

		token := req.Header.Get("Authorization")
		return strings.TrimPrefix(token, "Bearer "), nil
	}
	if in.AuthProvider != nil {
		if in.AuthProvider.Name == "gcp" {
			token := in.AuthProvider.Config["access-token"]
			if token == "" {
				return "", errors.New("GCP Access Token is not set. Try running a kubectl command, such as kubectl get nodes to set creds.")
			}
			token, err := RefreshTokenIfExpired(in, explicitKubeConfigPath, token)
			if err != nil {
				return "", err
			}
			return strings.TrimPrefix(token, "Bearer "), nil
		}
	}

	kubeClient, err := kubernetes.NewForConfig(in)
	if err != nil {
		return "", errors.Errorf("Could not get kubeClient")
	}
	secrets, err := kubeClient.CoreV1().Secrets("kube-system").List(v1.ListOptions{})
	if err != nil {
		return "", errors.Errorf("Could not get kube-system secrets.")
	}
	re := regexp.MustCompile(`^default-token-`)
	for _, secret := range secrets.Items {
		if re.Find([]byte(secret.ObjectMeta.Name)) != nil {
			return string(secret.Data["token"]), nil
		}
	}
	return "", errors.Errorf("could not find a token")
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
