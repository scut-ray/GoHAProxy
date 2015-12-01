package main

import (
	"log"
	"net"
	"sync"
)

type Client struct {
	client   net.Conn
	remote   net.Conn
	closed   bool
	serverWg *sync.WaitGroup
	clientWg sync.WaitGroup
}

func NewClient(client net.Conn, serverWg *sync.WaitGroup) *Client {
	//ch := make(chan int)
	return &Client{client: client, serverWg: serverWg}
}

func (this *Client) handle() {
	remote, err := net.Dial("tcp", "116.31.122.34:80")
	if err != nil {
		log.Println("Can not connect: ", err)
		this.client.Close()
		//this.closedClientChan <- this.client
		return
	}
	this.remote = remote
	this.clientWg.Add(2)
	go this.pipe(this.client, remote)
	go this.pipe(remote, this.client)
	this.clientWg.Wait()
	this.client.Close()
	remote.Close()
	this.serverWg.Done()
	//this.closedClientChan <- this.client
}

func (this *Client) pipe(r net.Conn, w net.Conn) {
	defer this.clientWg.Done()
	buf := make([]byte, 1024)
	for {
		total, err := r.Read(buf)
		if this.closed {
			return
		}
		if err != nil {
			log.Println("Read error: ", err, r.RemoteAddr())
			this.closed = true
			r.Close()
			w.Close()
			return
		}
		var writed int
		for total > writed {
			n2, err := w.Write(buf[writed:total])
			if this.closed {
				return
			}
			if err != nil {
				log.Println("Write error: ", err, w.RemoteAddr())
				this.closed = true
				w.Close()
				r.Close()
				return
			}
			writed += n2
		}
	}
}
