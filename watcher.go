package main

import (
	"bytes"
	"log"
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/samuel/go-zookeeper/zk"
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
			log.Printf("Get %s zk data error: %s\n", path, err)
			select {
			case <-this.quiting:
				// 如果已经关闭就退出循环
				return
			case <-time.After(3 * time.Second):
				// 否则等待3秒重试
			}
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
			log.Printf("List %s zk data error: %s\n", path, err)
			select {
			case <-this.quiting:
				// 如果已经关闭就退出循环
				return
			case <-time.After(3 * time.Second):
				// 否则等待3秒重试
			}
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

type WatcherState int32

const (
	WatcherStateNotRunning    = WatcherState(0)
	WatcherStateWatching      = WatcherState(1)
	WatcherStateFightForJudge = WatcherState(2)
	WatcherStateSwitchMaster  = WatcherState(3)
	WatcherStateClosed        = WatcherState(100)
)

type Watcher struct {
	conf     *Config
	zkConn   *zk.Conn
	bindData *zkBindData
	quiting  chan interface{}
	judgeId  string
	state    WatcherState
}

func NewWatcher(conf *Config) *Watcher {
	conn, _, err := zk.Connect(conf.Zk.ZkHost, time.Duration(conf.Zk.ZkTimeout)*time.Millisecond)
	if err != nil {
		log.Fatal("Can not connect zk server ", conf.Zk.ZkHost, err)
		return nil
	}
	bind := newZkBindData(conn, conf)
	judgeId := createJudgeId()
	log.Println("JudgeId: ", judgeId)
	quiting := make(chan interface{})
	return &Watcher{conf: conf, zkConn: conn, bindData: bind, quiting: quiting, judgeId: judgeId, state: WatcherStateNotRunning}
}

func createJudgeId() string {
	var buf bytes.Buffer
	buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))

	inters, err := net.InterfaceAddrs()
	if err != nil {
		log.Println("CanNotCreateJudgeId: ", err)
		buf.WriteString(",CanNotCallInterfaceAddrs")
	} else {
		for _, i := range inters {
			buf.WriteString(",")
			buf.WriteString(i.String())
		}
	}

	return string(buf.Bytes())
}

func (this *Watcher) Start() {
	if this.zkConn == nil {
		log.Println("zkConn is null, Can NOT start watcher !")
		return
	}
	if this.state != WatcherStateNotRunning {
		log.Println("It is not under WatcherStateNotRunning state")
		return
	}
	this.state = WatcherStateWatching
	go this.watchMaster()
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
		if this.state == WatcherStateWatching && m == "" {
			this.state = WatcherStateFightForJudge
			go this.fightForJudge()
			failCount = 0
			continue
		}
		conn, err := net.DialTimeout("tcp", m, time.Second)
		if err != nil {
			failCount++
			if this.state == WatcherStateWatching && failCount >= 5 {
				this.state = WatcherStateFightForJudge
				go this.fightForJudge()
				failCount = 0
				continue
			}
		} else {
			// 如果重新可以连接，那就失败数归0
			failCount = 0
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
	defer func() {
		if this.state == WatcherStateFightForJudge || this.state == WatcherStateSwitchMaster {
			this.state = WatcherStateWatching
		}
	}()

	minFailCount, cerr := strconv.Atoi(this.bindData.minFailCount)
	if cerr != nil {
		log.Println("Can not convert minFailCount: ", minFailCount)
		minFailCount = 100
	}
	if len(this.bindData.failList) < minFailCount {
		return
	}
	path := this.conf.ZkPath.Root + this.conf.ZkPath.Judge
	_, err := this.zkConn.Create(path, []byte(this.judgeId), zk.FlagEphemeral, nil)
	if err == zk.ErrNodeExists {
		//已经存在了
		return
	}
	defer this.zkConn.Delete(path, -1)

	mpath := this.conf.ZkPath.Root + this.conf.ZkPath.Master
	this.zkConn.Set(mpath, []byte(""), 0)

	select {
	case <-this.quiting:
		return
	case <-time.After(2 * time.Second):
		//
	}

	master := this.bindData.master
	for _, srv := range this.bindData.serverList {
		if srv == master {
			continue
		}
		conn, err := net.DialTimeout("tcp", srv, time.Second)
		if err != nil {
			log.Println("Judge, CANNOT connect server ", srv)
			continue
		} else {
			conn.Close()
			log.Println("Judge, Set master ", srv)
			this.state = WatcherStateSwitchMaster
			for j := 0; j < 5; j++ {
				//更新master的值，如果失败就重试
				_, err := this.zkConn.Set(mpath, []byte(srv), 0)
				if err != nil {
					log.Printf("CanNOT set master value in zk [%d]: %s\n", j, err)
					select {
					case <-this.quiting:
						return
					case <-time.After(2 * time.Second):
						//
					}
					continue
				}
				return
			}
			log.Println("Can NOT set master with zk")
			return
		}
	}

	log.Println("Judge, Not found valid master")
}

func (this *Watcher) Shutdown() {
	if this.zkConn != nil {
		close(this.quiting)
		this.state = WatcherStateClosed
		this.zkConn.Close()
		this.zkConn = nil
	}
}
