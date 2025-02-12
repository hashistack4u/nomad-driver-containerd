package containerd

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func getNamespaceName() string {
	// Calls to containerd API are namespaced.
	// "nomad" is the namespace that will be used for all nomad-driver-containerd
	// related containerd API calls.
	namespace := "nomad"
	// Unless we are operating in cgroups.v2 mode, in which case we use the
	// name "nomad.slice", which ends up being the cgroup parent.
	if cgroups.IsCgroup2UnifiedMode() {
		namespace = "nomad.slice"
	}
	return namespace
}

func getContainerName(name, allocID string) string {
	// Use Nomad's docker naming convention for the container name
	// https://www.nomadproject.io/docs/drivers/docker#container-name
	containerName := name + "-" + allocID
	if cgroups.IsCgroup2UnifiedMode() {
		// In cgroup.v2 mode, the name is slightly different.
		containerName = fmt.Sprintf("%s.%s.scope", allocID, name)
	}
	return containerName
}
