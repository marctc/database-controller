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
