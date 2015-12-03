package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	_ "syscall"
)

func main() {
	yamlFile := "app.yaml"
	conf, err := NewConfig(yamlFile)
	if err != nil {
		log.Printf("Can not load config file %s: %s\n", yamlFile, err)
		os.Exit(1)
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Kill, os.Interrupt)
	bindAddr := fmt.Sprintf("%s:%d", conf.Server.Ip, conf.Server.Port)
	log.Println("bind addr ", bindAddr)
	server := NewServer(bindAddr, conf)
	if server == nil {
		log.Fatal("Can NOT create server!")
		os.Exit(1)
		return
	}
	watcher := NewWatcher(conf)
	if watcher == nil {
		log.Fatal("Can NOT create watcher!")
		os.Exit(1)
		return
	}
	server.Start()
	watcher.Start()
	log.Println("start")
	go func() {
		<-quit
		log.Println("Recv quit signal")
		server.Shutdown()
		watcher.Shutdown()
	}()
	server.Wait()
	watcher.Wait()
	log.Println("finish")
}
