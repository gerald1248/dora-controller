package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"

	appsv1 "k8s.io/api/apps/v1"
	rest "k8s.io/client-go/rest"

	au "github.com/logrusorgru/aurora"
)

const annotationPrefix = "dora"
const annotationNameTeam = "team"
const annotationNameCommitTimestamp = "commitTimestamp"

// NewController constructs the central controller state
func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, clientset kubernetes.Interface, mutex *sync.Mutex, state map[string]map[string]Deployment, debug bool) *Controller {
	return &Controller{
		informer:  informer,
		indexer:   indexer,
		queue:     queue,
		clientset: clientset,
		mutex:     mutex,
		state:     state,
		debug:     debug,
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncToStdout(key.(string))
	c.handleErr(err, key)

	return true
}

func (c *Controller) syncToStdout(key string) error {
	obj, keyExists, err := c.indexer.GetByKey(key)
	if err != nil {
		log(fmt.Sprintf("%s: fetching object with key %s from store failed with %v", au.Bold(au.Red("Error")), key, err))
		return err
	}

	// exit condition: ignore deployments without annotation
	team := obj.(*appsv1.Deployment).ObjectMeta.Annotations[fmt.Sprintf("%s/%s", annotationPrefix, annotationNameTeam)]
	if len(team) == 0 {
		return nil
	}

	var commitTimestampString string // string representation of int64
	var commitTimestamp int64 // we store this form
	commitTimestampString = obj.(*appsv1.Deployment).ObjectMeta.Annotations[fmt.Sprintf("%s/%s", annotationPrefix, annotationNameCommitTimestamp)]

	intVar, err := strconv.Atoi(commitTimestampString)
	if err != nil {
		commitTimestamp = 0
	} else {
		commitTimestamp = int64(intVar)
	}

	name := obj.(*appsv1.Deployment).GetName()
	namespace := obj.(*appsv1.Deployment).ObjectMeta.Namespace
	image := obj.(*appsv1.Deployment).Spec.Template.Spec.Containers[0].Image
	replicas := obj.(*appsv1.Deployment).Status.Replicas
	readyReplicas := obj.(*appsv1.Deployment).Status.ReadyReplicas
	unavailableReplicas := obj.(*appsv1.Deployment).Status.UnavailableReplicas
	updatedReplicas := obj.(*appsv1.Deployment).Status.UpdatedReplicas
	conditions := obj.(*appsv1.Deployment).Status.Conditions
	lastTimestamp := conditions[len(conditions)-1].LastTransitionTime
	lastType := conditions[len(conditions)-1].Type
	lastStatus := conditions[len(conditions)-1].Status
	lastReason := conditions[len(conditions)-1].Reason

	_, teamExists := c.state[team]

	c.mutex.Lock()
	if !teamExists {
		c.state[team] = map[string]Deployment{}
	}
	c.mutex.Unlock()

	if !keyExists {
		log(fmt.Sprintf("%s: deployment %s deleted", au.Bold(au.Cyan("INFO")), key))
		c.mutex.Lock()
		if teamExists {
			c.state[team][name] = Deployment{}
		}
		c.mutex.Unlock()
		return nil
	}
	log(fmt.Sprintf("%s: scanning deployment %s", au.Bold(au.Cyan("INFO")), name))
	success := false
	c.mutex.Lock()
	if !teamExists {
		c.state[team] = map[string]Deployment{}
	}
	c.mutex.Unlock()

	// NEW images do not qualify as UPDATED images
	// don't process all available deployments right away
	imageChanged := false
	var recoverySeconds int64
	recoverySeconds = 0
	_, nameExists := c.state[team][name]
	previousSuccess := true
	if nameExists {
		previous := c.state[team][name]
		imageChanged = image != previous.Image
		previousSuccess = previous.Success
		if !previousSuccess {
			recoverySeconds = lastTimestamp.Unix() - previous.LastTimestamp
		}
	}

	if lastType == "Available" && lastStatus == "True" {
		// Successful deployment
		success = true
	} else if lastType == "Progressing" && lastStatus == "True" {
		// NB: typically the deployment remains stuck at the "Progressing" stage
		// Treat as successful only if the reason property is set to NewReplicaSetAvailable
		if lastReason == "NewReplicaSetAvailable" {
			success = true
		} else {
			log(fmt.Sprintf("%s: skipping - rollout in progress", au.Bold(au.Cyan("INFO"))))
			return nil
		}
	} else if lastType == "Progressing" && lastStatus == "False" {
		if !previousSuccess {
			return nil
		}
		success = false
	} else if lastType == "ReplicaFailure" {
		if !previousSuccess {
			return nil
		}
		// now record failed deployment and set previous deployment
		success = false
	}

	// ignore successful redeployments - only code changes count
	if success && !imageChanged {
		return nil
	}

	flags := getDeploymentFlags(success, previousSuccess, imageChanged)

	var leadTimeSeconds int64
	if commitTimestamp > 0 {
		leadTimeSeconds = lastTimestamp.Unix() - commitTimestamp
	}
	deployment := Deployment{
		name,
		namespace,
		image,
		flags,
		lastTimestamp.Unix(), //int64
		commitTimestamp, //int64
		success,
		imageChanged,
		recoverySeconds,
		leadTimeSeconds,
	}

	c.mutex.Lock()
	c.state[team][name] = deployment
	debug := c.debug
	c.mutex.Unlock()

	if len(team) > 0 {
		if debug {
			fmt.Fprintf(os.Stderr, "=> Team: %s\n", au.Bold(team))
			fmt.Fprintf(os.Stderr, "=> Image %s\n", au.Bold(image))
			fmt.Fprintf(os.Stderr, "=> Replicas %d\n", au.Bold(replicas))
			fmt.Fprintf(os.Stderr, "=> ReadyReplicas %d\n", au.Bold(readyReplicas))
			fmt.Fprintf(os.Stderr, "=> UnavailableReplicas %d\n", au.Bold(unavailableReplicas))
			fmt.Fprintf(os.Stderr, "=> UpdatedReplicas %d\n", au.Bold(updatedReplicas))
			fmt.Fprintf(os.Stderr, "=> LastTransitionTime %s\n", au.Bold(lastTimestamp))
			if (commitTimestamp > 0) {
				fmt.Fprintf(os.Stderr, "=> CommitTimestamp %s\n", au.Bold(commitTimestampString))
			}
			fmt.Fprintf(os.Stderr, "=> Type %s\n", au.Bold(lastType))
			fmt.Fprintf(os.Stderr, "=> Reason %s\n", au.Bold(lastReason))
			fmt.Fprintf(os.Stderr, "=> Status %s\n", au.Bold(lastStatus))
			fmt.Fprintf(os.Stderr, "=> Success %t\n", au.Bold(success))
			fmt.Fprintf(os.Stderr, "=> Raw %v\n", au.Bold(conditions))
		}

		bytes, err := json.Marshal(deployment)
		if err != nil {
			log(fmt.Sprintf("%s: %s", au.Bold(au.Red("Error")), au.Bold(err)))
			return nil
		}
		// main JSON output goes to stdout
		fmt.Printf("%s\n", bytes)
	}
	if c.queue.Len() == 0 {
		// TODO: is this significant here?
	}
	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < 5 {
		log(fmt.Sprintf("%s: can't sync deployment %v: %v", au.Bold(au.Red("Error")), key, err))
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	runtime.HandleError(err)
	log(fmt.Sprintf("%s: dropping deployment %q from the queue: %v", au.Bold(au.Cyan("INFO")), key, err))
}

// Run manages the controller lifecycle
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	log(fmt.Sprintf("%s: starting DORA controller", au.Bold(au.Cyan("INFO"))))

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	log(fmt.Sprintf("%s: stopping DORA controller", au.Bold(au.Cyan("INFO"))))
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func main() {
	var kubeconfig string
	var master string
	var debug bool

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
	flag.Parse()

	// support out-of-cluster deployments (param, env var only)
	if len(kubeconfig) == 0 {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	var config *rest.Config
	var configError error

	if len(kubeconfig) > 0 {
		config, configError = clientcmd.BuildConfigFromFlags(master, kubeconfig)
		if configError != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", au.Bold(au.Red("Out-of-cluster error")), configError)
			return
		}
	} else {
		config, configError = rest.InClusterConfig()
		if configError != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", au.Bold(au.Red("In-cluster error")), configError)
			return
		}

	}

	// creates clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", au.Bold(au.Red("Error")), err)
		return
	}

	var mutex = &sync.Mutex{}
	var state = map[string]map[string]Deployment{}

	deploymentListWatcher := cache.NewListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", "", fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(deploymentListWatcher, &appsv1.Deployment{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	controller := NewController(queue, indexer, informer, clientset, mutex, state, debug)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	select {}
}
