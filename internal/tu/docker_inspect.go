// mongodb_exporter
// Copyright (C) 2022 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tu

import "time"

type HostPort struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// nolint
type DockerInspectOutput []struct {
	ID      string    `json:"Id"`
	Created time.Time `json:"Created"`
	Path    string    `json:"Path"`
	Args    []string  `json:"Args"`
	State   struct {
		Status     string    `json:"Status"`
		Running    bool      `json:"Running"`
		Paused     bool      `json:"Paused"`
		Restarting bool      `json:"Restarting"`
		OOMKilled  bool      `json:"OOMKilled"`
		Dead       bool      `json:"Dead"`
		Pid        int       `json:"Pid"`
		ExitCode   int       `json:"ExitCode"`
		Error      string    `json:"Error"`
		StartedAt  time.Time `json:"StartedAt"`
		FinishedAt time.Time `json:"FinishedAt"`
	} `json:"State"`
	Image           string      `json:"Image"`
	ResolvConfPath  string      `json:"ResolvConfPath"`
	HostnamePath    string      `json:"HostnamePath"`
	HostsPath       string      `json:"HostsPath"`
	LogPath         string      `json:"LogPath"`
	Name            string      `json:"Name"`
	RestartCount    int         `json:"RestartCount"`
	Driver          string      `json:"Driver"`
	Platform        string      `json:"Platform"`
	MountLabel      string      `json:"MountLabel"`
	ProcessLabel    string      `json:"ProcessLabel"`
	AppArmorProfile string      `json:"AppArmorProfile"`
	ExecIDs         interface{} `json:"ExecIDs"`
	HostConfig      struct {
		Binds           []interface{} `json:"Binds"`
		ContainerIDFile string        `json:"ContainerIDFile"`
		LogConfig       struct {
			Type   string   `json:"Type"`
			Config struct{} `json:"Config"`
		} `json:"LogConfig"`
		NetworkMode  string `json:"NetworkMode"`
		PortBindings struct {
			Two7017TCP []struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"27017/tcp"`
		} `json:"PortBindings"`
		RestartPolicy struct {
			Name              string `json:"Name"`
			MaximumRetryCount int    `json:"MaximumRetryCount"`
		} `json:"RestartPolicy"`
		AutoRemove           bool          `json:"AutoRemove"`
		VolumeDriver         string        `json:"VolumeDriver"`
		VolumesFrom          []interface{} `json:"VolumesFrom"`
		CapAdd               interface{}   `json:"CapAdd"`
		CapDrop              interface{}   `json:"CapDrop"`
		CgroupnsMode         string        `json:"CgroupnsMode"`
		DNS                  interface{}   `json:"Dns"`
		DNSOptions           interface{}   `json:"DnsOptions"`
		DNSSearch            interface{}   `json:"DnsSearch"`
		ExtraHosts           interface{}   `json:"ExtraHosts"`
		GroupAdd             interface{}   `json:"GroupAdd"`
		IpcMode              string        `json:"IpcMode"`
		Cgroup               string        `json:"Cgroup"`
		Links                interface{}   `json:"Links"`
		OomScoreAdj          int           `json:"OomScoreAdj"`
		PidMode              string        `json:"PidMode"`
		Privileged           bool          `json:"Privileged"`
		PublishAllPorts      bool          `json:"PublishAllPorts"`
		ReadonlyRootfs       bool          `json:"ReadonlyRootfs"`
		SecurityOpt          interface{}   `json:"SecurityOpt"`
		UTSMode              string        `json:"UTSMode"`
		UsernsMode           string        `json:"UsernsMode"`
		ShmSize              int           `json:"ShmSize"`
		Runtime              string        `json:"Runtime"`
		ConsoleSize          []int         `json:"ConsoleSize"`
		Isolation            string        `json:"Isolation"`
		CPUShares            int           `json:"CpuShares"`
		Memory               int           `json:"Memory"`
		NanoCpus             int           `json:"NanoCpus"`
		CgroupParent         string        `json:"CgroupParent"`
		BlkioWeight          int           `json:"BlkioWeight"`
		BlkioWeightDevice    interface{}   `json:"BlkioWeightDevice"`
		BlkioDeviceReadBps   interface{}   `json:"BlkioDeviceReadBps"`
		BlkioDeviceWriteBps  interface{}   `json:"BlkioDeviceWriteBps"`
		BlkioDeviceReadIOps  interface{}   `json:"BlkioDeviceReadIOps"`
		BlkioDeviceWriteIOps interface{}   `json:"BlkioDeviceWriteIOps"`
		CPUPeriod            int           `json:"CpuPeriod"`
		CPUQuota             int           `json:"CpuQuota"`
		CPURealtimePeriod    int           `json:"CpuRealtimePeriod"`
		CPURealtimeRuntime   int           `json:"CpuRealtimeRuntime"`
		CpusetCpus           string        `json:"CpusetCpus"`
		CpusetMems           string        `json:"CpusetMems"`
		Devices              interface{}   `json:"Devices"`
		DeviceCgroupRules    interface{}   `json:"DeviceCgroupRules"`
		DeviceRequests       interface{}   `json:"DeviceRequests"`
		KernelMemory         int           `json:"KernelMemory"`
		KernelMemoryTCP      int           `json:"KernelMemoryTCP"`
		MemoryReservation    int           `json:"MemoryReservation"`
		MemorySwap           int           `json:"MemorySwap"`
		MemorySwappiness     interface{}   `json:"MemorySwappiness"`
		OomKillDisable       bool          `json:"OomKillDisable"`
		PidsLimit            interface{}   `json:"PidsLimit"`
		Ulimits              interface{}   `json:"Ulimits"`
		CPUCount             int           `json:"CpuCount"`
		CPUPercent           int           `json:"CpuPercent"`
		IOMaximumIOps        int           `json:"IOMaximumIOps"`
		IOMaximumBandwidth   int           `json:"IOMaximumBandwidth"`
		MaskedPaths          []string      `json:"MaskedPaths"`
		ReadonlyPaths        []string      `json:"ReadonlyPaths"`
	} `json:"HostConfig"`
	GraphDriver struct {
		Data struct {
			LowerDir  string `json:"LowerDir"`
			MergedDir string `json:"MergedDir"`
			UpperDir  string `json:"UpperDir"`
			WorkDir   string `json:"WorkDir"`
		} `json:"Data"`
		Name string `json:"Name"`
	} `json:"GraphDriver"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		Driver      string `json:"Driver"`
		Mode        string `json:"Mode"`
		Rw          bool   `json:"RW"`
		Propagation string `json:"Propagation"`
	} `json:"Mounts"`
	Config struct {
		Hostname     string `json:"Hostname"`
		Domainname   string `json:"Domainname"`
		User         string `json:"User"`
		AttachStdin  bool   `json:"AttachStdin"`
		AttachStdout bool   `json:"AttachStdout"`
		AttachStderr bool   `json:"AttachStderr"`
		ExposedPorts struct {
			Two7017TCP struct{} `json:"27017/tcp"`
		} `json:"ExposedPorts"`
		Tty       bool     `json:"Tty"`
		OpenStdin bool     `json:"OpenStdin"`
		StdinOnce bool     `json:"StdinOnce"`
		Env       []string `json:"Env"`
		Cmd       []string `json:"Cmd"`
		Image     string   `json:"Image"`
		Volumes   struct {
			DataConfigdb struct{} `json:"/data/configdb"`
			DataDb       struct{} `json:"/data/db"`
		} `json:"Volumes"`
		WorkingDir string      `json:"WorkingDir"`
		Entrypoint []string    `json:"Entrypoint"`
		OnBuild    interface{} `json:"OnBuild"`
		Labels     struct {
			ComDockerComposeConfigHash         string `json:"com.docker.compose.config-hash"`
			ComDockerComposeContainerNumber    string `json:"com.docker.compose.container-number"`
			ComDockerComposeOneoff             string `json:"com.docker.compose.oneoff"`
			ComDockerComposeProject            string `json:"com.docker.compose.project"`
			ComDockerComposeProjectConfigFiles string `json:"com.docker.compose.project.config_files"`
			ComDockerComposeProjectWorkingDir  string `json:"com.docker.compose.project.working_dir"`
			ComDockerComposeService            string `json:"com.docker.compose.service"`
			ComDockerComposeVersion            string `json:"com.docker.compose.version"`
		} `json:"Labels"`
	} `json:"Config"`
	NetworkSettings struct {
		Bridge                 string                `json:"Bridge"`
		SandboxID              string                `json:"SandboxID"`
		HairpinMode            bool                  `json:"HairpinMode"`
		LinkLocalIPv6Address   string                `json:"LinkLocalIPv6Address"`
		LinkLocalIPv6PrefixLen int                   `json:"LinkLocalIPv6PrefixLen"`
		Ports                  map[string][]HostPort `json:"Ports"`
		SandboxKey             string                `json:"SandboxKey"`
		SecondaryIPAddresses   interface{}           `json:"SecondaryIPAddresses"`
		SecondaryIPv6Addresses interface{}           `json:"SecondaryIPv6Addresses"`
		EndpointID             string                `json:"EndpointID"`
		Gateway                string                `json:"Gateway"`
		GlobalIPv6Address      string                `json:"GlobalIPv6Address"`
		GlobalIPv6PrefixLen    int                   `json:"GlobalIPv6PrefixLen"`
		IPAddress              string                `json:"IPAddress"`
		IPPrefixLen            int                   `json:"IPPrefixLen"`
		IPv6Gateway            string                `json:"IPv6Gateway"`
		MacAddress             string                `json:"MacAddress"`
		Networks               struct {
			MongodbExporterDefault struct {
				IPAMConfig          interface{} `json:"IPAMConfig"`
				Links               interface{} `json:"Links"`
				Aliases             []string    `json:"Aliases"`
				NetworkID           string      `json:"NetworkID"`
				EndpointID          string      `json:"EndpointID"`
				Gateway             string      `json:"Gateway"`
				IPAddress           string      `json:"IPAddress"`
				IPPrefixLen         int         `json:"IPPrefixLen"`
				IPv6Gateway         string      `json:"IPv6Gateway"`
				GlobalIPv6Address   string      `json:"GlobalIPv6Address"`
				GlobalIPv6PrefixLen int         `json:"GlobalIPv6PrefixLen"`
				MacAddress          string      `json:"MacAddress"`
				DriverOpts          interface{} `json:"DriverOpts"`
			} `json:"mongodb_exporter_default"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}
