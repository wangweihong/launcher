package config

var (
	GK8sDefault = struct {
		K8sVersion            string
		TimesOfCheckApiserver int
		TimesOfCheckEtcd      int
		EtcdListenPort        int
		EtcdPeerPort          int
		ChangeHostname        string
		CheckSubnetwork       string
		FederationZones       string
		SAHelm                string
	}{
		"v1.8.8",
		20,
		60,
		12379,
		12380,
		"true",
		"true",
		"example.com.",
		"helmor",
	}

	GDefault = struct {
		ServiceType    string
		CurrentDir     string
		HostIP         string
		LocalTempDir   string
		StorePath      string
		PortEtcd       int
		PortListen     int
		LogPath        string
		LogLevel       int
		RemoteTempDir  string
		RemoteLogDir   string
		BaseRegistory  string
		NtpdHost       string
		RegistryIp     string
		PrometheusPort int
		GrafanaPort  int
	}{
		"module",
		"",
		"127.0.0.1",
		"/tmp/launcher",
		"/var/data/xfleet/launcher.db",
		32379,
		8886,
		"/var/log/launcher/launcher.log",
		3,
		"/root/k8s",
		"/root/k8s/log",
		"ufleet.io",
		"",
		"",
		32380,
		32381,
	}
)
