package cmd

import (
	"fmt"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
	"log"
)

var (
	Token string
)

var authCmd = &cobra.Command{
	Use:                        "auth",
	Short:                      "Various provider auth commands.",
	Long:                       "Intented to be used to get authentication information.",
	Example:                    "auth token",
	Run: func(cmd *cobra.Command, args []string) {
		if Token == "" {
			log.Fatalln("Token cannot be empty.")
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

	authCmd.Flags().StringVarP(&Token,"token","","","Specify the provider to get auth token from. Valid values are the same as provider values.")
}