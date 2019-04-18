package main

import (
	"flag"
	"encoding/json"
	"fmt"
	"os"
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
        au "github.com/logrusorgru/aurora"
)

const annotationPrefix = "dora"
const annotationName = "team"

func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, clientset kubernetes.Interface, mutex *sync.Mutex, state map[string]map[string]Deployment, debug bool) *Controller {
	return &Controller{
		informer:    informer,
		indexer:     indexer,
		queue:       queue,
		clientset:   clientset,
		mutex:       mutex,
		state:       state,
		debug:       debug,
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
                fmt.Fprintf(os.Stderr, "%s: fetching object with key %s from store failed with %v", au.Bold(au.Red("Error")), key, err)
                return err
        }

	// exit condition: ignore deployments without annotation
        team := obj.(*appsv1.Deployment).ObjectMeta.Annotations[fmt.Sprintf("%s/%s", annotationPrefix, annotationName)]
	if len(team) == 0 {
		return nil
	}

        name := obj.(*appsv1.Deployment).GetName()
        namespace := obj.(*appsv1.Deployment).ObjectMeta.Namespace
        image := obj.(*appsv1.Deployment).Spec.Template.Spec.Containers[0].Image
        replicas := obj.(*appsv1.Deployment).Status.Replicas
        readyReplicas := obj.(*appsv1.Deployment).Status.ReadyReplicas
        conditions := obj.(*appsv1.Deployment).Status.Conditions
        lastTimestamp := conditions[len(conditions)-1].LastTransitionTime
        lastType := conditions[len(conditions)-1].Type
        lastStatus := conditions[len(conditions)-1].Status

        _, teamExists := c.state[team]

        c.mutex.Lock()
        if !teamExists {
		c.state[team] = map[string]Deployment{}
        }
        c.mutex.Unlock()

        if !keyExists {
                fmt.Fprintf(os.Stderr, "%s: deployment %s deleted\n", au.Bold(au.Cyan("Info")), key)
                c.mutex.Lock()
		if teamExists {
			c.state[team][name] = Deployment{}
		}
                c.mutex.Unlock()
		return nil
        }
        fmt.Fprintf(os.Stderr, "%s: scanning deployment %s\n", au.Bold(au.Cyan("Info")), name)
        success := false
	c.mutex.Lock()
        if !teamExists {
                c.state[team] = map[string]Deployment{}
        }
        c.mutex.Unlock()

        // NB: a NEW image does not qualify as an UPDATED image
        // don't process all available deployments right away
        imageChanged := false
        var recoverySeconds int64
        recoverySeconds = 0
        _, nameExists := c.state[team][name]
	previousSuccess := true // we only use the negative for tests later
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
		// sometimes successful deployments get stuck in this state
		// skip only if replicas and readyReplicas don't match
		if replicas != readyReplicas {
			fmt.Fprintf(os.Stderr, "%s: skipping - rollout in progress\n", au.Bold(au.Cyan("Info")))
			return nil
		}
		success = true
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

        deployment := Deployment{
                name,
                namespace,
                image,
                lastTimestamp.Unix(),
                success,
                imageChanged,
                recoverySeconds,
        }

        c.mutex.Lock()
        c.state[team][name] = deployment
        debug := c.debug
        c.mutex.Unlock()

        if len(team) > 0 {
                if (debug) {
                        fmt.Fprintf(os.Stderr, "=> Team: %s\n", au.Bold(team))
                        fmt.Fprintf(os.Stderr, "=> Image %s\n", au.Bold(image))
                        fmt.Fprintf(os.Stderr, "=> Replicas %d\n", au.Bold(replicas))
                        fmt.Fprintf(os.Stderr, "=> ReadyReplicas %d\n", au.Bold(readyReplicas))
                        fmt.Fprintf(os.Stderr, "=> LastTransitionTime %s\n", au.Bold(lastTimestamp))
                        fmt.Fprintf(os.Stderr ,"=> Type %s\n", au.Bold(lastType))
                        fmt.Fprintf(os.Stderr, "=> Status %s\n", au.Bold(lastStatus))
                        fmt.Fprintf(os.Stderr, "=> Success %t\n", au.Bold(success))
                        fmt.Fprintf(os.Stderr, "=> Deployment:\n")
                }

                bytes, err := json.Marshal(deployment)
                if err != nil {
                        fmt.Fprintf(os.Stderr, "%s: %s", au.Bold(au.Red("Error")), au.Bold(err))
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
                fmt.Fprintf(os.Stderr, "%s: can't sync deployment %v: %v", au.Bold(au.Red("Error")), key, err)
                c.queue.AddRateLimited(key)
                return
        }

        c.queue.Forget(key)
        runtime.HandleError(err)
        fmt.Fprintf(os.Stderr, "%s: dropping deployment %q from the queue: %v", au.Bold(au.Cyan("Info")), key, err)
}

func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
        defer runtime.HandleCrash()

        defer c.queue.ShutDown()
        fmt.Fprintf(os.Stderr, "%s: starting DORA controller\n", au.Bold(au.Cyan("Info")))

        go c.informer.Run(stopCh)

        if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
                runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
                return
        }

        for i := 0; i < threadiness; i++ {
                go wait.Until(c.runWorker, time.Second, stopCh)
        }

        <-stopCh
        fmt.Fprintf(os.Stderr, "%s: stopping DORA controller\n", au.Bold(au.Cyan("Info")))
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

        // creates the connection
        config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
        if err != nil {
                fmt.Fprintf(os.Stderr, "%s: %s", au.Bold(au.Red("Error")), err)
                return
        }

        // creates the clientset
        clientset, err := kubernetes.NewForConfig(config)
        if err != nil {
                fmt.Fprintf(os.Stderr, "%s: %s", au.Bold(au.Red("Error")), err)
                return
        }

        var mutex = &sync.Mutex{}
        var state = map[string]map[string]Deployment{}

        namespaceListWatcher := cache.NewListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", "", fields.Everything())

        queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

        indexer, informer := cache.NewIndexerInformer(namespaceListWatcher, &appsv1.Deployment{}, 0, cache.ResourceEventHandlerFuncs{
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
