package main

import (
	"net"
	"sync"
	"log"
)

type Client struct {
	client net.Conn
	remote net.Conn
	closed bool
	serverWg *sync.WaitGroup
	clientWg sync.WaitGroup
	clientChan chan int
}

func NewClient(client net.Conn, serverWg *sync.WaitGroup) *Client {
	ch := make(chan int)
	return &Client{client:client, clientChan:ch, serverWg:serverWg}
}

func (this *Client) handle() {
	remote, err := net.Dial("tcp", "116.31.122.34:80")
	if err != nil {
		log.Println("Can not connect: ", err)
		this.client.Close()
		return
	}
	this.remote = remote
	this.clientWg.Add(2)
	go this.pipe(this.client, remote)
	go this.pipe(remote, this.client)
	this.clientWg.Wait()
	this.client.Close()
	remote.Close()
	close(this.clientChan)
	this.serverWg.Done()
}

func (this *Client) pipe(r net.Conn, w net.Conn) {
	defer this.clientWg.Done()
	buf := make([]byte, 1024)
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