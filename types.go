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
        state     map[string][]Deployment // map[TEAM][]Deployment
}

type Deployment struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	TimeScheduled int64  `json:"timeScheduled"`
	TimeCompleted int64  `json:"timeCompleted"`
	Success       bool   `json:"success"`
}
