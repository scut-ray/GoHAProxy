package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

type Config struct {
	Server struct {
		Ip   string
		Port uint16
	}
	Watcher struct {
		ZkHost    []string
		ZkTimeout uint
	}
}

func NewConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	var conf Config
	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &conf, nil
}
