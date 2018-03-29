package main

import (
	"flag"
	"github.com/ant0ine/go-json-rest/rest"
	"net/http"
	"os"

	"ufleet/launcher/code/model/base"
	"ufleet/launcher/code/view/apiserver/http"
	"ufleet/launcher/code/controller/worker"
	"ufleet/launcher/code/controller/scheduler"
	"ufleet/launcher/code/config"
	"path"
	"log"
	"strconv"
	"fmt"
)

func main() {
	/* set and parse params */
	// etcd
	etcdServer := flag.String("etcd", fmt.Sprintf("http://%s:%d", config.GDefault.HostIP, config.GDefault.PortEtcd), "etcd endpoint")
	// launcher listening port
	serverPort := flag.String("port", strconv.Itoa(config.GDefault.PortListen), "service port")
	// set log
	err := os.MkdirAll(path.Dir(config.GDefault.LogPath), 0700) // create log dir.
	if err != nil {
		panic(err)
	}
	logPath := flag.String("log", config.GDefault.LogPath, "log file path")
	logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logFile)
	log.SetFlags(config.GDefault.LogLevel)

	// parse params
	flag.Parse()

	log.Printf("Begin launcher...")
	// set server mode
	config.GDefault.ServiceType = "server"

	// init etcd which launcher depended
	base.InitDB(*etcdServer)
	log.Printf("start scheduler...")
	go scheduler.NewScheduler()
	log.Printf("start worker...")
	go worker.NewWorker()

	log.Printf("start apiserver...")
	restapi := rest.NewApi()
	restapi.Use(rest.DefaultDevStack...)
	router, err := rest.MakeRouter(
		rest.Get(apiserver.ClusterPath, apiserver.GetClusters),
		rest.Get(apiserver.ClusterPath+"/#clustername", apiserver.GetCluster),
		rest.Post(apiserver.ClusterPath+apiserver.CreatePath, apiserver.PostCluster),
		rest.Delete(apiserver.ClusterPath+"/#clustername", apiserver.RemoveCluster),
		rest.Post(apiserver.ClusterPath+apiserver.LoadPath, apiserver.LoadCluster),

		rest.Get(apiserver.ClusterPath+"/#clustername"+apiserver.MasterPath, apiserver.GetMasters),
		rest.Get(apiserver.ClusterPath+"/#clustername"+apiserver.MasterPath+"/#mastername", apiserver.GetMaster),
		rest.Post(apiserver.ClusterPath+"/#clustername"+apiserver.MasterPath+apiserver.CreatePath, apiserver.AddMaster),
		rest.Delete(apiserver.ClusterPath+"/#clustername"+apiserver.MasterPath+"/#mastername", apiserver.RemoveMaster),

		rest.Get(apiserver.ClusterPath+"/#clustername"+apiserver.NodePath, apiserver.GetNodes),
		rest.Get(apiserver.ClusterPath+"/#clustername"+apiserver.NodePath+"/#nodename", apiserver.GetNode),
		rest.Post(apiserver.ClusterPath+"/#clustername"+apiserver.NodePath+apiserver.CreatePath, apiserver.PostNode),
		rest.Delete(apiserver.ClusterPath+"/#clustername"+apiserver.NodePath+"/#nodename", apiserver.RemoveNode),

		rest.Get(apiserver.FederationPath, apiserver.GetFederations),
		rest.Get(apiserver.FederationPath+"/#fedname", apiserver.GetFederation),
		rest.Post(apiserver.FederationPath+"/#fedname"+apiserver.CreatePath, apiserver.PostFederation),
		rest.Delete(apiserver.FederationPath+"/#fedname", apiserver.RemoveFederation),
		rest.Post(apiserver.FederationPath+"/#fedname"+ apiserver.FollowerPath +"/#clustername"+apiserver.CreatePath, apiserver.AddFederationFollower),
		rest.Delete(apiserver.FederationPath+"/#fedname"+apiserver.FollowerPath+"/#clustername", apiserver.DeleteFederationFollower),

		rest.Get(apiserver.ClusterPath+"/#clustername"+apiserver.StoragePath+"/#sclustername", apiserver.GetStorage),
		rest.Post(apiserver.ClusterPath+"/#clustername"+apiserver.StoragePath+apiserver.CreatePath, apiserver.PostStorage),
		rest.Delete(apiserver.ClusterPath+"/#clustername"+apiserver.StoragePath+"/#sclustername", apiserver.DeleteStorage),
		rest.Post(apiserver.ClusterPath+"/#clustername"+apiserver.StoragePath+"/#sclustername"+"/#nodename", apiserver.AddStorageNode),
		rest.Delete(apiserver.ClusterPath+"/#clustername"+apiserver.StoragePath+"/#sclustername"+"/#nodename", apiserver.RemoveStorageNode),
	)
	if err != nil {
		log.Printf(err.Error())
	}
	restapi.SetApp(router)

	log.Printf("Begin listening...")
	http.ListenAndServe(":"+*serverPort, restapi.MakeHandler())
}
