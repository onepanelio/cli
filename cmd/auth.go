package cmd

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
)

var (
	// ServiceAccountName is the "username" we show to the user. We look up this value in k8s
	ServiceAccountName string
)

var authCmd = &cobra.Command{
	Use:     "auth",
	Short:   "Get authentication information.",
	Long:    "Intended to be used to get authentication information.",
	Example: "auth token",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var tokenCmd = &cobra.Command{
	Use:     "token",
	Short:   "Get the token for a provider.",
	Long:    "Get a token for a given provider. Google Cloud Platform is different from minikube, for example.",
	Example: "auth token",
	Run: func(cmd *cobra.Command, args []string) {
		config := util.NewConfig()
		if ServiceAccountName == "" {
			ServiceAccountName = "admin"
		}
		token, username, err := util.GetBearerToken(config, "", ServiceAccountName)
		if err != nil {
			fmt.Printf("Error encountered for user %s: %s\n", username, err.Error())
		}

		if token != "" {
			currentTokenBytes := md5.Sum([]byte(token))
			currentTokenString := hex.EncodeToString(currentTokenBytes[:])

			fmt.Println(username)
			fmt.Println(currentTokenString)
		}
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(tokenCmd)
	tokenCmd.Flags().StringVarP(&ServiceAccountName, "username", "u", "", "Username you want the token for. Defaults to 'admin'")
}
