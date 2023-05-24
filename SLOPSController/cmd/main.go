package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/workqueue"
	klog "k8s.io/klog/v2"
)

var (
	ep_map  = make(map[string][]string)
	version = 1
	port    = ":62000"
)

type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
}

func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller) *Controller {
	return &Controller{
		indexer:  indexer,
		queue:    queue,
		informer: informer,
	}
}

func (c *Controller) ProcessNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)

	err := c.syncToStdOut(key.(string))
	c.handleError(err, key)
	return true
}

func (c *Controller) syncToStdOut(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		fmt.Printf("Endpoint %s does not exist anymore\n", key)
		delete(ep_map, key)
		version += 1
	} else {
		endpoints, ok := obj.(*v1.Endpoints)
		if !ok {
			panic("Could not cast to Endpoint")
		}
		// fmt.Printf("Sync/Add/Update for endpoint %s\n", endpoints.GetName())
		var addresses []string
		for _, ep := range endpoints.Subsets {
			for _, address := range ep.Addresses {
				// fmt.Printf("%v\n", address.IP)
				addresses = append(addresses, address.IP)
			}
		}
		ep_map[endpoints.GetName()] = addresses
		version += 1
	}
	return nil
}

func (c *Controller) handleError(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing endpoint %v:%v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	// report to an external entity that we couldn't process this key even after 5 retries
	runtime.HandleError(err)
	klog.Infof("Dropping endpoint %q out of queue: %v", key, err)
}

func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	klog.Info("Starting endpoint controller")

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info("Stopping endpoint controller")
}

func (c *Controller) runWorker() {
	for c.ProcessNextItem() {
	}
}

func main() {
	var kubeconfig string
	var master string

	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	config, err := rest.InClusterConfig()
	// config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	// create watcher
	epWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "endpoints", v1.NamespaceAll, fields.Everything())

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// bind the workqueue to a cache with the help of an informer
	// thus whenever the cache is update the key is added to the workqueue
	indexer, informer := cache.NewIndexerInformer(epWatcher, &v1.Endpoints{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue
			// thus for deletes we have to use this key function
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	controller := NewController(queue, indexer, informer)

	// start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	http.HandleFunc("/", fetchSvc)
	log.Println("new router") // debug
	// router := mux.NewRouter()
	// router.PathPrefix("/").HandlerFunc(fetchSvc)
	log.Println("listen and serve") // debug
	// log.Fatal(http.ListenAndServe(port, router))
	log.Fatal(http.ListenAndServe(port, nil))
}

type Endpoint struct {
	Svcname string   `json:"Svcname"`
	Ips     []string `json:"Ips"`
}

func fetchSvc(w http.ResponseWriter, r *http.Request) {
	sp := strings.Split(r.URL.String(), "/")[1]
	ep := Endpoint{Svcname: sp, Ips: ep_map[sp]}
	json.NewEncoder(w).Encode(ep)
}
