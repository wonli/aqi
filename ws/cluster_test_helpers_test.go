package ws

func setClusterTransport(t ClusterTransport) {
	clearClusterTransport()
	if t == nil {
		return
	}
	instanceID, err := newClusterInstanceID()
	if err != nil {
		panic("aqi: cannot generate cluster instance id: " + err.Error())
	}
	if !installClusterTransport(t, instanceID) {
		panic("aqi: cannot install test cluster transport")
	}
}

func clearClusterTransport() {
	if old := clusterState.Swap(nil); old != nil {
		_ = old.transport.Close()
	}
}

func clusterCurrentInstanceID() [clusterInstanceIDSize]byte {
	if c := clusterState.Load(); c != nil {
		return c.instanceID
	}
	return [clusterInstanceIDSize]byte{}
}
