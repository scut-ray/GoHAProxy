package main

import (
	_ "fmt"
	"log"
	"net"
	"sync"
	"runtime"
)

type ClientMap map[net.Conn] *Client

type Server struct {
	mu               sync.Mutex
	listener         net.Listener
	killChan         chan interface{}
	closedClientChan chan net.Conn
	closed           bool
	serverWg         sync.WaitGroup
	wait             sync.WaitGroup
	stop             sync.Once
	clients          ClientMap
}

func New(laddr string) *Server {
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatalf("Can not listen %s: %s", laddr, err)
		return nil
	}
	s := &Server{}
	s.listener = l
	s.closed = true
	s.clients = make(ClientMap)
	s.killChan = make(chan interface{})
	s.closedClientChan = make(chan net.Conn)
	return s
}

func (this *Server) Start() {
	this.mu.Lock()
	if ! this.closed {
		log.Println("Server is started!")
		return
	}
	this.closed = false
	this.mu.Unlock()
	
	this.wait.Add(1)
	go this.listen()
}

func (this *Server) Shutdown() {
	log.Println("call Shutdown")
	
	this.mu.Lock()
	if this.closed {
		log.Println("Server is closed!")
		return
	}
	this.closed = true
	this.mu.Unlock()
	
	log.Println("Close listener")
	this.listener.Close()
	runtime.Gosched()

	for _, c := range this.clients {
		log.Println("Close ", c.client.RemoteAddr())
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
		if this.closed {
			if err == nil {
				client.Close()
			}
			return
		}
		if err != nil {
			log.Println("error: ", err)
			continue
		}
		log.Println("Accept: ", client.RemoteAddr())
		this.serverWg.Add(1)
		cli := NewClient(client, &this.serverWg)
		this.clients[client] = cli
		go cli.handle()
	}
}
