package detector

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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

// TestCPUStatsUnderLimit test if CPU is under limit.
func TestCPUStatsUnderLimit(t *testing.T) {
	expected := &types.HealthCheck{
		Type:   "CPUUnderPressure",
		Result: "false",
	}

	cpuLimit := 50.0
	getCPUStats(cpuLimit)

	actual := m[expected.Type]
	delete(m, expected.Type)

	assert.Equal(t, actual.Type, expected.Type, "Type should be equal")
	assert.Equal(t, actual.Result, expected.Result, "Result should be equal")
	assert.Contains(t, actual.Message, "CPU usage", "Message should contain CPU usage")
}

// TestMemoryStats test if memory is under/over limit.
func TestMemoryStats(t *testing.T) {
	type test struct {
		expected    *types.HealthCheck
		memoryLimit float64
	}

	tests := []test{
		{&types.HealthCheck{
			Type:   "MemoryUnderPressure",
			Result: "false",
		}, 60},
		{&types.HealthCheck{
			Type:   "MemoryUnderPressure",
			Result: "true",
		}, 5},
	}

	for _, tc := range tests {
		getMemoryStats(tc.memoryLimit)
		actual := m[tc.expected.Type]
		delete(m, tc.expected.Type)

		assert.Equal(t, actual.Type, tc.expected.Type, "Type should be equal")
		assert.Equal(t, actual.Result, tc.expected.Result, "Result should be equal")
		assert.Contains(t, actual.Message, "memory available out of", "Message should contain \"memory available out of\" string")
	}
}

// TestDiskStats test if disk is under/over limit.
func TestDiskStats(t *testing.T) {
	type test struct {
		expected  *types.HealthCheck
		diskLimit float64
	}

	tests := []test{
		{&types.HealthCheck{
			Type:   "DiskUsageHigh",
			Result: "false",
		}, 60},
		{&types.HealthCheck{
			Type:   "DiskUsageHigh",
			Result: "true",
		}, 2},
	}

	for _, tc := range tests {
		getDiskStats(tc.diskLimit)
		actual := m[tc.expected.Type]
		delete(m, tc.expected.Type)

		assert.Equal(t, actual.Type, tc.expected.Type, "Type should be equal")
		assert.Equal(t, actual.Result, tc.expected.Result, "Result should be equal")
		assert.Contains(t, actual.Message, "disk usage is", "Message should contain \"disk usage is\" string")
	}
}

// TestNodeHealthEndpoint test the /v1/nodehealth/ HTTP endpoint.
func TestNodeHealthEndpoint(t *testing.T) {
	// Set the contents of global map (m) which will be returned when /v1/nodehealth/
	// endpoint is hit.
	m["docker"] = &types.HealthCheck{
		Type:    "docker",
		Result:  "Healthy",
		Message: "docker daemon is healthy",
	}
	m["portworx"] = &types.HealthCheck{
		Type:    "portworx",
		Result:  "Unhealthy",
		Message: "portworx is unhealthy",
	}

	req, err := http.NewRequest("POST", "/v1/nodehealth/", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(nodeHealthHandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("nodeHealthHandler returned incorrect status code: got %v, expected %v", status, http.StatusOK)
	}

	actual := []types.HealthCheck{}
	if err := json.Unmarshal([]byte(rr.Body.String()), &actual); err != nil {
		t.Fatal(err)
	}

	for _, hc := range actual {
		if hc.Type == "docker" {
			assert.Equal(t, hc.Type, m["docker"].Type, "Type should be equal")
			assert.Equal(t, hc.Result, m["docker"].Result, "Result should be equal")
			assert.Equal(t, hc.Message, m["docker"].Message, "Message should be equal")
		} else if hc.Type == "portworx" {
			assert.Equal(t, hc.Type, m["portworx"].Type, "Type should be equal")
			assert.Equal(t, hc.Result, m["portworx"].Result, "Result should be equal")
			assert.Equal(t, hc.Message, m["portworx"].Message, "Message should be equal")
		}
	}
	delete(m, "docker")
	delete(m, "portworx")
}

// TestHealthEndpoint test the /v1/health/ HTTP endpoint.
func TestHealthEndpoint(t *testing.T) {
	req, err := http.NewRequest("POST", "/v1/health/", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthCheckHandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("healthCheckHandler returned incorrect status code: got %v, expected %v", status, http.StatusOK)
	}
}
