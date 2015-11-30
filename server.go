package main

import (
	_ "fmt"
	"log"
	"net"
	"sync"
)

type ClientMap map[net.Conn] Client

type Server struct {
	mu       sync.Mutex
	listener net.Listener
	kill     chan interface{}
	closed   bool
	serverWg sync.WaitGroup
	wait     sync.WaitGroup
	stop     sync.Once
	clients  ClientMap
}

func New(laddr string) *Server {
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatalf("Can not listen %s: %s", laddr, err)
		return nil
	}
	s := &Server{}
	s.listener = l
	s.kill = make(chan interface{})
	return s
}

func (this *Server) Start() {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.closed {
		log.Println("Server is started!")
		return
	}
	this.closed = true
	this.wait.Add(1)
	go this.listen()
}

func (this *Server) Shutdown() {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.closed {
		log.Println("Server is started!")
		return
	}
	this.listener.Close()
	this.closed = true
	
	for _, c := range this.clients {
		c.client.Close()
		c.remote.Close()
	}
	this.serverWg.Wait()
	
	this.wait.Done()
}

func (this *Server) Wait() {
	this.wait.Wait()
}

func (this *Server) boardcast(msg int) {
	for _, c := range this.clients {
		c.client.Close()
		c.remote.Close()
	}
}

func (this *Server) listen() {
	for {
		client, err := this.listener.Accept()
		if err != nil {
			log.Println("error: ", err)
			continue
		}
		log.Println("Accept: ", client.RemoteAddr())
		this.serverWg.Add(1)
		cli := NewClient(client, &this.serverWg)
		go cli.handle()
	}
}


