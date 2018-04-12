package main

import (
	"net/http"
	"github.com/gorilla/websocket"
	"log"
	"fmt"
	"encoding/json"
	"path/filepath"
	"time"
	"os"
)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
type server struct {
	watch *fileWatch
	hb 	   *Hub
}

func NewServer(conf *Config) *server {

	hub := newHub()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	for s,pa:=range conf.Watchs{
		fix:=fmt.Sprintf("/%s/",s)
		http.Handle(fix, http.StripPrefix(fix, http.FileServer(http.Dir(pa.Path))))
	}
	go func() {
		log.Println("listen ip=",conf.Addr)
		err:=http.ListenAndServe(conf.Addr, nil)
		if err!=nil {
			log.Fatal(err)
		}
	}()
	go hub.run()
	 ser := &server{}
	 ser.watch=WatchList(conf.Watchs, func(mode string,Watcher* myEvent) {
	 	 log.Printf("[%s]:%s",mode,Watcher)
		 ModeTie:=time.Now()
		 Mod:=os.ModePerm
		 modepath:=prg.conf.Watchs[mode].Path

		 rel,_:=filepath.Rel(modepath,Watcher.Name)
		 Watcher.Name=rel
		 if Watcher.Ext!="" {
			 Watcher.Ext,_=filepath.Rel(modepath,Watcher.Ext)
		 }
		 msg ,err:=json.Marshal(Proto{
			Event:&FileEvent{
				Event: *Watcher,
				Mode:  mode,
				State: FileState {
					Url:fmt.Sprintf("%s/%s", mode, rel),
					ModTime:ModeTie,
					Mod:Mod,
					},
				},
		 })
		 if err!=nil {
			log.Println(err)
			return
		 }
		 if IsExclude(mode,Watcher.Name) {
			 return
		 }

		 hub.broadcast<-msg
	 })
	ser.hb=hub
	return ser
}
func (p *server)Close()  {
	p.watch.Close()
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{ conn: conn, send: make(chan []byte, 256)}
	hub.register <- client
	log.Println("new client connect")
	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go func() {
		client.writePump()
		hub.unregister<-client
	}()
	go func() {
		client.readPump()
		hub.unregister<-client
	}()
}

func IsExclude(basePath,file string) bool {
	ext:= filepath.Ext(file)
	if prg.conf.excludeMap[ext]!=0 {
		return true
	}
	watch:=prg.conf.Watchs[basePath]
	for _,ex :=range watch.Exclude {
		match,err:=filepath.Match(ex,file)
		if err==nil {
			return match
		}
	}
	return false
}
