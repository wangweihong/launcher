package utils

import (
	"fmt"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHSession create a new session for client
func SSHSession(client *ssh.Client) (*ssh.Session, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Execute command on remote machine
func Execute(command string, client *ssh.Client) (string, error) {
	session, err := SSHSession(client)
	if err != nil {
		return "", err
	}
	res, err := session.CombinedOutput(command)
	if err != nil {
		return "", err
	}
	resStr := string(res[:])
	fmt.Printf("%v", resStr)
	defer session.Close()
	return resStr, nil
}

// SFTPClient client for sftp
func SFTPClient(client *ssh.Client) (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	return sftpClient, nil
}

// SFTPSender send files to remote host
func SFTPSender(localFile string, remotePath string, sftpClient *sftp.Client, buf []byte) error {
	srcFile, err := os.Open(localFile)
	if err != nil {
		return fmt.Errorf("open file %s failed: %s", localFile, err.Error())
	}
	srcFileStat, err := os.Stat(localFile)
	if err != nil {
		return fmt.Errorf("get file %s's privilege failed: %s", localFile, err.Error())
	}
	defer srcFile.Close()

	dstFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("sftp create %s failed.: %s", remotePath, err.Error())
	}
	defer dstFile.Close()

	for {
		n, _ := srcFile.Read(buf)
		if n == 0 {
			break
		}
		dstFile.Write(buf[:n])
	}

	dstFile.Chmod(srcFileStat.Mode()) // change file's privilege

	return nil
}