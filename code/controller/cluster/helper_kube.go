package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	"fmt"
	"golang.org/x/build/kubernetes"
	"log"
)

// APIServerClient check if kubernetes apiserver avaiable to access
func APIServerClient(ca, apiserver, apiserverKey, url string) (*kubernetes.Client, error) {
	cert, err := tls.X509KeyPair([]byte(apiserver), []byte(apiserverKey))
	if err != nil {
		log.Println("Failed when get tls.X509KeyPair([]byte(apiserver), []byte(apiserverKey)):", err)
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(ca)) {
		log.Println("Failed when caCertPool.AppendCertsFromPEM([]byte(ca))")
		return nil, err
	}

	// Setup HTTPS client
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	httpClient := &http.Client{Transport: transport}

	client, err := kubernetes.NewClient(url, httpClient)
	if err != nil {
		log.Println("Failed, when launcher.NewClient(url, httpClient): ", err)
		return nil, err
	}

	return client, err
}

// APICheck check if apiserver available
func APICheck(ca, apiserver, apiserverKey, url string, retry int) error {
	client, err := APIServerClient(ca, apiserver, apiserverKey, url)
	if err != nil {
		log.Printf(err.Error())
		return err
	}
	defer client.Close()
	for i, j := 0, retry; i < j; i++ {
		_, err = client.GetNodes(context.Background())
		if err != nil {
			fmt.Println(err.Error())
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	return err
}

// APIServerClient check if kubernetes apiserver avaiable to access
func APIServerClientUnsecurity(url string) (*kubernetes.Client, error) {
	transport := &http.Transport{}
	httpClient := &http.Client{Transport: transport}

	client, err := kubernetes.NewClient(url, httpClient)
	if err != nil {
		log.Println("Failed, when launcher.NewClient(url, httpClient): ", err)
		return nil, err
	}

	return client, err
}

func APICheckUnsecurity(url string, retry int) error {
	client, err := APIServerClientUnsecurity(url)
	if err != nil {
		return err
	}
	defer client.Close()
	for i, j := 0, retry; i < j; i++ {
		_, err = client.GetNodes(context.Background())
		if err != nil {
			log.Println(err.Error())
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	return err
}
