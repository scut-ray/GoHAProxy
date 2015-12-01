package main

import (
	"github.com/samuel/go-zookeeper/zk"
	"log"
	"time"
)

type WatcherConfig struct {
}

type Watcher struct {
	conf   *Config
	zkConn *zk.Conn
}

func NewWatcher(conf *Config) *Watcher {
	conn, _, err := zk.Connect(conf.Watcher.ZkHost, time.Duration(conf.Watcher.ZkTimeout) * time.Millisecond)
	if err != nil {
		log.Fatal("Can not connect zk server ", conf.Watcher.ZkHost, err)
		return nil
	}
	return &Watcher{conf: conf, zkConn: conn}
}

func (*Watcher) Start() {

}

func (this *Watcher) Shutdown() {
	this.zkConn.Close()
}
