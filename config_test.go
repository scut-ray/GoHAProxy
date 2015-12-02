package main

import (
	"gopkg.in/yaml.v2"
	"testing"
)

func TestConfig(t *testing.T) {
	conf, err := NewConfig("app.yaml")
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log("conf is ", conf)
}

func TestConfig2(t *testing.T) {
	var conf Config
	conf.Server.Ip = "127.0.0.1"
	conf.Server.Port = 3456
	conf.Zk.ZkHost = []string{"a", "b"}
	data, err := yaml.Marshal(&conf)
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log("data is \n", string(data))
}
