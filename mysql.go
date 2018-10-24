package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

const mysqlDefaultPort = "3306"

func (cllr *DatabaseController) handleAddMysql(db *Database) {
	var server *MySQLConfig = nil

	for _, candidate := range cllr.DBConfig.MySQL {
		if db.Spec.Class != candidate.Class {
			continue
		}
		server = &candidate
		break
	}

	if server == nil {
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
			db.Namespace, db.Name, err)
		cllr.setError(db, fmt.Sprintf("database connection failed: %v", err))
		return
	}

	defer dbconn.Close()

	dbname := strings.Replace(
		fmt.Sprintf("%s_%s", db.Namespace, db.Name),
		"-", "_", -1)
	gen_password, err := makepassword()

	if err != nil {
		cllr.setError(db, "failed to generate password")
		log.Printf("%s/%s: failed to generate password: %s",
			db.Namespace, db.Name, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("CREATE DATABASE `%s`", dbname))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create database: %v", err))
		log.Printf("%s/%s: failed to create database: %v\n",
			db.Namespace, db.Name, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO `%s`@`%%` IDENTIFIED BY \"%s\"",
		strings.Replace(dbname, "_", "\\_", -1),
		dbname, gen_password))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create user: %v", err))
		log.Printf("%s/%s: failed to create user: %v\n",
			db.Namespace, db.Name, err)
		return
	}

	surl := []byte(fmt.Sprintf("mysql://%s:%s@%s/%s", dbname, gen_password, u.Host, dbname))
	secret, err := cllr.Clientset.Secrets(db.Namespace).Get(db.Spec.Secret, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			cllr.setError(db, fmt.Sprintf("failed to create secret: %v", err))
			log.Printf("%s/%s: failed to create secret: %v\n",
				db.Namespace, db.Name, err)
			return
		}

		port := u.Port()
		if port == "" {
			port = mysqlDefaultPort
		}
		secret = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      db.Spec.Secret,
				Namespace: db.Namespace,
			},
			Data: map[string][]byte{
				"database-url":      surl,
				"database-host":     []byte(u.Hostname()),
				"database-port":     []byte(port),
				"database-name":     []byte(dbname),
				"database-user":     []byte(dbname),
				"database-password": []byte(gen_password),
			},
		}

		_, err := cllr.Clientset.Secrets(db.Namespace).Create(secret)
		if err != nil {
			cllr.setError(db, fmt.Sprintf("failed to create secret: %v", err))
			log.Printf("%s/%s: failed to create secret: %v\n",
				db.Namespace, db.Name, err)
			return
		}
	} else {
		secret.Data["database-url"] = surl
		_, err := cllr.Clientset.Secrets(db.Namespace).Update(secret)
		if err != nil {
			cllr.setError(db, fmt.Sprintf("failed to create secret: %v", err))
			log.Printf("%s/%s: failed to create secret: %v\n",
				db.Namespace, db.Name, err)
			return
		}
	}

	db.Status.Phase = "done"
	db.Status.Server = server.Name
	cllr.client.Put().Namespace(db.Namespace).Resource("databases").Name(db.Name).Body(db).Do()
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

	if server == nil {
		log.Printf("%s/%s: delete failed: server \"%s\" not found in config\n",
			db.Namespace, db.Name, db.Status.Server)
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
			db.Namespace, db.Name, err)
		return
	}

	defer dbconn.Close()

	dbname := fmt.Sprintf("%s_%s", db.Namespace, db.Name)

	_, err = dbconn.Exec(fmt.Sprintf("DROP DATABASE `%s`", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Namespace, db.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("DROP USER `%s`@`%%`", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop user \"%s\": %v\n",
			db.Namespace, db.Name, err)
		return
	}
}
