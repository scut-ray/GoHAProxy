package main

import (
	"fmt"
	"os"
	"os/signal"
	_ "syscall"
)

func main() {
	yamlFile := "app.yaml"
	conf, err := NewConfig(yamlFile)
	if err != nil {
		fmt.Printf("Can not load config file %s: %s\n", yamlFile, err)
		os.Exit(1)
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Kill, os.Interrupt)
	bindAddr := fmt.Sprintf("%s:%d", conf.Server.Ip, conf.Server.Port)
	fmt.Println("bind addr ", bindAddr)
	server := NewServer(bindAddr, conf)
	server.Start()
	fmt.Println("start")
	go func() {
		<-quit
		fmt.Println("Recv quit signal")
		server.Shutdown()
	}()
	server.Wait()
	fmt.Println("finish")
}
