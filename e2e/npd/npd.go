package npd

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
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

	var allocID string
	for _, alloc := range allocs {
		allocID = alloc.ID
	}

	file := fmt.Sprintf("/tmp/nomad/data/%s/alloc/var/lib/nnpd/docker/docker_health_check.sh", allocID)
	content, err := ioutil.ReadFile(file)
	require.NoError(t, err)
	fmt.Println(string(content))

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
