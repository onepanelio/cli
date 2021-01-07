module github.com/onepanelio/cli

go 1.13

require (
	github.com/ghodss/yaml v1.0.0
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.5.0
	golang.org/x/crypto v0.0.0-20200128174031-69ecbb4d6d5d
	github.com/stretchr/testify v1.4.0
	gopkg.in/yaml.v2 v2.2.8
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/apimachinery v0.17.3
	k8s.io/cli-runtime v0.17.3
	k8s.io/client-go v0.17.3
	k8s.io/kubectl v0.17.3
	k8s.io/utils v0.0.0-20200229041039-0a110f9eb7ab // indirect
	sigs.k8s.io/kustomize/api v0.3.2
)
