package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	opConfig "github.com/onepanelio/cli/config"
	"github.com/spf13/cobra"
	"io/ioutil"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8error "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/kubectl/pkg/cmd/apply"
	k8delete "k8s.io/kubectl/pkg/cmd/delete"
	"k8s.io/kubectl/pkg/cmd/get"
	"k8s.io/kubectl/pkg/cmd/patch"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func KubectlGet(resource string, resourceName string, namespace string, extraArgs []string, flags map[string]interface{}) (stdout string, stderr string, err error) {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.Namespace = &namespace
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    out,
		ErrOut: errOut,
	}
	cmd := get.NewCmdGet("kubectl", f, ioStreams)
	getOptions := get.NewGetOptions("kubectl", ioStreams)

	for flagName, flagVal := range flags {
		boolVal, okBool := flagVal.(bool)
		if okBool {
			if err = cmd.Flags().Set(flagName, strconv.FormatBool(boolVal)); err != nil {
				return "", "", err
			}
			continue
		}
		stringVal, okStr := flagVal.(string)
		if okStr {
			if flagName == "output" {
				getOptions.PrintFlags.OutputFormat = &stringVal
			}

			if err = cmd.Flags().Set(flagName, stringVal); err != nil {
				return "", "", err
			}
			continue
		}
		return "", "", errors.New(flagName + ", unexpected flag value type")
	}

	var args []string
	if resource != "" {
		args = append(args, resource)
	}
	if resourceName != "" {
		args = append(args, resourceName)
	}
	args = append(args, extraArgs...)

	if err = getOptions.Complete(f, cmd, args); err != nil {
		return "", "", err
	}
	if err = getOptions.Validate(cmd); err != nil {
		return "", "", err
	}
	if err = getOptions.Run(f, cmd, args); err != nil {
		return "", "", err
	}

	stdout = out.String()
	stderr = errOut.String()
	return
}

// KubectlApply applies the yaml at the given filePath
func KubectlApply(filePath string) (err error) {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	cmd := ApplyCmdWithError(f, ioStreams)
	if err := cmd.Flags().Set("filename", filePath); err != nil {
		return err
	}

	return cmd.RunE(cmd, []string{})
}

// KubectlPatch patches a resource.
// resource example: serviceaccount/default
func KubectlPatch(namespace string, resource string, filePath string) (err error) {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.Namespace = &namespace
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	cmd := patch.NewCmdPatch(f, ioStreams)

	if err := cmd.Flags().Set("patch", string(content)); err != nil {
		return err
	}

	cmd.Run(cmd, []string{resource})

	return
}

// KubectlDelete run's kubectl delete using the input filePath
func KubectlDelete(filePath string) (err error) {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	deleteFlags := k8delete.NewDeleteCommandFlags("containing the resource to delete.")
	cmd := &cobra.Command{}
	deleteFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	deleteOptions, err := deleteFlags.ToOptions(nil, ioStreams)
	if err != nil {
		return err
	}
	deleteOptions.Filenames = []string{filePath}
	if err := deleteOptions.Complete(f, []string{}, cmd); err != nil {
		return err
	}
	if err := deleteOptions.Validate(); err != nil {
		return err
	}

	if err := deleteOptions.RunDelete(f); err != nil {
		errorAggregate, ok := err.(k8error.Aggregate)
		if ok {
			finalErrors := make([]error, 0)
			// Skip any errors that mean "not found"
			for _, errItem := range errorAggregate.Errors() {
				if strings.Contains(errItem.Error(), "not found") {
					continue
				}

				if strings.Contains(errItem.Error(), "no matches for kind") {
					continue
				}

				if strings.Contains(errItem.Error(), "the server could not find the requested resource") {
					continue
				}

				finalErrors = append(finalErrors, errItem)
			}

			if len(finalErrors) == 0 {
				return nil
			}

			return k8error.NewAggregate(finalErrors)
		}

		return err
	}

	return
}

func validateArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmdutil.UsageErrorf(cmd, "Unexpected args: %v", args)
	}
	return nil
}

func validatePruneAll(prune, all bool, selector string) error {
	if all && len(selector) > 0 {
		return fmt.Errorf("cannot set --all and --selector at the same time")
	}
	if prune && !all && selector == "" {
		return fmt.Errorf("all resources selected for prune without explicitly passing --all. To prune all resources, pass the --all flag. If you did not mean to prune all resources, specify a label selector")
	}
	return nil
}

// getDeployedIP attempts to get the ip address of the load balancer
// ("pending", nil) is returned if there is no ip yet
func getDeployedIP(c *kubernetes.Clientset) (string, error) {
	svc, err := c.CoreV1().Services("istio-system").Get(context.Background(), "istio-ingressgateway", v1.GetOptions{})
	if err != nil {
		return "", err
	}

	ingress := svc.Status.LoadBalancer.Ingress
	if ingress == nil {
		return "pending", nil
	}

	if len(ingress) == 1 {
		if ingress[0].IP == "" && ingress[0].Hostname != "" {
			return ingress[0].Hostname, nil
		}

		return ingress[0].IP, nil
	}

	return "", fmt.Errorf("unable to get load balancer ip")
}

// getDeployedIPRetry calls getDeployedIP retries times, with a delay in between each call while the ip is pending
func getDeployedIPRetry(c *kubernetes.Clientset, retries int, delay time.Duration) (string, error) {
	for tries := 0; tries < retries; tries++ {
		ip, err := getDeployedIP(c)
		if err != nil {
			return ip, err
		}

		if ip != "pending" {
			return ip, err
		}

		time.Sleep(delay)
	}

	return "", fmt.Errorf("unable to get deployed ip from LoadBalancer")
}

// PrintClusterNetworkInformation prints the ip address of the cluster and network DNS configuration required
func PrintClusterNetworkInformation(c *kubernetes.Clientset, url string) {
	clusterIP, err := getDeployedIPRetry(c, 20, 6*time.Second)
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	configFilePath := "config.yaml"

	config, err := opConfig.FromFile(configFilePath)
	if err != nil {
		fmt.Printf("Unable to read configuration file: %v", err.Error())
		return
	}

	yamlFile, err := LoadDynamicYamlFromFile(config.Spec.Params)
	if err != nil {
		fmt.Printf("Unable to load yaml file: %v", err.Error())
		return
	}

	var dnsRecordMessage string
	if yamlFile.HasKey("application.provider") {
		provider := yamlFile.GetValue("application.provider").Value
		if provider == "minikube" || provider == "microk8s" {
			domain := yamlFile.GetValue("application.domain").Value
			fqdn := yamlFile.GetValue("application.fqdn").Value

			hostsPath := "/etc/hosts"
			if runtime.GOOS == "windows" {
				hostsPath = "C:\\Windows\\System32\\Drivers\\etc\\hosts"
			}

			dnsRecordMessage = "local"
			fmt.Printf("\nIn your %v file, add\n", hostsPath)
			fmt.Printf("  %v %v\n", clusterIP, fqdn)

			defaultNamespace := yamlFile.GetValue("application.defaultNamespace").Value
			sysStorageURL := fmt.Sprintf("sys-storage-%v.%v", defaultNamespace, domain)
			fmt.Printf("  %v %v\n", clusterIP, sysStorageURL)

			if config.Spec.HasLikeComponent("kfserving") {
				modelServingURL := fmt.Sprintf("serving.%v", domain)
				fmt.Printf("  %v %v\n", clusterIP, modelServingURL)
			}

			fmt.Println()
		} else {
			dnsRecordMessage = "an A"
			if !IsIpv4(clusterIP) {
				dnsRecordMessage = "a CNAME"
			}
			fmt.Printf("\nIn your DNS, add %v record for %v and point it to %v\n", dnsRecordMessage, GetWildCardDNS(url), clusterIP)
		}
	}
	//If yaml key is missing due to older params.yaml file, use this default.
	if dnsRecordMessage == "" {
		dnsRecordMessage = "an A"
		if !IsIpv4(clusterIP) {
			dnsRecordMessage = "a CNAME"
		}
		fmt.Printf("\nIn your DNS, add %v record for %v and point it to %v\n", dnsRecordMessage, GetWildCardDNS(url), clusterIP)
	}
	fmt.Printf("Once complete, your application will be running at %v\n\n", url)
}

// ApplyCmdWithError runs the kubectl apply command and returns an error, if any.
func ApplyCmdWithError(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := apply.NewApplyOptions(ioStreams)

	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(validateArgs(cmd, args))
			cmdutil.CheckErr(validatePruneAll(o.Prune, o.All, o.Selector))
			return o.Run()
		},
	}

	// bind flag structs
	o.DeleteFlags.AddFlags(cmd)
	o.RecordFlags.AddFlags(cmd)
	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.Overwrite, "overwrite", o.Overwrite, "Automatically resolve conflicts between the modified and live configuration by using values from the modified configuration")
	cmd.Flags().BoolVar(&o.Prune, "prune", o.Prune, "Automatically delete resource objects, including the uninitialized ones, that do not appear in the configs and are created by either apply or create --save-config. Should be used with either -l or --all.")
	cmdutil.AddValidateFlags(cmd)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "Select all resources in the namespace of the specified resource types.")
	cmd.Flags().StringArrayVar(&o.PruneWhitelist, "prune-whitelist", o.PruneWhitelist, "Overwrite the default whitelist with <group/version/kind> for --prune")
	cmd.Flags().BoolVar(&o.OpenAPIPatch, "openapi-patch", o.OpenAPIPatch, "If true, use openapi to calculate diff when the openapi presents and the resource can be found in the openapi spec. Otherwise, fall back to use baked-in types.")
	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddServerSideApplyFlags(cmd)
	cmdutil.AddFieldManagerFlagVar(cmd, &o.FieldManager, apply.FieldManagerClientSideApply)

	return cmd
}
