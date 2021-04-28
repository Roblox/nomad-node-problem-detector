package detector

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	types "github.com/nomad-node-problem-detector/types"
	"github.com/stretchr/testify/assert"
)

// TestReadValidConfig validates if nnpd health checks config.json can be read without error.
func TestReadValidConfig(t *testing.T) {
	inputConfig := []types.Config{
		{
			Type:        "docker",
			HealthCheck: "docker_health_check.sh",
		},
		{
			Type:        "portworx",
			HealthCheck: "portworx_health_check.sh",
		},
	}

	byteOutput, err := json.MarshalIndent(inputConfig, "", "\t")
	if err != nil {
		t.Errorf("Error in marshaling input config: %v\n", err)
	}

	s := string(byteOutput) + "\n"
	inputBytes := []byte(s)
	configPath := "/tmp/nnpd-test-config.json"

	if err = ioutil.WriteFile(configPath, inputBytes, 0644); err != nil {
		t.Errorf("Error in writing temp input config file: %v\n", err)
	}
	defer os.Remove(configPath)

	outputConfig := []types.Config{}
	err = readConfig(configPath, &outputConfig)

	assert.Nil(t, err)
	for index, val := range outputConfig {
		assert.Equal(t, val.Type, inputConfig[index].Type, "Type should be equal")
		assert.Equal(t, val.HealthCheck, inputConfig[index].HealthCheck, "Health check should be equal")
	}
}
