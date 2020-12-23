package util

import (
	"bytes"
	"errors"
	"fmt"
	opConfig "github.com/onepanelio/cli/config"
	"github.com/spf13/cobra"
	k8error "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/kubectl/pkg/cmd/apply"
	k8delete "k8s.io/kubectl/pkg/cmd/delete"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"os"
	"runtime"
	"strconv"
	"strings"
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

func KubectlApply(filePath string) (stdout string, stderr string, err error) {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    out,
		ErrOut: errOut,
	}
	cmd := apply.NewCmdApply("kubectl", f, ioStreams)
	var args []string
	applyOptions := apply.NewApplyOptions(ioStreams)
	applyOptions.DeleteFlags.FileNameFlags.Filenames = &[]string{filePath}
	err = cmd.Flags().Set("filename", filePath)
	if err != nil {
		return "", "", err
	}
	err = cmd.Flags().Set("validate", "false")
	if err != nil {
		return "", "", err
	}
	if err = applyOptions.Complete(f, cmd); err != nil {
		return "", "", err
	}
	if err = validateArgs(cmd, args); err != nil {
		return "", "", err
	}
	if err = validatePruneAll(applyOptions.Prune, applyOptions.All, applyOptions.Selector); err != nil {
		return "", "", err
	}
	if err = applyOptions.Run(); err != nil {
		return "", "", err
	}

	stdout = out.String()
	stderr = errOut.String()

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
	cmd := k8delete.NewCmdDelete(f, ioStreams)

	deleteOptions := k8delete.DeleteOptions{IOStreams: ioStreams}
	deleteOptions.Filenames = []string{filePath}
	err = cmd.Flags().Set("filename", filePath)
	if err != nil {
		return err
	}

	if err := deleteOptions.Complete(f, []string{}, cmd); err != nil {
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

func GetClusterIp(url string) {
	kubectlGetFlags := make(map[string]interface{})
	kubectlGetFlags["output"] = "jsonpath='{.status.loadBalancer.ingress[0].ip}'"
	extraArgs := []string{}
	stdout, stderr, err := KubectlGet("service", "istio-ingressgateway", "istio-system", extraArgs, kubectlGetFlags)
	if err != nil {
		fmt.Printf("[error] Unable to get IP from istio-ingressgateway service: %v", err.Error())
		return
	}
	if stderr != "" {
		fmt.Printf("[error] Unable to get IP from istio-ingressgateway service: %v", stderr)
		return
	}

	if stdout == "" || stdout == "''" {
		kubectlGetFlags["output"] = "jsonpath='{.status.loadBalancer.ingress[0].hostname}'"
		extraArgs := []string{}
		stdout, stderr, err = KubectlGet("service", "istio-ingressgateway", "istio-system", extraArgs, kubectlGetFlags)
		if err != nil {
			fmt.Printf("[error] Unable to get Hostname from istio-ingressgateway service: %v", err.Error())
			return
		}
		if stderr != "" {
			fmt.Printf("[error] Unable to get Hostname from istio-ingressgateway service: %v", stderr)
			return
		}
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
			fqdn := yamlFile.GetValue("application.fqdn").Value

			hostsPath := "/etc/hosts"
			if runtime.GOOS == "windows" {
				hostsPath = "C:\\Windows\\System32\\Drivers\\etc\\hosts"
			}

			fmt.Printf("\nIn your %v file, add %v and point it to %v\n", hostsPath, stdout, fqdn)
		} else {
			dnsRecordMessage = "an A"
			if !IsIpv4(stdout) {
				dnsRecordMessage = "a CNAME"
			}
			fmt.Printf("\nIn your DNS, add %v record for %v and point it to %v\n", dnsRecordMessage, GetWildCardDNS(url), stdout)
		}
	}
	//If yaml key is missing due to older params.yaml file, use this default.
	if dnsRecordMessage == "" {
		dnsRecordMessage = "an A"
		if !IsIpv4(stdout) {
			dnsRecordMessage = "a CNAME"
		}
		fmt.Printf("\nIn your DNS, add %v record for %v and point it to %v\n", dnsRecordMessage, GetWildCardDNS(url), stdout)
	}
	fmt.Printf("Once complete, your application will be running at %v\n\n", url)
}
