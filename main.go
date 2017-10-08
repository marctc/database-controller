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
	"flag"
	"log"
	"os"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kube config. Only required if out-of-cluster.")
	configfile := flag.String("config", "config.yaml", "Path to YAML configuration file.")
	flag.Parse()

	dbconfig, err := read_config(*configfile)
	if err != nil {
		log.Println("failed to read configuration:", err)
		os.Exit(1)
	}

	controller := createController(*kubeconfig, dbconfig)
	controller.run()
}
