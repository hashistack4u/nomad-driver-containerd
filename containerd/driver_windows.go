package containerd

func getNamespaceName() string {
	return "nomad"
}

func getContainerName(name, allocID string) string {
	containerName := name + "-" + allocID
	return containerName
}
