// Copyright 2015 Google Inc. All Rights Reserved.
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

//go:build linux

// Handler for /validate content.
// Validates cadvisor dependencies - kernel, os, docker setup.

package docker

import (
	"net/http"

	"github.com/docker/go-connections/tlsconfig"
	"github.com/google/cadvisor/lib/container/containerd"
	dclient "github.com/moby/moby/client"
)

// Client creates a Docker API client based on the given Docker options.
func (opts *Options) Client() (*dclient.Client, error) {
	opts.dockerClientOnce.Do(func() {
		var client *http.Client
		if opts.DockerTLS {
			client = &http.Client{}
			options := tlsconfig.Options{
				CAFile:             opts.DockerCA,
				CertFile:           opts.DockerCert,
				KeyFile:            opts.DockerKey,
				InsecureSkipVerify: false,
			}
			tlsc, err := tlsconfig.Client(options)
			if err != nil {
				opts.dockerClientErr = err
				return
			}
			client.Transport = &http.Transport{
				TLSClientConfig: tlsc,
			}
		}
		opts.dockerClient, opts.dockerClientErr = dclient.New(
			dclient.WithHost(opts.DockerEndpoint),
			dclient.WithHTTPClient(client),
		)
	})
	return opts.dockerClient, opts.dockerClientErr
}

// ContainerDClient returns a containerd client for the containerd instance
// backing this Docker daemon (Docker's own "moby" namespace).
func (opts *Options) ContainerDClient() (containerd.ContainerdClient, error) {
	cOpts := &containerd.Options{
		ContainerdEndpoint:  opts.ContainerDEndpoint,
		ContainerdNamespace: "moby",
	}
	return cOpts.Client(cOpts.ContainerdEndpoint, cOpts.ContainerdNamespace)
}
