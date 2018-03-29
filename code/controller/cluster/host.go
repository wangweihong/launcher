package cluster

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"ufleet/launcher/code/utils"
	"strings"
)

// getSSHClient get ssh client, if not exists, create it.
func (h *Host) GetSSHClient() (*ssh.Client, error) {
	if h.sshClient == nil {
		if h.HostSSHNetwork == "" {
			h.HostSSHNetwork = "tcp"
		}
		if h.HostSSHPort == "" {
			h.HostSSHPort = "22"
		}

		// Step : 检查主机连接情况
		sshClient, err := SSHClient(*h)
		if err != nil {
			return nil, fmt.Errorf("SSH Connect Failed! ErrorMsg: " + err.Error())
		}
		h.sshClient = sshClient
	}
	return h.sshClient, nil
}

// getNetworkCardName get network card name
func (h *Host) getNetworkCardName() (string, error) {
	sshClient, err := h.GetSSHClient()
	if err != nil {
		return "", fmt.Errorf("get ssh client failed")
	}

	cmd := fmt.Sprintf("ip addr | grep -v grep | tr '/' ' ' | grep ' %s ' | head -n 1 | awk '{print $NF}'", h.HostIP)
	netcard, err := utils.Execute(cmd, sshClient)
	if err != nil {
		return "", err
	}
	if len(netcard) == 0 {
		return "", fmt.Errorf("get network card name failed")
	}
	netcard = strings.Replace(netcard, "\n", "", -1)

	return netcard, nil
}