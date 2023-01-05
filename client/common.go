package client

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
)

var ClientSet *kubernetes.Clientset

func init() {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Panicf("k8s init in cluster config failed: %v", err)
	}

	ClientSet, err = kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		log.Panicf("k8s create new client failed: %v", err)
	}

}
