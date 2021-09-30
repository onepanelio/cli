package cmd

import (
	"fmt"
	"path/filepath"

	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// skipConfirmDelete if true, will skip the confirmation prompt of the delete command
	skipConfirmDelete bool
)

var deleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Deletes onepanel cluster resources",
	Long:    "Delete all onepanel kubernetes cluster resources. Does not delete database unless it is in-cluster.",
	Example: "delete",
	Run: func(cmd *cobra.Command, args []string) {
		if skipConfirmDelete == false {
			options := clientcmd.NewDefaultPathOptions()
			config, err := options.GetStartingConfig()
			if err != nil {
				fmt.Printf("Unable to get kubernetes config: %v", err.Error())
				return
			}

			fmt.Println("The current kubernetes context is:", config.CurrentContext)
			fmt.Printf("Are you sure you want to delete onepanel from '%s'? ('y' or 'yes' to confirm. Anything else to cancel): ", config.CurrentContext)
			userInput := ""
			if _, err := fmt.Scanln(&userInput); err != nil {
				fmt.Printf("Unable to get response\n")
				return
			}

			if userInput != "y" && userInput != "yes" {
				return
			}
		}

		config, err := opConfig.FromFile("config.yaml")
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			return
		}

		paramsYamlFile, err := util.LoadDynamicYamlFromFile(config.Spec.Params)
		if err != nil {
			fmt.Println("Error parsing configuration file.")
			return
		}

		defaultNamespaceNode := paramsYamlFile.GetValue("application.defaultNamespace")
		if defaultNamespaceNode == nil {
			fmt.Printf("application.defaultNamespace is missing from your '%s' file\n", config.Spec.Params)
			return
		}

		if defaultNamespaceNode.Value == "default" {
			fmt.Println("Unable to delete onepanel in the 'default' namespace")
			return
		}

		if defaultNamespaceNode.Value == "<namespace>" {
			fmt.Println("Unable to delete onepanel. No namespace set.")
			return
		}

		filesToDelete := []string{
			filepath.Join(".onepanel", "kubernetes.yaml"),
			filepath.Join(".onepanel", "application.kubernetes.yaml"),
		}

		for _, filePath := range filesToDelete {
			exists, err := files.Exists(filePath)
			if err != nil {
				fmt.Printf("Error checking if onepanel files exist: %v\n", err.Error())
				return
			}

			if !exists {
				fmt.Printf("'%v' file does not exist. Are you in the directory where you ran 'opctl init'?\n", filePath)
				return
			}
		}

		fmt.Printf("Deleting onepanel from your cluster...\n")
		for _, filePath := range filesToDelete {
			if err := util.KubectlDelete(filePath); err != nil {
				fmt.Printf("Unable to delete: %v\n", err.Error())
				return
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&skipConfirmDelete, "yes", "y", false, "Add this in to skip the confirmation prompt")
}
