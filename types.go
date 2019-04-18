package main

import (
	"sync"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/tools/cache"
        "k8s.io/client-go/util/workqueue"
)

type Controller struct {
        indexer   cache.Indexer
        queue     workqueue.RateLimitingInterface
        informer  cache.Controller
        clientset kubernetes.Interface
        mutex     *sync.Mutex
        state     map[string]map[string]Deployment // map[TEAM]map[NAME]Deployment
	debug     bool
}

type Deployment struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Image           string `json:"image"`
	LastTimestamp   int64  `json:"lastTimestamp"`
	Success         bool   `json:"success"`
	ImageChanged    bool   `json:"imageChanged"`
	RecoverySeconds int64  `json:"recoverySeconds,omitempty"`
}
