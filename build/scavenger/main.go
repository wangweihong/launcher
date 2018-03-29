package main

import (
	"fmt"

	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

var client *docker.Client

// removeExitedCon 删除已经退出的容器
// 防止误删除，容器退出时间超过 10 分钟
func removeExitedCon() {
	exitedLives := time.Minute * 10 // 容器退出的最大容忍的时间，超过这个时间即删除，单位　秒

	apiContainers, err := client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	for _, apiContainer := range apiContainers {
		if !strings.Contains(apiContainer.State, "xited") {
			continue
		}

		con, err := client.InspectContainer(apiContainer.ID)
		if err != nil {
			continue
		}
		if con.State.FinishedAt.Before(time.Now().Add(-1 * exitedLives)) {
			fmt.Printf("%s already exited, will be removed soon\n", apiContainer.ID)
			// 删除容器，连带删除卷
			// try 2 times.
			err := client.RemoveContainer(docker.RemoveContainerOptions{ID:apiContainer.ID, RemoveVolumes: true, Force: true})
			if err != nil {
				fmt.Printf("%v", err)
				client.RemoveContainer(docker.RemoveContainerOptions{ID:apiContainer.ID, RemoveVolumes: true, Force: true})
			}
		}
	}
}

func removeNoTagImages(){
	filter := map[string][]string{}
	filter["dangling"] = []string{"true"}

	images, err := client.ListImages(docker.ListImagesOptions{All: true, Filters: filter})
	if err != nil {
		return
	}
	for _, image := range images {
		if err := client.RemoveImage(image.ID); err != nil {
			fmt.Println(err)
		}
	}
}

func removeNoUsedVolumes(){
	filter := map[string][]string{}
	filter["dangling"] = []string{"true"}

	volumes, err := client.ListVolumes(docker.ListVolumesOptions{Filters: filter})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, volume := range volumes {
		client.RemoveVolume(volume.Name)
	}
}

func cutLogFile(){
	cons, err := client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		return
	}

	for _, con := range cons {
		// 只处理正在运行的容器
		if !strings.Contains(con.State, "unning") {
			continue
		}

		conDetail, err := client.InspectContainer(con.ID)
		if err != nil {
			continue
		}

		if len(conDetail.LogPath) == 0 {
			fmt.Printf("This docker version not support, can't get %s's log path from docker", conDetail.ID)
			continue
		}

		CutFile(conDetail.LogPath)
	}
}

func waiting(){
	// Wait a long term, for machine restart or something else, for clean safely.
	// default waiting for 10 minutes.
	time.Sleep(time.Minute * 30)
}

func main() {
	var err error
	endpoint := "unix:///var/run/docker.sock"
	client, err = docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}

	msg := `

======================================
    scavenger works
======================================
`
	fmt.Println("")

	waiting()

	for {
		// 显示提示信息
		fmt.Printf("%s %v\n", msg, time.Now())

		// 删除已经退出的容器
		removeExitedCon()

		// 删除悬挂的镜像
		removeNoTagImages()

		// 删除悬挂的卷
		removeNoUsedVolumes()

		// 截断容器日志文件
		cutLogFile()

		// 每分钟执行一次
		time.Sleep(time.Minute)
	}
}

