package cluster

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
)

func NewK8sClient(host string, certData []byte, keyData []byte, timeout time.Duration) (*kubernetes.Clientset,error) {
	tlsClientConfig:= rest.TLSClientConfig{
		Insecure: true, // not check ca cert
		CertData: certData,
		KeyData:  keyData,
	}
	config := rest.Config{
		Host: host,
		TLSClientConfig: tlsClientConfig,
		Timeout: timeout,
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(&config)
	if err != nil {
		return nil,err
	}

	return clientset, nil
}
