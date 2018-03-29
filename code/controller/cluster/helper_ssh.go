package cluster

import (
	"encoding/pem"
	"time"
	"golang.org/x/crypto/ssh"
	"crypto/x509"
	"fmt"
	"net"
)

// SSHClient remote host and return client
func SSHClient(host Host) (*ssh.Client, error) {
	auths := []ssh.AuthMethod{}
	if host.UserPwd != "" {
		auths = []ssh.AuthMethod{ssh.Password(host.UserPwd)}
	} else if len(host.Prikey) != 0 {
		if len(host.PrikeyPwd) == 0 {
			signer, err := ssh.ParsePrivateKey([]byte(host.Prikey))
			if err != nil {
				return nil, err
			}
			auths = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		} else {
			block, rest := pem.Decode([]byte(host.Prikey))
			if len(rest) > 0 {
			}
			der, err := x509.DecryptPEMBlock(block, []byte(host.PrikeyPwd))
			if err != nil {
				return nil, err
			}
			dkey, err := x509.ParsePKCS1PrivateKey(der)
			if err != nil {
				return nil, err
			}
			signer, err := ssh.NewSignerFromKey(dkey)
			auths = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	} else {
		return nil, fmt.Errorf("no connect info provide")
	}

	// user host config to make connect
	config := &ssh.ClientConfig{
		User: host.UserName,
		Auth: auths,
		Timeout: time.Second * 30, // set timeout
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	client, err := ssh.Dial(
		host.HostSSHNetwork,
		host.HostIP+":"+host.HostSSHPort,
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("HostSSHNetwork: %s, Connect: %s:%s, ErrorMsg: %s", host.HostSSHNetwork, host.HostIP, host.HostSSHPort, err.Error())
	}
	return client, nil
}
