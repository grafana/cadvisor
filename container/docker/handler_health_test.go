// Copyright 2026 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker

import (
	"testing"

	dockercontainer "github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/opencontainers/cgroups"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"

	"github.com/google/cadvisor/container"
	containerlibcontainer "github.com/google/cadvisor/container/libcontainer"
	info "github.com/google/cadvisor/info/v1"
)

// fakeDockerClient reports a mutable health status for any inspected
// container. Only ContainerInspect is implemented; calling anything else
// panics through the embedded nil interface.
type fakeDockerClient struct {
	docker.APIClient
	health dockercontainer.HealthStatus
}

func (c *fakeDockerClient) ContainerInspect(ctx context.Context, id string) (dockercontainer.InspectResponse, error) {
	return dockercontainer.InspectResponse{
		ContainerJSONBase: &dockercontainer.ContainerJSONBase{
			State: &dockercontainer.State{
				Health: &dockercontainer.Health{Status: c.health},
			},
		},
	}, nil
}

// fakeCgroupManager returns empty stats so GetStats can run without a real
// cgroup behind it.
type fakeCgroupManager struct {
	cgroups.Manager
}

func (fakeCgroupManager) GetStats() (*cgroups.Stats, error) { return cgroups.NewStats(), nil }
func (fakeCgroupManager) Path(string) string                { return "" }

type fakeMachineInfoFactory struct{}

func (fakeMachineInfoFactory) GetMachineInfo() (*info.MachineInfo, error) {
	return &info.MachineInfo{}, nil
}

func (fakeMachineInfoFactory) GetVersionInfo() (*info.VersionInfo, error) {
	return &info.VersionInfo{}, nil
}

// GetStats must report the health status docker reports at scrape time, not
// the status the container had when the handler was created.
// Regression test for https://github.com/grafana/alloy/issues/6928.
func TestGetStatsReportsCurrentHealthStatus(t *testing.T) {
	client := &fakeDockerClient{health: dockercontainer.Starting}
	handler := &dockerContainerHandler{
		client:              client,
		reference:           info.ContainerReference{Id: "abcd"},
		machineInfoFactory:  fakeMachineInfoFactory{},
		libcontainerHandler: containerlibcontainer.NewHandler(fakeCgroupManager{}, "/", 0, container.MetricSet{}),
	}

	stats, err := handler.GetStats()
	assert.NoError(t, err)
	assert.Equal(t, "starting", stats.Health.Status)

	// The health check passes after the handler was created.
	client.health = dockercontainer.Healthy

	stats, err = handler.GetStats()
	assert.NoError(t, err)
	assert.Equal(t, "healthy", stats.Health.Status)

	// ... and the container can also become unhealthy later on.
	client.health = dockercontainer.Unhealthy

	stats, err = handler.GetStats()
	assert.NoError(t, err)
	assert.Equal(t, "unhealthy", stats.Health.Status)
}
