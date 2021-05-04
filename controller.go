package main

import (
	"log"
	"regexp"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

const (
	nthreads = 1
)

type DatabaseController struct {
	Clientset *kubernetes.Clientset
	DBConfig  *DBConfig
	config    *rest.Config
	client    *rest.RESTClient
	indexer   cache.Indexer
	queue     workqueue.RateLimitingInterface
	informer  cache.Controller
}

type QueueItem struct {
	event string
	item  *Database
}

func (cllr *DatabaseController) setError(db *Database, err string) {
	db.Status.Phase = "error"
	db.Status.Error = err
	cllr.client.Put().Namespace(db.Namespace).Resource("databases").Name(db.Name).Body(db).Do()
}

func (cllr *DatabaseController) handleAdd(obj interface{}) {
	db := obj.(*Database)

	/* Skip if we've already provisioned this database */
	if db.Status.Phase == "error" || db.Status.Phase == "done" {
		return
	}

	if db.Spec.Secret == "" {
		cllr.setError(db, "secretName not found in spec")
		return
	}

	if db.Spec.Class == "" {
		db.Spec.Class = "default"
	}

	matched, err := regexp.MatchString("[a-z][a-z0-9-]+", db.Namespace)
	if !matched || err != nil {
		cllr.setError(db, "invalid namespace name")
		return
	}

	matched, err = regexp.MatchString("[a-z][a-z0-9-]+", db.Name)
	if !matched || err != nil {
		cllr.setError(db, "invalid name: must only contain a-z, 0-9 and '-'")
		return
	}

	log.Printf("%s/%s: provisioning new database\n",
		db.Namespace,
		db.Name)

	switch db.Spec.Type {
	case "mysql":
		cllr.handleAddMysql(db)
		break

	case "postgresql":
		cllr.handleAddPostgresql(db)
		break

	default:
		log.Printf("%s/%s: provisioning failed: unrecognised database type \"%s\"\n",
			db.Namespace,
			db.Name,
			db.Spec.Type)
		cllr.setError(db, "unrecognised database type")
		return
	}
}

func (cllr *DatabaseController) handleDelete(obj interface{}) {
	db := obj.(*Database)

	/* Skip if this database has not been provisioned */
	if db.Status.Phase != "done" {
		return
	}

	matched, err := regexp.MatchString("[a-z][a-z0-9-]+", db.Namespace)
	if !matched || err != nil {
		log.Printf("%s/%s: delete ignored: invalid namespace",
			db.Namespace,
			db.Name)
		return
	}

	matched, err = regexp.MatchString("[a-z][a-z0-9-]+", db.Name)
	if !matched || err != nil {
		log.Printf("%s/%s: delete ignored: invalid name",
			db.Namespace,
			db.Name)
		return
	}

	log.Printf("%s/%s: dropping database\n",
		db.Namespace,
		db.Name)

	switch db.Spec.Type {
	case "mysql":
		cllr.handleDeleteMysql(db)
		break

	case "postgresql":
		cllr.handleDeletePostgresql(db)
		break

	default:
		log.Printf("%s/%s: delete ignored: unrecognised database type \"%s\"\n",
			db.Namespace,
			db.Name,
			db.Spec.Type)
		return
	}
}

func createController(kubeconfig string, dbconfig *DBConfig) *DatabaseController {
	var err error
	cllr := &DatabaseController{
		DBConfig: dbconfig,
	}

	if kubeconfig != "" {
		cllr.config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cllr.config, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err)
	}

	cllr.Clientset, err = kubernetes.NewForConfig(cllr.config)
	if err != nil {
		panic(err)
	}

	groupversion := schema.GroupVersion{
		Group:   "kubejam.io",
		Version: "v1",
	}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(groupversion,
		&Database{},
		&DatabaseList{},
	)
	metav1.AddToGroupVersion(scheme, groupversion)

	cllr.config.GroupVersion = &groupversion
	cllr.config.APIPath = "/apis"
	cllr.config.ContentType = runtime.ContentTypeJSON
	cllr.config.NegotiatedSerializer = serializer.DirectCodecFactory{
		CodecFactory: serializer.NewCodecFactory(scheme),
	}

	cllr.client, err = rest.RESTClientFor(cllr.config)
	if err != nil {
		panic(err)
	}

	return cllr
}

func (cllr *DatabaseController) processNextItem() bool {
	qitem, quit := cllr.queue.Get()
	if quit {
		return false
	}

	item := qitem.(*QueueItem)

	defer cllr.queue.Done(qitem)

	switch item.event {
	case "add":
		cllr.handleAdd(item.item)
	case "delete":
		cllr.handleDelete(item.item)
	}

	return true
}

func (cllr *DatabaseController) runWorker() {
	for cllr.processNextItem() {
	}
}

func (cllr *DatabaseController) run() {
	watchlist := cache.NewListWatchFromClient(cllr.client,
		"databases", apiv1.NamespaceAll, fields.Everything())

	cllr.indexer, cllr.informer = cache.NewIndexerInformer(
		watchlist,
		&Database{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cllr.queue.Add(&QueueItem{"add", obj.(*Database)})
			},
			DeleteFunc: func(obj interface{}) {
				cllr.queue.Add(&QueueItem{"delete", obj.(*Database)})
			},
			UpdateFunc: func(oldobj, obj interface{}) {
				/* no update handling right now */
				//cllr.queue.Add(&QueueItem{"update", obj.(Database)})
			},
		},
		cache.Indexers{},
	)

	cllr.queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	log.Println("running")

	defer cllr.queue.ShutDown()
	stop := make(chan struct{})
	defer close(stop)
	go cllr.informer.Run(stop)

	if !cache.WaitForCacheSync(stop, cllr.informer.HasSynced) {
		log.Println("Timed out waiting for caches to sync")
		return
	}

	for i := 0; i < nthreads; i++ {
		go wait.Until(cllr.runWorker, time.Second, stop)
	}

	<-stop
	log.Println("shutting down")
}
