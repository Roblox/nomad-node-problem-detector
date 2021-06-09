/*
Copyright 2021 Roblox Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0


Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package npd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type NPDTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "npd",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(NPDTest),
		},
	})
}

func (tc *NPDTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)

	t := f.T()
	nomadClient := tc.Nomad()

	// Deploy detector and aggregator jobs.
	tc.deployJob(t, nomadClient, "detector", "npd/jobs/detector.nomad")
	tc.deployJob(t, nomadClient, "aggregator", "npd/jobs/aggregator.nomad")
}

func (tc *NPDTest) TestMarkNodeIneligible(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()

	var detectorJobID string
	for _, id := range tc.jobIDs {
		if strings.Contains(id, "detector") {
			detectorJobID = id
			break
		}
	}

	allocs, _, err := jobs.Allocations(detectorJobID, true, nil)
	require.NoError(t, err)

	srcFile := fmt.Sprintf("/tmp/nomad/data/%s/alloc/var/lib/nnpd/docker/docker_health_check.sh", allocs[0].ID)
	destDir, err := ioutil.TempDir("", "nnpd-")
	require.NoError(t, err)
	defer os.RemoveAll(destDir)
	destFile := filepath.Join(destDir, "docker_health_check.sh")
	err = os.Rename(srcFile, destFile)
	require.NoError(t, err)

	content := []byte("echo \"docker daemon is unhealthy.\"\nexit 1\n")
	err = ioutil.WriteFile(srcFile, content, 0755)
	require.NoError(t, err)

	// Sleep for 6 seconds. This will allow aggregator to pick up the
	// updated (unhealthy) docker health checks.
	time.Sleep(6 * time.Second)

	// npd aggregator should mark the node as ineligible, since docker daemon is unhealthy.
	nh := nomadClient.Nodes()
	nodes, _, err := nh.List(nil)
	require.NoError(t, err)

	assert.Equal(t, nodes[0].SchedulingEligibility, "ineligible", "Scheduling Eligibility should be false.")

	// Flip docker health check back to healthy.
	err = os.Rename(destFile, srcFile)
	require.NoError(t, err)

	// Sleep for 6 seconds. This will allow aggregator to pick up the
	// updated (healthy) docker health checks.
	time.Sleep(6 * time.Second)

	nodes, _, err = nh.List(nil)
	require.NoError(t, err)

	assert.Equal(t, nodes[0].SchedulingEligibility, "eligible", "Scheduling Eligibility should be true.")
}

func (tc *NPDTest) AfterAll(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIDs {
		jobs.Deregister(id, true, nil)
	}
	tc.jobIDs = []string{}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}

func (tc *NPDTest) deployJob(t *testing.T, nomadClient *api.Client, jobName, jobPath string) {
	uuid := uuid.Generate()
	jobID := jobName + "-" + uuid[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, jobPath, jobID, "")

	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	var allocIDs []string
	for _, alloc := range allocs {
		allocIDs = append(allocIDs, alloc.ID)
	}

	// Wait for allocations to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	require.Len(t, allocs, 1)
	require.Equal(t, allocs[0].ClientStatus, "running")
}
