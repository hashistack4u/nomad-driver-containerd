/*
Copyright 2020 Roblox Corporation

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

package containerd

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	refdocker "github.com/containerd/containerd/reference/docker"
	remotesdocker "github.com/containerd/containerd/remotes/docker"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type ContainerConfig struct {
	Image                 containerd.Image
	ContainerName         string
	ContainerSnapshotName string
	NetworkNamespacePath  string
	SecretsDirSrc         string
	TaskDirSrc            string
	AllocDirSrc           string
	SecretsDirDest        string
	TaskDirDest           string
	AllocDirDest          string
	Env                   []string
	MemoryLimit           int64
	MemoryHardLimit       int64
	CPUShares             int64
	User                  string
}

func (d *Driver) isContainerdRunning() (bool, error) {
	ctxWithTimeout, cancel := context.WithTimeout(d.ctxContainerd, 30*time.Second)
	defer cancel()

	return d.client.IsServing(ctxWithTimeout)
}

func (d *Driver) getContainerdVersion() (containerd.Version, error) {
	ctxWithTimeout, cancel := context.WithTimeout(d.ctxContainerd, 30*time.Second)
	defer cancel()

	return d.client.Version(ctxWithTimeout)
}

type CredentialsOpt func(string) (string, string, error)

func (d *Driver) parshAuth(auth *RegistryAuth) CredentialsOpt {
	return func(string) (string, string, error) {
		var username, password string
		if d.config.Auth.Username != "" && d.config.Auth.Password != "" {
			username = d.config.Auth.Username
			password = d.config.Auth.Password
		}

		// Job auth will take precedence over plugin auth options.
		if auth.Username != "" && auth.Password != "" {
			username = auth.Username
			password = auth.Password
		}
		return username, password, nil
	}
}

func withResolver(creds CredentialsOpt) containerd.RemoteOpt {
	resolver := remotesdocker.NewResolver(remotesdocker.ResolverOptions{
		Hosts: remotesdocker.ConfigureDefaultRegistries(remotesdocker.WithAuthorizer(
			remotesdocker.NewDockerAuthorizer(remotesdocker.WithAuthCreds(creds)))),
	})
	return containerd.WithResolver(resolver)
}

func withFileLimit(maxOpenFiles uint64) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *oci.Spec) error {
		newRlimits := []specs.POSIXRlimit{{
			Type: "RLIMIT_NOFILE",
			Hard: maxOpenFiles,
			Soft: maxOpenFiles,
		}}

		// Copy existing rlimits excluding previous RLIMIT_NOFILE
		for _, rlimit := range spec.Process.Rlimits {
			if rlimit.Type != "RLIMIT_NOFILE" {
				newRlimits = append(newRlimits, rlimit)
			}
		}

		spec.Process.Rlimits = newRlimits

		return nil
	}
}

func (d *Driver) pullImage(imageName, imagePullTimeout string, auth *RegistryAuth) (containerd.Image, error) {
	pullTimeout, err := time.ParseDuration(imagePullTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image_pull_timeout: %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(d.ctxContainerd, pullTimeout)
	defer cancel()

	named, err := refdocker.ParseDockerRef(imageName)
	if err != nil {
		return nil, err
	}

	pullOpts := []containerd.RemoteOpt{
		containerd.WithPullUnpack,
		withResolver(d.parshAuth(auth)),
	}

	return d.client.Pull(ctxWithTimeout, named.String(), pullOpts...)
}

func (d *Driver) loadContainer(id string) (containerd.Container, error) {
	ctxWithTimeout, cancel := context.WithTimeout(d.ctxContainerd, 30*time.Second)
	defer cancel()

	return d.client.LoadContainer(ctxWithTimeout, id)
}

func (d *Driver) createTask(container containerd.Container, stdoutPath, stderrPath string) (containerd.Task, error) {
	stdout, stderr, err := getStdoutStderrFifos(stdoutPath, stderrPath)
	if err != nil {
		return nil, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(d.ctxContainerd, 30*time.Second)
	defer cancel()

	return container.NewTask(ctxWithTimeout, cio.NewCreator(cio.WithStreams(nil, stdout, stderr)))
}

func (d *Driver) getTask(container containerd.Container, stdoutPath, stderrPath string) (containerd.Task, error) {
	stdout, stderr, err := getStdoutStderrFifos(stdoutPath, stderrPath)
	if err != nil {
		return nil, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(d.ctxContainerd, 30*time.Second)
	defer cancel()

	return container.Task(ctxWithTimeout, cio.NewAttach(cio.WithStreams(nil, stdout, stderr)))
}
