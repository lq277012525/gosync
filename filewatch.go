package main

import (
	"github.com/fsnotify/fsnotify"
	"log"
	"path/filepath"
	"os"
	"time"
)

type  myEvent struct {
	fsnotify.Event
	Ext	string
	Dir  bool
}
type fileWatch struct {
	watch *fsnotify.Watcher
	cur  *myEvent
	last *myEvent
	uptime time.Time
}

func NewWatch() *fileWatch {
	Watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("Init monitor error: ", err.Error())
		return nil
	}
	watch :=&fileWatch{
		watch:Watcher,
	}
	return watch
}


func WatchList(list map[string]*ModuleWatch,f func(basePath string,Watcher* myEvent) ) *fileWatch {

	watch:=NewWatch()
	for _,pa:=range list{
		watch.WatchPath(pa.Path)
	}

	go func() {
		t:=time.NewTicker(time.Microsecond*250)
		for {
			select {
			case event := <-watch.watch.Events:
				watch.PushChange(event)
				if watch.last!=nil {
					f(GetBasePath(list,watch.last.Name),watch.last)
					watch.last=nil
				}
			case err := <-watch.watch.Errors:
				log.Println("error:", err)
			case <-t.C:
				if watch.cur==nil {
					continue
				}
				if !time.Now().Before(watch.uptime) {
					f(GetBasePath(list,watch.cur.Name),watch.cur)
					watch.cur=nil
				}
			}
		}
	}()
	return  watch
}

func (p *fileWatch)Close(){
	p.watch.Close()
}
func GetBasePath(list map[string]*ModuleWatch,file string) string {
	file=filepath.ToSlash(file)
	for mod,base:=range list {
		if file[0:len(base.Path)]==base.Path {
			return mod
		}
	}
	return ""
}
func (this * fileWatch)WatchPath(root string) {
	this.watch.Add(root)
	log.Println("watch path=",root)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir(){
			s,_:=filepath.Rel(root,path)
			if s=="." {
				return nil
			}
			this.WatchPath(filepath.ToSlash(path))
		}
		return nil
	})
}
func (this * fileWatch)PushChange(info fsnotify.Event){

	// REMOVE os.Stat(info.Name) 回会为空号
	log.Println(info)
	isDir:=false
	if info.Op&fsnotify.Write == fsnotify.Write {
		state, _ := os.Stat(info.Name)
		if state.IsDir() {
			isDir=true
			return
		}
	}else if info.Op&fsnotify.Create == fsnotify.Create {
		state, _ := os.Stat(info.Name)
		if state.IsDir() {
			isDir=true
			this.WatchPath(info.Name)
		}
	}
	cur:= myEvent{Event:info,Dir:isDir}

	if this.cur==nil||this.cur.Name==info.Name {
		this.cur=&cur
		this.uptime=time.Now().Add(time.Millisecond*800)
	}else  {
		this.last=this.cur
		if this.last.Op&fsnotify.Rename == fsnotify.Rename &&
			info.Op&fsnotify.Create == fsnotify.Create{
			this.last.Ext=info.Name
			this.last.Dir=isDir
			this.cur=nil
		}else {
			this.cur=&cur
			this.uptime=time.Now().Add(time.Millisecond*800)
		}
	}
}
