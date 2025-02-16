package containerd

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/netns"
)

func (d *Driver) createContainer(containerConfig *ContainerConfig, config *TaskConfig) (containerd.Container, error) {
	if config.Command != "" && config.Entrypoint != nil {
		return nil, fmt.Errorf("both command and entrypoint are set. Only one of them needs to be set")
	}

	// Entrypoint or Command set by the user, to override entrypoint or cmd defined in the image.
	var args []string
	if config.Command != "" {
		args = append(args, config.Command)
	} else if config.Entrypoint != nil && config.Entrypoint[0] != "" {
		args = append(args, config.Entrypoint...)
	}

	// Arguments to the command set by the user.
	if len(config.Args) > 0 {
		args = append(args, config.Args...)
	}

	var opts []oci.SpecOpts

	if config.Entrypoint != nil {
		opts = append(opts, oci.WithImageConfig(containerConfig.Image))
		// WithProcessArgs replaces the args on the generated spec.
		opts = append(opts, oci.WithProcessArgs(args...))
	} else {
		// WithImageConfigArgs configures the spec to from the configuration of an Image
		// with additional args that replaces the CMD of the image.
		opts = append(opts, oci.WithImageConfigArgs(containerConfig.Image, args))
	}

	// This translates to docker create/run --cpuset-cpus option.
	// --cpuset-cpus limit the specific CPUs or cores a container can use.
	if config.CPUSetCPUs != "" {
		opts = append(opts, oci.WithCPUs(config.CPUSetCPUs))
	}

	// --cpuset-mems is the list of memory nodes on which processes
	// in this cpuset are allowed to allocate memory.
	if config.CPUSetMEMs != "" {
		opts = append(opts, oci.WithCPUsMems(config.CPUSetMEMs))
	}

	// Set current working directory (cwd).
	if config.Cwd != "" {
		opts = append(opts, oci.WithProcessCwd(config.Cwd))
	}

	// Set environment variables.
	opts = append(opts, oci.WithEnv(containerConfig.Env))

	// Set CPU Shares.
	// FixMe: This causes nil value crash in Windows
	// opts = append(opts, oci.WithCPUShares(uint64(containerConfig.CPUShares)))

	// Set Hostname
	hostname := containerConfig.ContainerName
	if config.Hostname != "" {
		hostname = config.Hostname
	}
	opts = append(opts, oci.WithHostname(hostname))

	// Add linux devices into the container.
	for _, device := range config.Devices {
		opts = append(opts, oci.WithLinuxDevice(device, "rwm"))
	}

	// Set mounts. fstab style mount options are supported.
	// List of all supported mount options.
	// https://github.com/containerd/containerd/blob/master/mount/mount_linux.go#L187-L211
	/*
		mounts := make([]specs.Mount, 0)
		for _, mount := range config.Mounts {
			if (mount.Type == "bind" || mount.Type == "volume") && len(mount.Options) <= 0 {
				return nil, fmt.Errorf("options cannot be empty for mount type: %s. You need to atleast pass rbind and ro", mount.Type)
			}

			// Allow paths relative to $NOMAD_TASK_DIR.
			// More details: https://github.com/hashistack4u/nomad-driver-containerd/issues/116#issuecomment-983171458
			if mount.Type == "bind" && strings.HasPrefix(mount.Source, "local") {
				mount.Source = containerConfig.TaskDirSrc + mount.Source[5:]
			}

			m := buildMountpoint(mount.Type, mount.Target, mount.Source, mount.Options)
			mounts = append(mounts, m)
		}
	*/

	/*
		// Setup host DNS (/etc/resolv.conf) into the container.
		if config.HostDNS {
			opts = append(opts, oci.WithHostResolvconf)
		}

		// Setup "/secrets" (NOMAD_SECRETS_DIR) in the container.
		if containerConfig.SecretsDirSrc != "" && containerConfig.SecretsDirDest != "" {
			secretsMount := buildMountpoint("bind", containerConfig.SecretsDirDest, containerConfig.SecretsDirSrc, []string{"rbind", "rw"})
			mounts = append(mounts, secretsMount)
		}

		// Setup "/local" (NOMAD_TASK_DIR) in the container.
		if containerConfig.TaskDirSrc != "" && containerConfig.TaskDirDest != "" {
			taskMount := buildMountpoint("bind", containerConfig.TaskDirDest, containerConfig.TaskDirSrc, []string{"rbind", "rw"})
			mounts = append(mounts, taskMount)
		}

		// Setup "/alloc" (NOMAD_ALLOC_DIR) in the container.
		if containerConfig.AllocDirSrc != "" && containerConfig.AllocDirDest != "" {
			allocMount := buildMountpoint("bind", containerConfig.AllocDirDest, containerConfig.AllocDirSrc, []string{"rbind", "rw"})
			mounts = append(mounts, allocMount)
		}
	*/

	// User will specify extra_hosts to be added to container's /etc/hosts.
	// If host_network=true, extra_hosts will be added to host's /etc/hosts.
	// If host_network=false, extra hosts will be added to the default /etc/hosts provided to the container.
	// If the user doesn't set anything (host_network, extra_hosts), a default /etc/hosts will be provided to the container.
	/*
		var extraHostsMount specs.Mount
		hostsFile := containerConfig.TaskDirSrc + "/etc_hosts"
		if len(config.ExtraHosts) > 0 {
			if config.HostNetwork {
				if err := etchosts.CopyEtcHosts(hostsFile); err != nil {
					return nil, err
				}
			} else {
				if err := etchosts.BuildEtcHosts(hostsFile); err != nil {
					return nil, err
				}
			}
			if err := etchosts.AddExtraHosts(hostsFile, config.ExtraHosts); err != nil {
				return nil, err
			}
			extraHostsMount = buildMountpoint("bind", "/etc/hosts", hostsFile, []string{"rbind", "rw"})
			mounts = append(mounts, extraHostsMount)
		} else if !config.HostNetwork {
			if err := etchosts.BuildEtcHosts(hostsFile); err != nil {
				return nil, err
			}
			extraHostsMount = buildMountpoint("bind", "/etc/hosts", hostsFile, []string{"rbind", "rw"})
			mounts = append(mounts, extraHostsMount)
		}

		if len(mounts) > 0 {
			opts = append(opts, oci.WithMounts(mounts))
		}
	*/

	// nomad use CNI plugins e.g bridge to setup a network (and network namespace) for the container.
	// CNI plugins need to be installed under /opt/cni/bin.
	// network namespace is created at /var/run/netns/<id>.
	// containerConfig.NetworkNamespacePath is the path to the network namespace, which
	// containerd joins to provide network for the container.
	// NOTE: Only bridge networking mode is supported at this point.

	// FixMe: Current always add NS, is that good enough?
	//if containerConfig.NetworkNamespacePath != "" {
	ns, err := netns.NewNetNS("")
	if err != nil {
		return nil, err
	}
	opts = append(opts, oci.WithWindowsNetworkNamespace(ns.GetPath()))
	//}
	/*
		if containerConfig.User != "" {
			opts = append(opts, oci.WithUser(containerConfig.User))
		}
	*/

	ctxWithTimeout, cancel := context.WithTimeout(d.ctxContainerd, 30*time.Second)
	defer cancel()

	containerdRuntime := d.config.ContainerdRuntime
	if containerdRuntime == "" {
		containerdRuntime = "io.containerd.runhcs.v1"
	}

	return d.client.NewContainer(
		ctxWithTimeout,
		containerConfig.ContainerName,
		containerd.WithRuntime(containerdRuntime, nil),
		containerd.WithNewSnapshot(containerConfig.ContainerSnapshotName, containerConfig.Image),
		containerd.WithNewSpec(opts...),
	)
}
