package main

import (
	"os"
	"fmt"
	"io"
)

// 大于　12M 处理，开始截取，只保留最近的 8M
const chunkSize int64 = 8  << 20 // 8M
const maxSize   int64 = 12 << 20 // 12M

func CutFile(filePath string) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Println(err)
		return
	}

	if fileInfo.Size() < maxSize {
		// 低于需要处理的长度，不处理
		return
	}

	// 读取文件
	destFile, err := os.OpenFile(filePath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(){
		if destFile != nil {
			destFile.Close()
		}
	}()

	// 临时文件
	tmpFile, err := os.OpenFile(filePath + ".new", os.O_CREATE|os.O_RDWR, 0640)
	defer func(){
		if tmpFile != nil {
			tmpFile.Close()
		}
	}()

	// 定位到拷贝的位置
	destFile.Seek(-1 * chunkSize, 2)
	io.Copy(tmpFile, destFile)

	// 关闭文件
	destFile.Close()
	tmpFile.Close()

	// 删除老的文件
	err = os.Remove(filePath)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 使用新的文件替换
	err = os.Rename(filePath+".new", filePath)
	if err != nil {
		fmt.Println(err)
		return
	}
}
