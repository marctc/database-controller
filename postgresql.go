/* vim:set sw=8 ts=8 noet:
 *
 * Copyright (c) 2017 Torchbox Ltd.
 *
 * Permission is granted to anyone to use this software for any purpose,
 * including commercial applications, and to alter it and redistribute it
 * freely. This software is provided 'as-is', without any express or implied
 * warranty.
 */

package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"

	_ "github.com/lib/pq"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
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

	if server == nil {
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

	_, err = dbconn.Exec(fmt.Sprintf("CREATE ROLE \"%s\" LOGIN PASSWORD '%s'",
		dbname, gen_password))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create user: %v", err))
		log.Printf("%s/%s: failed to create user: %v\n",
			db.Namespace, db.Name, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("CREATE DATABASE \"%s\" TEMPLATE \"template0\"", dbname))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create database: %v", err))
		log.Printf("%s/%s: failed to create database: %v\n",
			db.Namespace, db.Name, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("GRANT ALL ON DATABASE \"%s\" TO \"%s\"", dbname, dbname))
	if err != nil {
		cllr.setError(db, fmt.Sprintf("failed to create database: %v", err))
		log.Printf("%s/%s: failed to grant privileges: %v\n",
			db.Namespace, db.Name, err)
		return
	}

	surl := []byte(fmt.Sprintf("postgresql://%s:%s@%s/%s", dbname, gen_password, u.Host, dbname))
	secret, err := cllr.Clientset.Secrets(db.Namespace).Get(db.Spec.Secret, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			cllr.setError(db, fmt.Sprintf("failed to create secret: %v", err))
			log.Printf("%s/%s: failed to create secret: %v\n",
				db.Namespace, db.Name, err)
			return
		}

		secret = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      db.Spec.Secret,
				Namespace: db.Namespace,
			},
			Data: map[string][]byte{
				"database-url": surl,
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

func (cllr *DatabaseController) handleDeletePostgresql(db *Database) {
	var server *PostgreSQLConfig

	for _, candidate := range cllr.DBConfig.PostgreSQL {
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
	dsn := fmt.Sprintf("dbname=%s user=%s password=%s host=%s sslmode=disable",
		strings.TrimLeft(u.Path, "/"), u.User.Username(), password, u.Host)
	dbconn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("%s/%s: delete failed: database connection failed: %v\n",
			db.Namespace, db.Name, err)
		return
	}

	defer dbconn.Close()

	dbname := strings.Replace(
		fmt.Sprintf("%s_%s", db.Namespace, db.Name),
		"-", "_", -1)

	_, err = dbconn.Exec("UPDATE pg_database SET datallowconn=false WHERE datname=$1", dbname)
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Namespace, db.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1", dbname)
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Namespace, db.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("DROP DATABASE \"%s\"", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Namespace, db.Name, dbname, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("DROP ROLE \"%s\"", dbname))
	if err != nil {
		log.Printf("%s/%s: failed to drop user \"%s\": %v\n",
			db.Namespace, db.Name, err)
		return
	}
}
