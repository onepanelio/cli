package cmd

import (
	"github.com/onepanelio/cli/util"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	ParamsApplication = `application:
  nodePool:
    options:
      - name: 'CPU: 2, RAM: 8GB'
        value: Standard_D2s_v3
      - name: 'CPU: 4, RAM: 16GB'
        value: Standard_D4s_v3
      - name: 'GPU: 1xK80, CPU: 6, RAM: 56GB'
        value: Standard_NC6`
)

func Test_generateApplicationNodePoolOptions(t *testing.T) {
	// Have to account for extra spaces yaml.v3 seems to add
	nodePoolOptionsExpected := `|
    - name: 'CPU: 2, RAM: 8GB'
      value: Standard_D2s_v3
    - name: 'CPU: 4, RAM: 16GB'
      value: Standard_D4s_v3
    - name: 'GPU: 1xK80, CPU: 6, RAM: 56GB'
      value: Standard_NC6
    
`

	data, err := util.LoadDynamicYamlFromString(ParamsApplication)
	nodePoolData := data.GetValue("application.nodePool")
	nodePoolOptionsActual := generateApplicationNodePoolOptions(nodePoolData)

	assert.Nil(t, err)
	assert.Equal(t, nodePoolOptionsExpected, nodePoolOptionsActual)
}