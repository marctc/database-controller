// vim:set sw=8 ts=8 noet:

package main

import (
	"log"
	"regexp"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/cache"
)

type DatabaseController struct {
	Clientset	*kubernetes.Clientset
	DBConfig	*DBConfig
	tprconfig	*rest.Config
	tprclient	*rest.RESTClient
}

func (cllr *DatabaseController) setError(db *Database, err string) {
	db.Status.Phase = "error"
	db.Status.Error = err
	cllr.tprclient.Put().Namespace(db.Metadata.Namespace).Resource("databases").Name(db.Metadata.Name).Body(db).Do()
}

func (cllr *DatabaseController) handleAdd(obj interface{}) {
	db := obj.(*Database)

	/* Skip if we've already provisioned this database */
	if (db.Status.Phase == "error" || db.Status.Phase == "done") {
		return;
	}

	if (db.Spec.Secret == "") {
		cllr.setError(db, "secretName not found in spec")
		return
	}

	if (db.Spec.Class == "") {
		db.Spec.Class = "default"
	}

	matched, err := regexp.MatchString("[a-z][a-z0-9-]+", db.Metadata.Namespace)
	if !matched || err != nil {
		cllr.setError(db, "invalid namespace name")
		return;
	}

	matched, err = regexp.MatchString("[a-z][a-z0-9-]+", db.Metadata.Name)
	if !matched || err != nil {
		cllr.setError(db, "invalid name: must only contain a-z, 0-9 and '-'")
		return;
	}

	log.Printf("%s/%s: provisioning new database\n",
		db.Metadata.Namespace,
		db.Metadata.Name)

	switch (db.Spec.Type) {
	case "mysql":
		cllr.handleAddMysql(db)
		break

	case "postgresql":
		cllr.handleAddPostgresql(db)
		break

	default:
		log.Printf("%s/%s: provisioning failed: unrecognised database type \"%s\"\n",
			db.Metadata.Namespace,
			db.Metadata.Name,
			db.Spec.Type)
		cllr.setError(db, "unrecognised database type")
		return
	}
}

func (cllr *DatabaseController) handleDelete(obj interface{}) {
	db := obj.(*Database)

	/* Skip if this database has not been provisioned */
	if db.Status.Phase != "done" {
		return;
	}

	matched, err := regexp.MatchString("[a-z][a-z0-9-]+", db.Metadata.Namespace)
	if !matched || err != nil {
		log.Printf("%s/%s: delete ignored: invalid namespace",
			db.Metadata.Namespace,
			db.Metadata.Name)
		return;
	}

	matched, err = regexp.MatchString("[a-z][a-z0-9-]+", db.Metadata.Name)
	if !matched || err != nil {
		log.Printf("%s/%s: delete ignored: invalid name",
			db.Metadata.Namespace,
			db.Metadata.Name)
		return;
	}

	log.Printf("%s/%s: dropping database\n",
		db.Metadata.Namespace,
		db.Metadata.Name)

	switch (db.Spec.Type) {
	case "mysql":
		cllr.handleDeleteMysql(db)
		break

	case "postgresql":
		cllr.handleDeletePostgresql(db)
		break

	default:
		log.Printf("%s/%s: delete ignored: unrecognised database type \"%s\"\n",
			db.Metadata.Namespace,
			db.Metadata.Name,
			db.Spec.Type)
		return
	}
}

func (cllr *DatabaseController) handleUpdate(oldobj, newobj interface{}) {
}

func createController(kubeconfig string, dbconfig *DBConfig) *DatabaseController {
	var err error
	cllr := &DatabaseController{
		DBConfig: dbconfig,
	}

	if kubeconfig != "" {
		cllr.tprconfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cllr.tprconfig, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err)
	}

	cllr.Clientset, err = kubernetes.NewForConfig(cllr.tprconfig)
	if err != nil {
		panic(err)
	}

	groupversion := unversioned.GroupVersion{
		Group:   "torchbox.com",
		Version: "v1",
	}

	cllr.tprconfig.GroupVersion = &groupversion
	cllr.tprconfig.APIPath = "/apis"
	cllr.tprconfig.ContentType = runtime.ContentTypeJSON
	cllr.tprconfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	schemeBuilder := runtime.NewSchemeBuilder(
		func(scheme *runtime.Scheme) error {
			scheme.AddKnownTypes(
				groupversion,
				&Database{},
				&DatabaseList{},
				&api.ListOptions{},
				&api.DeleteOptions{},
			)
			return nil
		})
	schemeBuilder.AddToScheme(api.Scheme)

	cllr.tprclient, err = rest.RESTClientFor(cllr.tprconfig)
	if err != nil {
		panic(err)
	}

	return cllr
}

func (cllr *DatabaseController) run() {
	_, err := cllr.Clientset.Extensions().ThirdPartyResources().Get("database.torchbox.com")
	if err != nil {
		if !errors.IsNotFound(err) {
			panic(err)
		}

		tpr := &v1beta1.ThirdPartyResource{
			ObjectMeta: v1.ObjectMeta{
				Name: "database.torchbox.com",
			},
			Versions: []v1beta1.APIVersion{
				{Name: "v1"},
			},
			Description: "A database resource",
		}

		_, err := cllr.Clientset.Extensions().ThirdPartyResources().Create(tpr)
		if err != nil {
			panic(err)
		}
	}

	watchlist := cache.NewListWatchFromClient(cllr.tprclient,
			"databases", api.NamespaceAll, fields.Everything())

	_, controller := cache.NewInformer(
		watchlist,
		&Database{},
		time.Second * 0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cllr.handleAdd(obj)
			},
			DeleteFunc: func(obj interface{}) {
				cllr.handleDelete(obj)
			},
			UpdateFunc: func(oldobj, obj interface{}) {
				cllr.handleUpdate(oldobj, obj)
			},
		},
	)

	log.Println("running")
	go controller.Run(wait.NeverStop)
	for {
		time.Sleep(time.Second)
	}
}

