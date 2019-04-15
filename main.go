package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	"k8s.io/api/core/v1"
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

func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, clientset kubernetes.Interface, mutex *sync.Mutex, state map[string][]Deployment) *Controller {
	return &Controller{
		informer:    informer,
		indexer:     indexer,
		queue:       queue,
		clientset:   clientset,
		mutex:       mutex,
		state:       state,
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
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		fmt.Printf("%s: fetching object with key %s from store failed with %v", au.Bold(au.Red("Error")), key, err)
		return err
	}

	name := obj.(*appsv1.Deployment).GetName()
	team := obj.(*appsv1.Deployment).ObjectMeta.Annotations[fmt.Sprintf("%s/%s", annotationPrefix, annotationName)]
	image := obj.(*appsv1.Deployment).Spec.Template.Spec.Containers[0].Image
	replicas := obj.(*appsv1.Deployment).Status.Replicas
	readyReplicas := obj.(*appsv1.Deployment).Status.ReadyReplicas
	conditions := obj.(*appsv1.Deployment).Status.Conditions
	lastTransitionTime := conditions[len(conditions)-1].LastTransitionTime
	typeValue := conditions[len(conditions)-1].Type

	if !exists {
		fmt.Printf("%s: deployment %s deleted\n", au.Bold(au.Cyan("Info")), key)
		c.mutex.Lock()
		// delete(MAP, KEY)
		// TODO
		c.mutex.Unlock()
	} else {
		fmt.Printf("%s: scanning deployment %s\n", au.Bold(au.Cyan("Info")), name)

		if len(team) > 0 {
			c.mutex.Lock()
			fmt.Printf("=> Team: %s\n", au.Bold(team))
			fmt.Printf("=> Image %s\n", au.Bold(image))
			fmt.Printf("=> Replicas %d\n", au.Bold(replicas))
			fmt.Printf("=> ReadyReplicas %d\n", au.Bold(readyReplicas))
			fmt.Printf("=> LastTransitionTime %s\n", au.Bold(lastTransitionTime))
			fmt.Printf("=> Type %s\n", au.Bold(typeValue))
			c.mutex.Unlock()
		}
	}
	if c.queue.Len() == 0 {
		c.mutex.Lock()
		fmt.Printf("%s: DORA state: %v\n", au.Bold(au.Cyan("Info")), c.state)
		c.mutex.Unlock()
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
		fmt.Printf("%s: can't sync deployment %v: %v", au.Bold(au.Red("Error")), key, err)
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	runtime.HandleError(err)
	fmt.Printf("%s: dropping deployment %q out of the queue: %v", au.Bold(au.Cyan("Info")), key, err)
}

func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	fmt.Printf("%s: starting DORA controller\n", au.Bold(au.Cyan("Info")))

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	fmt.Printf("%s: stopping DORA controller\n", au.Bold(au.Cyan("Info")))
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func main() {
	var kubeconfig string
	var master string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	// creates the connection
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		fmt.Printf("%s: %s", au.Bold(au.Red("Error")), err)
		return
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("%s: %s", au.Bold(au.Red("Error")), err)
		return
	}

	var mutex = &sync.Mutex{}
	var state = map[string][]Deployment{}

	namespaceListWatcher := cache.NewListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", "", fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(namespaceListWatcher, &v1.Namespace{}, 0, cache.ResourceEventHandlerFuncs{
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

	controller := NewController(queue, indexer, informer, clientset, mutex, state)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	select {}
}
