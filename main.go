package main

import (
	"fmt"
	"os"
	"os/signal"
	_ "syscall"
)

func main() {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Kill, os.Interrupt)
	server := New(":9999")
	server.Start()
	fmt.Println("start")
	go func() {
		<- quit
		fmt.Println("Recv quit signal")
		server.Shutdown()
	}()
	server.Wait()
	fmt.Println("finish")
}
