package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

type Config struct {
	Zk struct {
		ZkHost    []string
		ZkTimeout uint
	}
	ZkPath struct {
		Root   string
		Master string
		Server string
		Judge  string
		Proxy  string
	}
	Server struct {
		Ip   string
		Port uint16
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
