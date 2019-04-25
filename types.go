package main

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sync"
)

// Controller represents the controller state
type Controller struct {
	indexer   cache.Indexer
	queue     workqueue.RateLimitingInterface
	informer  cache.Controller
	clientset kubernetes.Interface
	mutex     *sync.Mutex
	state     map[string]map[string]Deployment // map[TEAM]map[NAME]Deployment
	debug     bool
}

// Deployment captures the information written to stdout
type Deployment struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Image           string `json:"image"`
	Flags           string `json:"flags"`
	LastTimestamp   int64  `json:"lastTimestamp"`
	CommitTimestamp int64  `json:"commitTimestamp"`
	Success         bool   `json:"success"`
	ImageChanged    bool   `json:"imageChanged"`
	RecoverySeconds int64  `json:"recoverySeconds,omitempty"`
	LeadTimeSeconds int64  `json:"leadTimeSeconds,omitempty"`
}
