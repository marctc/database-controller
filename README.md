Kubernetes database controller
==============================

This is a database controller for Kubernetes.  It allows users to provision
on-demand databases for their applications by creating "Database" resources in
Kubernetes, without requiring access to the database server.  This is useful
for staging sites, CI builds, review apps and other situations where new
databases need to be provisioned frequently.

The controller works by watching for creation of a new custom resouce type
called `Database`, and running `CREATE USER` / `CREATE DATABASE` statements on
the configure database servers.  Both MySQL and PostgreSQL databases are
supported, and multiple database classes can be configured for each type (e.g.
"staging" and "review").  When the Database resource is deleted in Kubernetes,
the corresponding database and user are dropped.

**WARNING**: This is **experimental** software and **drops databases by
design**.  We strongly recommend keeping regular backups, and not running it on
production databases.

Cluster setup
-------------

For the provisioner to work, you must create the Database custom resource type
on your Kubernetes cluster:

```
$ kubectl apply -f https://raw.githubusercontent.com/torchbox/k8s-database-controller/master/crd.yaml
```

(For Kubernetes 1.6 or earlier, use `tpr.yaml` instead; this creates a
ThirdPartyResource which you will need to
[migrate](https://kubernetes.io/docs/tasks/access-kubernetes-api/migrate-third-party-resource/)
when upgrading to 1.8 or later.)

In addition, if your cluster uses authorization (e.g. RBAC) you should give
users permission to create and delete Database resources in the torchbox.com/v1
API group.

Installation
------------

The easiest way to run the provisioner is as a pod inside Kubernetes.  See the
example `deployment.yaml` for an example.  A configuration file is required; an
example is provided in `config.yaml.example`.

Create the configuration in `config.yaml` and apply it to the cluster:

```
$ kubectl create namespace database-controller
$ kubectl -n database-controller create secret generic config --from-file=config.yaml=config.yaml
```

Then deploy the controller:

```
$ kubectl apply -f https://raw.githubusercontent.com/torchbox/k8s-database-controller/master/deployment.yaml
```

Usage
-----

To provision a database, create a Database resource that looks like this:

```
apiVersion: torchbox.com/v1
kind: Database
metadata:
  namespace: default
  name: mydb
spec:
  type: postgresql
  class: default
  secretName: mydb-secret
```

`type` is the type of database to create; currently, `postgresql` and `mysql`
are the supported types.  `class` is the class of database to create.  If not
specified, the default is `default`.  The specified class must be configured in
`config.yaml`, or provisioning will fail.

`secretName` is the name of a secret that will be created to store the database
URL.  The created secret will look like this:

```
apiVersion: v1
kind: Secret
metadata:
  namespace: default
  name: mydb-secret
type: Opaque
data:
  database-url: cG9zdGdyZXNxbDovL2RlZmF1bHRfbXlkYjplOFFFTGZUWkpkdW0wVHJVQHBvc3RncmVzLmRhdGFiYXNlLnN2Yy9kZWZhdWx0X215ZGI=
```

```
$ echo 'cG9zdGdyZXNxbDovL2RlZmF1bHRfbXlkYjplOFFFTGZUWkpkdW0wVHJVQHBvc3RncmVzLmRhdGFiYXNlLnN2Yy9kZWZhdWx0X215ZGI=' | base64 -d
postgresql://default_mydb:e8QELfTZJdum0TrU@postgres.database.svc/default_mydb
```

To delete a provisioned database, delete the Database resource:

```
$ kubectl delete database mydb
```

Notes / bugs
------------

Compared to the persistent volume provisioner, this controller is very
primitive.

It does not use finalizers to ensure consistent database deletion, so it's
possible for a Database record to be deleted while the corresponding database
isn't dropped, if an error occurs or the controller misses the deletion event.

It does not use a lease to serialize operations, so only one copy can be
running at once.

There is no permission checking on the created Secret, so a user with access to
create Database resources in a namespace can overwrite existing Secrets in that
namespace.  (This is not considered a serious problem since Kubernetes access
control is usually done on the namespace level anyway.)

Contact and support
-------------------

The database controller was written by Felicity Tarnell <ft@le-Fay.ORG> and is
copyright (c) 2016-2017 Torchbox Ltd.

For support, bug reports or feature requests, please open an issue on Github.
