package main

import (
	"github.com/samuel/go-zookeeper/zk"
	"log"
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type zkBindData struct {
	conn         *zk.Conn
	conf         *Config
	wg           sync.WaitGroup
	quiting      chan interface{}
	master       string
	serverList   []string
	judge        string
	failList     []string
	minFailCount string
}

func newZkBindData(conn *zk.Conn, conf *Config) *zkBindData {
	qch := make(chan interface{})
	bind := &zkBindData{conn: conn, conf: conf, quiting: qch}
	bind.bindAll()
	runtime.Gosched()
	return bind
}

func (this *zkBindData) bindAll() {
	go this.bindKey(this.conf.ZkPath.Master, &this.master)
	go this.bindKey(this.conf.ZkPath.Judge, &this.judge)
	go this.bindKey(this.conf.ZkPath.Master, &this.master)
	go this.bindKey(this.conf.ZkPath.MinFail, &this.minFailCount)

	go this.bindList(this.conf.ZkPath.Server, &this.serverList)
	go this.bindList(this.conf.ZkPath.Fail, &this.failList)
}

func (this *zkBindData) shutdown() {
	close(this.quiting)
	this.wg.Wait()
}

func (this *zkBindData) bindKey(path string, data *string) {
	this.wg.Add(1)
	defer this.wg.Done()
	for {
		d, _, evt, err := this.conn.GetW(path)
		if err != nil {
			select {
			case <-this.quiting:
				// 如果已经关闭就退出循环
				return
			case <-time.After(3 * time.Second):
				// 否则等待3秒重试
			}
			log.Printf("Get %s zk data error: %s\n", path, err)
			continue
		}
		*data = string(d)
		select {
		case <-this.quiting:
			// 如果已经关闭就退出循环
			return
		case <-evt:
			// 有新的值，需要更新
			continue
		}
	}
}

func (this *zkBindData) bindList(path string, data *[]string) {
	this.wg.Add(1)
	defer this.wg.Done()
	for {
		d, _, evt, err := this.conn.ChildrenW(path)
		if err != nil {
			select {
			case <-this.quiting:
				// 如果已经关闭就退出循环
				return
			case <-time.After(3 * time.Second):
				// 否则等待3秒重试
			}
			log.Printf("List %s zk data error: %s\n", path, err)
			continue
		}
		*data = []string(d)
		select {
		case <-this.quiting:
			// 如果已经关闭就退出循环
			return
		case <-evt:
			// 有新的值，需要更新
			continue
		}
	}
}

type Watcher struct {
	conf     *Config
	zkConn   *zk.Conn
	bindData *zkBindData
	quiting  chan interface{}
	judgeId  string
}

func NewWatcher(conf *Config) *Watcher {
	conn, _, err := zk.Connect(conf.Zk.ZkHost, time.Duration(conf.Zk.ZkTimeout)*time.Millisecond)
	if err != nil {
		log.Fatal("Can not connect zk server ", conf.Zk.ZkHost, err)
		return nil
	}
	bind := newZkBindData(conn, conf)
	quiting := make(chan interface{})
	return &Watcher{conf: conf, zkConn: conn, bindData: bind, quiting: quiting}
}

func (this *Watcher) Start() {
	if this.zkConn == nil {
		log.Println("zkConn is null, Can NOT start watcher !")
		return
	}

}

func (this *Watcher) watchMaster() {
	var failCount int
	for {
		m := this.bindData.master
		minFailCount, cerr := strconv.Atoi(this.bindData.minFailCount)
		if cerr != nil {
			log.Println("Can not convert minFailCount: ", minFailCount)
			minFailCount = 100
		}
		if m == "" {
			this.fightForJudge()
			failCount = 0
			continue
		}
		conn, err := net.DialTimeout("tcp", m, time.Second)
		if err != nil {
			failCount++
			if failCount >= 5 {
				this.fightForJudge()
				failCount = 0
				continue
			}
		} else {
			conn.Close()
		}
		select {
		case <-this.quiting:
			return
		case <-time.After(2 * time.Second):
			//
		}
	}
}

func (this *Watcher) fightForJudge() {
	minFailCount, cerr := strconv.Atoi(this.bindData.minFailCount)
	if cerr != nil {
		log.Println("Can not convert minFailCount: ", minFailCount)
		minFailCount = 100
	}
	if len(this.bindData.failList) < minFailCount {
		return
	}
}

func (this *Watcher) Shutdown() {
	if this.zkConn != nil {
		close(this.quiting)
		this.zkConn.Close()
		this.zkConn = nil
	}
}
