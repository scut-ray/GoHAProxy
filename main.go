package main

import (
	"fmt"
)

func main() {
	server := New(":9999")
	server.Start()
	fmt.Println("start")
	server.Wait()
	fmt.Println("finish")
}
