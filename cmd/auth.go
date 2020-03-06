package cmd

import (
	"fmt"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
)


var authCmd = &cobra.Command{
	Use:                        "auth",
	Short:                      "Get authentication information.",
	Long:                       "Intented to be used to get authentication information.",
	Example:                    "auth token",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var tokenCmd = &cobra.Command{
	Use: "token",
	Short: "Get the token for a provider.",
	Long: "Get a token for a given provider. Google Cloud Platform is different from minikube, for example.",
	Example: "auth token --provider=gke",
	Run: func(cmd *cobra.Command, args []string) {
		if Provider == "" {
			fmt.Println("Provider cannot be empty.")
			return
		}
		if err := validateProvider(Provider); err != nil {
			fmt.Println(err.Error())
			return
		}

		config := util.NewConfig()
		token, err := util.GetBearerToken(config,"")
		if err != nil {
			fmt.Println("Error encountered: ", err.Error())
		}
		fmt.Println(token)
	},
}
func init()  {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(tokenCmd)

	tokenCmd.Flags().StringVarP(&Provider,"provider","p","","Specify the provider to get auth token from. Valid values are the same as provider values.")
}