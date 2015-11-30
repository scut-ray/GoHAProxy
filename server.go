package main

import (
	_ "fmt"
	"sync"
	"net"
	"log"
)

type Server struct {
	mu sync.Mutex
	listener net.Listener
	kill chan interface{}
	closed bool
	wait sync.WaitGroup
	stop sync.Once
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
	this.wait.Done()
}

func (this *Server) Wait() {
	this.wait.Wait()
}

func (this *Server) listen() {
	for {
		client, err := this.listener.Accept()
		if err != nil {
			log.Println("error: ", err)
			continue
		}
		log.Println("Accept: ", client.RemoteAddr())
		go this.handleClient(client)
	}
}

func (this *Server) handleClient(client net.Conn) {
	remote, err := net.Dial("tcp", "116.31.122.34:80")
	if err != nil {
		log.Println("Can not connect: ", err)
		client.Close()
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go this.pipe(wg, client, remote)
	go this.pipe(wg, remote, client)
	wg.Wait()
	client.Close()
	remote.Close()
}

func (this *Server) pipe (wg sync.WaitGroup, r net.Conn, w net.Conn) {
	defer wg.Done()
	buf := make([]byte,1024)
	for {
		total, err := r.Read(buf)
		if err != nil {
			log.Println("Read error: ", err, r.RemoteAddr())
			r.Close()
			w.Close()
			return
		}
		var writed int
		for total > writed {
			n2, err := w.Write(buf[writed:total])
			if err != nil {
				log.Println("Write error: ", err, w.RemoteAddr())
				w.Close()
				r.Close()
				return
			}
			writed += n2
		}
	}
}