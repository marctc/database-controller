package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type MySQLConfig struct {
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	Class string `yaml:"class"`
}

type PostgreSQLConfig struct {
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	Class string `yaml:"class"`
}

type DBConfig struct {
	MySQL      []MySQLConfig      `yaml:"mysql"`
	PostgreSQL []PostgreSQLConfig `yaml:"postgresql"`
}

func read_config(filename string) (*DBConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	dbconfig := new(DBConfig)
	err = yaml.Unmarshal([]byte(data), dbconfig)
	if err != nil {
		return nil, err
	}

	for _, mysql := range dbconfig.MySQL {
		if mysql.Name == "" {
			return nil, errors.New("MySQL server missing 'name'")
		}
		if mysql.URL == "" {
			return nil, errors.New(fmt.Sprintf(`MySQL server "%s" missing URL`,
				mysql.Name))
		}
		if mysql.Class == "" {
			mysql.Class = "default"
			log.Printf(`note: MySQL server "%s" missing class; set to "default"`,
				mysql.Name)
		}
	}

	return dbconfig, nil
}
