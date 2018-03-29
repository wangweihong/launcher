package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

// Spinner process bar
func Spinner(delay time.Duration) {
	for {
		for _, r := range `-\|/` {
			fmt.Printf("\r%c", r)
			time.Sleep(delay)
		}
	}
}

//获取指定目录及所有子目录下的所有文件，可以匹配后缀过滤。即使dirPth为文件，也支持
func WalkDir(dirPth string) (files []string, err error) {
	fmt.Println(dirPth)

	// 判断文件是否存在，不存在直接报错退出
	exist, err := PathExists(dirPth)
	if !exist {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s not exist", dirPth)
	}

	files = make([]string, 0, 64)
	err = filepath.Walk(dirPth, func(filename string, fi os.FileInfo, err error) error { //遍历目录
		if fi.IsDir() {
			if strings.Compare(dirPth, filename) != 0 { // 进入子目录继续遍历目录
				WalkDir(filename)
			}
			return nil
		}
		fname, err := filepath.Rel(dirPth, filename) // 获取相对l录的相对路径名
		if err != nil {
			return nil
		}
		files = append(files, fname)
		return nil
	})
	return files, err
}

// 获取IP地址的某个字段，ipv4只有 4个字段
func GetIPField(ip string, field int) string {
	if field > 4 || field < 1 {
		return ""
	}

	bits := strings.Split(ip, ".")
	if len(bits) == 4 {
		return bits[field - 1]
	}

	return ""
}

// 提取字符串中的字母、数字和中横杆-
func GetValidCh(src string) string {
	res := make([]byte, 0, len(src))

	chArray := []byte(src)
	for _, ch := range chArray {
		if value:=int(ch); value >= int('a') && value <= int('z') {
			res = append(res, ch)
			continue
		}
		if value:=int(ch); value >= int('A') && value <= int('Z') {
			res = append(res, ch)
			continue
		}
		if value:=int(ch); value >= int('0') && value <= int('9') {
			res = append(res, ch)
			continue
		}
		if value:=int(ch); value == int('-') {
			res = append(res, ch)
			continue
		}
	}

	return string(res)
}

func GetLowerCh(src string) string {
	res := make([]byte, 0, len(src))

	chArray := []byte(src)
	for _, ch := range chArray {
		if value:=int(ch); value >= int('a') && value <= int('z') {
			res = append(res, ch)
			continue
		}
	}

	return string(res)
}

// GetSystemType 获取系统类型
// 		当前支持的类型： centos，ubuntu，redhat，debian，unkown
func GetSystemType(sshClient *ssh.Client) (string, error) {
	cmd := "cat /etc/os-release*"
	resp, err := Execute(cmd, sshClient)
	if err != nil {
		return "", err
	}
	resp = strings.ToLower(resp)
	for _, systemType := range []string{"centos", "ubuntu", "redhat", "debian"} {
		if strings.Contains(resp, systemType) {
			return systemType, nil
		}
	}

	return "unkown", nil
}