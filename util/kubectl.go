package util

import (
	"bytes"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"os"
)

func KubectlGet(resource string, resourceName string, namespace string) (stdout string, stderr string, err error) {
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
	args := []string{resource, resourceName}
	getOptions := get.NewGetOptions("kubectl", ioStreams)
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
