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

	_ "github.com/go-sql-driver/mysql"
)

func (cllr *DatabaseController) handleAddMysql(db *Database) {
	var server *MySQLConfig = nil

	for _, candidate := range cllr.DBConfig.MySQL {
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
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/",
		u.User.Username(),
		password,
		u.Host)
	dbconn, err := sql.Open("mysql", dsn)
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

	_, err = dbconn.Exec(fmt.Sprintf("CREATE DATABASE `%s`", dbname))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create database: %v", err))
		log.Printf("%s/%s: failed to create database: %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO `%s`@`%%` IDENTIFIED BY \"%s\"",
					strings.Replace(dbname, "_", "\\_", -1),
					dbname, gen_password))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create user: %v", err))
		log.Printf("%s/%s: failed to create user: %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return
	}

	surl := []byte(fmt.Sprintf("mysql://%s:%s@%s/%s", dbname, gen_password, u.Host, dbname))
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

func (cllr *DatabaseController) handleDeleteMysql(db *Database) {
	var server *MySQLConfig

	for _, candidate := range cllr.DBConfig.MySQL {
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
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/",
		u.User.Username(),
		password,
		u.Host)
	dbconn, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("%s/%s: delete failed: database connection failed: %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return;
	}

	defer dbconn.Close()

	dbname := fmt.Sprintf("%s_%s", db.Metadata.Namespace, db.Metadata.Name)

	_, err = dbconn.Exec(fmt.Sprintf("DROP DATABASE `%s`", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Metadata.Namespace, db.Metadata.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("DROP USER `%s`@`%%`", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop user \"%s\": %v\n",
			db.Metadata.Namespace, db.Metadata.Name, err)
		return
	}
}
