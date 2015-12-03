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
	go this.bindKey(this.conf.ZkPath.Master, &this.master, true)
	go this.bindKey(this.conf.ZkPath.Judge, &this.judge, false)
	go this.bindKey(this.conf.ZkPath.MinFail, &this.minFailCount, true)

	go this.bindList(this.conf.ZkPath.Server, &this.serverList)
	go this.bindList(this.conf.ZkPath.Fail, &this.failList)
}

func (this *zkBindData) shutdown() {
	close(this.quiting)
	this.wg.Wait()
}

func (this *zkBindData) bindKey(path string, data *string, printLogIfAbsent bool) {
	this.wg.Add(1)
	defer this.wg.Done()
	path = this.conf.ZkPath.Root + path
	for {
		d, _, evt, err := this.conn.GetW(path)
		if err != nil {
			if err != zk.ErrNoNode || printLogIfAbsent {
				log.Printf("Get %s zk data error: %s\n", path, err)
			}
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
		log.Printf("Update value from zk %s: %s\n", path, *data)
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
	path = this.conf.ZkPath.Root + path
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
		log.Printf("Update value from zk %s: %s\n", path, *data)
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
	conf       *Config
	zkConn     *zk.Conn
	zkConnChan <-chan zk.Event
	wg         sync.WaitGroup
	bindData   *zkBindData
	quiting    chan interface{}
	judgeKey   string
	state      WatcherState
}

func NewWatcher(conf *Config) *Watcher {
	conn, ch, err := zk.Connect(conf.Zk.ZkHost, time.Duration(conf.Zk.ZkTimeout)*time.Millisecond)
	if err != nil {
		log.Fatal("Can not connect zk server ", conf.Zk.ZkHost, err)
		return nil
	}
	bind := newZkBindData(conn, conf)
	//judgeValue := createJudge()
	//log.Println("JudgeId: ", judgeId)
	quiting := make(chan interface{})
	return &Watcher{conf: conf, zkConn: conn, zkConnChan: ch, bindData: bind, quiting: quiting, state: WatcherStateNotRunning}
}

func createJudge() string {
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
	log.Println("Start watcher")
}

func (this *Watcher) watchMaster() {
	defer this.wg.Done()
	this.wg.Add(1)

	var failCount int
	for {
		m := this.bindData.master
		var success bool
		if this.state == WatcherStateWatching && m == "" {
			success = false
		} else {
			conn, err := net.DialTimeout("tcp", m, time.Second)
			if err != nil {
				log.Printf("Connect master [%d] error %s: %s\n", failCount, m, err)
				success = false
			} else {
				// 如果重新可以连接，那就失败数归0
				failCount = 0
				conn.Close()
				this.delFail()
			}
		}
		if !success {
			failCount++
			if this.state == WatcherStateWatching && failCount >= 3 {
				if this.addFail() {
					runtime.Gosched()
					select {
					case <-this.quiting:
						return
					case <-time.After(3 * time.Second):
						//
					}

					this.state = WatcherStateFightForJudge
					go this.fightForJudge()
					failCount = 0
				}
			}
		}

		select {
		case <-this.quiting:
			return
		case <-time.After(2 * time.Second):
			//
		}
	}
}

func (this *Watcher) addFail() bool {
	if this.judgeKey != "" {
		log.Println("judgeKey is existing: ", this.judgeKey)
		return true
	}
	path := this.conf.ZkPath.Root + this.conf.ZkPath.Fail + "/watcher"
	newPath, err := this.zkConn.Create(path, []byte("1"), zk.FlagEphemeral+zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		log.Println("Can not create fail node:", err)
		return false
	}
	log.Println("added fail, key ", newPath)
	this.judgeKey = newPath
	return true
}

func (this *Watcher) delFail() bool {
	if this.judgeKey != "" {
		return false
	}
	err := this.zkConn.Delete(this.judgeKey, -1)
	if err != nil {
		log.Println("Can not create fail node:", err)
		return false
	}
	this.judgeKey = ""
	return true
}

func (this *Watcher) fightForJudge() {
	defer func() {
		if this.state == WatcherStateFightForJudge || this.state == WatcherStateSwitchMaster {
			this.state = WatcherStateWatching
		}
	}()

	mfc := this.bindData.minFailCount
	minFailCount, cerr := strconv.Atoi(mfc)
	if cerr != nil {
		log.Println("Can not convert minFailCount at fightForJudge: ", mfc)
		minFailCount = 100
	}
	//log.Printf("len(this.bindData.failList) %d, minFailCount %d\n", len(this.bindData.failList), minFailCount)
	if len(this.bindData.failList) < minFailCount {
		return
	}

	log.Println("fightForJudge")
	if len(this.bindData.serverList) == 0 {
		log.Println("ServerList is empty")
	}

	path := this.conf.ZkPath.Root + this.conf.ZkPath.Judge
	_, err := this.zkConn.Create(path, []byte(this.judgeKey), zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err == zk.ErrNodeExists {
		//已经存在了
		return
	}
	defer this.zkConn.Delete(path, -1)
	log.Println("Create judge key:", path)

	var newMaster string
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
			log.Println("Judge, new master ", srv)
			newMaster = srv
			break
		}
	}

	if newMaster != "" {
		this.state = WatcherStateSwitchMaster
		log.Println("Set Master to blank first")
		mpath := this.conf.ZkPath.Root + this.conf.ZkPath.Master
		this.zkConn.Set(mpath, []byte(""), 0)

		log.Println("Wait for 2 seconds")
		select {
		case <-this.quiting:
			return
		case <-time.After(2 * time.Second):
			//
		}

		for j := 0; j < 5; j++ {
			//更新master的值，如果失败就重试
			_, err := this.zkConn.Set(mpath, []byte(newMaster), 0)
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
			break
		}
	}

	log.Println("Judge, Not found valid master")
}

func (this *Watcher) Wait() {
	this.wg.Wait()
}

func (this *Watcher) Shutdown() {
	if this.zkConn != nil {
		log.Println("Shutdown watcher")
		close(this.quiting)
		this.state = WatcherStateClosed
		this.wg.Wait()
		this.zkConn.Close()
		this.zkConn = nil
	}
}
