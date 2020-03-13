package util

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"os"
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
		stringVal, ok := flagVal.(string)
		if !ok {
			return "", "", errors.New(flagName + ", unexpected flag value type")
		} //Check if getoptions is losing it's setting
		// todo remove -- uncomment this chunk to set the flags on getOptions
		//test := "jsonpath='{.status.loadBalancer.ingress[0].ip}'"
		//getOptions.PrintFlags.OutputFormat = &test
		if err = cmd.Flags().Set(flagName, stringVal); err != nil {
			return "", "", err
		}
	}
	args := []string{resource, resourceName}
	args = append(args, extraArgs...)
	// todo this works with just setting the flags on the command
	//cmd.Run(cmd, args)

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
