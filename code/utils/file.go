package utils

import (
	"os"
	"path"
	"os/exec"
	"bytes"
	"golang.org/x/crypto/ssh"
	"path/filepath"
	"io"
	"fmt"
	"text/template"
	"io/ioutil"
)

func GenNewFile(filepath string, content string) error{
	dirPath := path.Dir(filepath)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return err
	}
	dstFile, err := os.Create(filepath)
	if err != nil{
		return err
	}
	defer dstFile.Close()
	dstFile.WriteString(content + "\n")
	return nil
}

func GetFileContent(filepath string, istr *string) error {
	command := "/bin/cat " + filepath
	cmd := exec.Command("/bin/sh", "-c", command)
	var out bytes.Buffer

	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	*istr = out.String()

	return nil
}

/**
 * sshClient ssh客户端
 * map[srcdir string] destdir string: srcdir 源目录，destdir 目的目录
 */
func SendToRemote(sshClient *ssh.Client, dir map[string]string) error {
	sftpClient, err := SFTPClient(sshClient)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	MEGABYTES := 1024 * 1024
	buf := make([]byte, MEGABYTES * 4)

	for ldir, rdir := range dir {
		fileInfo, err := os.Stat(ldir)
		if err != nil {
			return err
		}

		// 如果 ldir 为文件
		if !fileInfo.IsDir() {
			fname := filepath.Base(ldir)
			remotePath := path.Join(rdir, fname)
			_, err = Execute("mkdir -p " + path.Dir(remotePath), sshClient)
			if err != nil {
				return fmt.Errorf("mkdir -p %s failed: %s", path.Dir(remotePath), err.Error())
			}
			if err := SFTPSender(ldir, remotePath, sftpClient, buf); err != nil{
				return err
			}
			continue
		}

		files, err := WalkDir(ldir)
		if err != nil {
			return fmt.Errorf("[ SendInstallFiles ] walk dir failed: %s", err.Error())
		}
		for _, fname := range files {
			remotePath := path.Join(rdir, fname)
			remoteBaseDir := path.Dir(remotePath)
			_, err = Execute("mkdir -p "+remoteBaseDir, sshClient)
			if err != nil {
				return fmt.Errorf("mkdir -p %s failed: %s", remoteBaseDir, err.Error())
			}
			if err = SFTPSender(path.Join(ldir, fname), remotePath, sftpClient, buf); err != nil {
				return err
			}
		}
	}

	return nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func CopyDir(srcDir string, destDir string) (err error) {
	// 遍历获取source下所有文件
	files, err := WalkDir(srcDir)
	if err != nil {
		return fmt.Errorf("walk dir failed: %s", err.Error())
	}
	for _, fname := range files {
		sourcePath := path.Join(srcDir, fname)
		remotePath := path.Join(destDir, fname)
		remoteBaseDir := path.Dir(remotePath)
		srcinfo, err := os.Stat(path.Dir(sourcePath))
		if err != nil {
			return err
		}
		if err = os.MkdirAll(remoteBaseDir, srcinfo.Mode()); err != nil {
			return err
		}
		if err = CopyFile(sourcePath, remotePath); err != nil {
			return err
		}
	}

	return nil
}

func CopyFile(source string, dest string) (err error) {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourcefile.Close()

	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destfile.Close()

	if _, err = io.Copy(destfile, sourcefile); err == nil {
		sourceinfo, err := os.Stat(source)
		if err != nil {
			return err
		}
		if err = os.Chmod(dest, sourceinfo.Mode()); err != nil {
			return err
		}
	}

	return nil
}

// exists returns whether the given file or directory exists or not
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
	return true, err
}

func TemplateReplaceByObject(path string, t *template.Template, object interface{}) error {
	thedir := filepath.Dir(path)
	err := os.MkdirAll(thedir, 0755)
	if err != nil {
		return err
	}
	thefile, err := os.OpenFile(path, os.O_CREATE | os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	if err = t.Execute(thefile, object); err != nil {
		return err
	}
	if err = thefile.Close(); err != nil {
		return err
	}

	return nil
}

func TmplReplaceByObject(path string, tmpl string, object interface{}, perm os.FileMode) (err error) {
	destObjectBytes, err := ParseTemplate(tmpl, object)
	if err != nil {
		return
	}

	thedir := filepath.Dir(path)
	err = os.MkdirAll(thedir, 0755)
	if err != nil {
		return
	}

	err = ioutil.WriteFile(path, destObjectBytes, perm)

	return
}