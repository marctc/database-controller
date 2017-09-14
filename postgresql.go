// vim:set sw=8 ts=8 noet:

package main

import (
	"fmt"
	"log"
	"strings"
	"net/url"
	"database/sql"

	"k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"

	_ "github.com/lib/pq"
)

func (cllr *DatabaseController) handleAddPostgresql(db *Database) {
	var server *PostgreSQLConfig = nil

	for _, candidate := range cllr.DBConfig.PostgreSQL {
		if db.Spec.Class != candidate.Class {
			continue
		}
		server = &candidate
		break
	}

	if (server == nil) {
		cllr.setError(db, "no available providers")
		return
	}

	u, err := url.Parse(server.URL)
	password, _ := u.User.Password()
	dsn := fmt.Sprintf("dbname=%s user=%s password=%s host=%s sslmode=disable",
		strings.TrimLeft(u.Path, "/"), u.User.Username(), password, u.Host)
	dbconn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("%s/%s: database connection failed: %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		cllr.setError(db, fmt.Sprintf("database connection failed: %v", err))
		return;
	}

	defer dbconn.Close()

	dbname := strings.Replace(
			fmt.Sprintf("%s_%s", db.Metadata.Namespace, db.Metadata.Name),
			"-", "_", -1);
	gen_password, err := makepassword()

	if err != nil {
		cllr.setError(db, "failed to generate password")
		log.Printf("%s/%s: failed to generate password: %s",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("CREATE ROLE \"%s\" LOGIN PASSWORD '%s'",
					dbname, gen_password))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create user: %v", err))
		log.Printf("%s/%s: failed to create user: %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("CREATE DATABASE \"%s\" OWNER \"%s\" TEMPLATE \"template0\"", dbname, dbname))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create database: %v", err))
		log.Printf("%s/%s: failed to create database: %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return
	}

	surl := []byte(fmt.Sprintf("postgresql://%s:%s@%s/%s", dbname, gen_password, u.Host, dbname))
	secret, err := cllr.Clientset.Secrets(db.Metadata.Namespace).Get(db.Spec.Secret)
	if err != nil {
		if !errors.IsNotFound(err) {
			cllr.setError(db, fmt.Sprintf("failed to create secret: %v", err))
			log.Printf("%s/%s: failed to create secret: %v\n",
				db.Metadata.Namespace, db.Metadata.Name, err)
			return
		}

		secret = &v1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name: db.Spec.Secret,
				Namespace: db.Metadata.Namespace,
			},
			Data: map[string][]byte {
				"database-url": surl,
			},
		}

		_, err := cllr.Clientset.Secrets(db.Metadata.Namespace).Create(secret)
		if err != nil {
			cllr.setError(db, fmt.Sprintf("failed to create secret: %v", err))
			log.Printf("%s/%s: failed to create secret: %v\n",
				db.Metadata.Namespace, db.Metadata.Name, err)
			return
		}
	} else {
		secret.Data["database-url"] = surl
		_, err := cllr.Clientset.Secrets(db.Metadata.Namespace).Update(secret)
		if err != nil {
			cllr.setError(db, fmt.Sprintf("failed to create secret: %v", err))
			log.Printf("%s/%s: failed to create secret: %v\n",
				db.Metadata.Namespace, db.Metadata.Name, err)
			return
		}
	}

	db.Status.Phase = "done"
	db.Status.Server = server.Name
	cllr.tprclient.Put().Namespace(db.Metadata.Namespace).Resource("databases").Name(db.Metadata.Name).Body(db).Do()
}

func (cllr *DatabaseController) handleDeletePostgresql(db *Database) {
	var server *PostgreSQLConfig

	for _, candidate := range cllr.DBConfig.PostgreSQL {
		if db.Status.Server != candidate.Name {
			continue
		}
		server = &candidate
		break
	}

	if (server == nil) {
		log.Printf("%s/%s: delete failed: server \"%s\" not found in config\n",
			db.Metadata.Namespace, db.Metadata.Name, db.Status.Server)
		return
	}

	u, err := url.Parse(server.URL)
	password, _ := u.User.Password()
	dsn := fmt.Sprintf("dbname=%s user=%s password=%s host=%s sslmode=disable",
		strings.TrimLeft(u.Path, "/"), u.User.Username(), password, u.Host)
	dbconn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("%s/%s: delete failed: database connection failed: %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return;
	}

	defer dbconn.Close()

	dbname := strings.Replace(
			fmt.Sprintf("%s_%s", db.Metadata.Namespace, db.Metadata.Name),
			"-", "_", -1);

	_, err = dbconn.Exec("UPDATE pg_database SET datallowconn=false WHERE datname=$1", dbname)
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Metadata.Namespace, db.Metadata.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1", dbname)
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Metadata.Namespace, db.Metadata.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("DROP DATABASE \"%s\"", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Metadata.Namespace, db.Metadata.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("DROP ROLE \"%s\"", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop user \"%s\": %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return
	}
}
