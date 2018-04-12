package main

import (
	"github.com/fsnotify/fsnotify"
	"path/filepath"
	"os"
	"time"
	"encoding/json"
	"fmt"
	"log"
)

type Proto struct {
	Err    string
	Event  *FileEvent
	//
	GetAll *string
	GetAllResp map[string]FileState
}
type FileEvent struct {
	Event   myEvent
	State   FileState
	Mode    string
}
type FileState struct {
	Url string
	ModTime time.Time
	Mod  os.FileMode
}
func (this *Proto)toByte()[]byte{
	b,_:=json.Marshal(this)
	return b
}

func (this *Proto)ReqGetAll(c *Context){
	mod:=*this.GetAll
	p,find:=prg.conf.Watchs[mod]
	if  !find {
		panic("not find mod")
	}
	if this.GetAllResp==nil {
		this.GetAllResp=make(map[string]FileState)
	}
	err := filepath.Walk(p.Path, func(path string, f os.FileInfo, err error) error {
		if ( f == nil ) {return err}
		if f.IsDir() {return nil}
		if IsExclude(mod,path) {
			return nil
		}
		//相对路径
		rel,err:=filepath.Rel(p.Path,path)
		if err!=nil {
			log.Println(err)
			return nil
		}
		//防止重复 文件名
		this.GetAllResp[rel]=FileState{
			Url:fmt.Sprintf("%s/%s",mod,rel),
			ModTime:f.ModTime(),
			Mod:f.Mode(),
		}
		return nil
	})

	if err!=nil {
		panic(err)
	}

	c.Send(this.toByte())
}
func RemoveDir(path string, f os.FileInfo, err error) error{
	if err!=nil {
		log.Println(err)
		return err
	}

	if f.IsDir() {
		filepath.Walk(path, func(pathTmp string, info os.FileInfo, err error) error {
			if pathTmp!=path {
				return RemoveDir(pathTmp,info,err)
			}
			return nil
		})
	}else {
		err:=os.Remove(path)
		if err!=nil {
			log.Println(err)
		}
	}
	return nil
}


func (this *Proto)RespEvent(c *Context)  {
	Event:=this.Event
	if Event.Mode!=prg.conf.Client.Watch {
		return
	}
	opcode:=Event.Event.Op
	if opcode&fsnotify.Create == fsnotify.Create ||
		opcode&fsnotify.Write == fsnotify.Write{
		if Event.Event.Dir {
			os.MkdirAll(filepath.Join(prg.conf.Client.Path,Event.Event.Name),os.ModePerm)
		}else{
			GetFile(Event.State)
		}
	}else if opcode &fsnotify.Rename==fsnotify.Rename  {
		pathDir:=filepath.Join(prg.conf.Client.Path,Event.Event.Name)
		newPath:=filepath.Join(prg.conf.Client.Path,Event.Event.Ext)
		err:=os.Rename(pathDir,newPath)
		if err!=nil {
			log.Println(err)
		}
	}else {
		pathDir:=filepath.Join(prg.conf.Client.Path,Event.Event.Name)
		stat,err:=os.Stat(pathDir)
		RemoveDir(pathDir,stat,err)
		os.Remove(pathDir)
	}
}
func (this *Proto)RespGetAllResp(c *Context)  {

	err := filepath.Walk(prg.conf.Client.Path, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {return nil}
		if IsExcludeClient(path) {
			return nil
		}
		rel,err:=filepath.Rel(prg.conf.Client.Path,path)
		if err!=nil {
			log.Println(err)
			return nil
		}
		info,find:= this.GetAllResp[rel]
		if find&& info.ModTime==f.ModTime() {
			delete(this.GetAllResp,rel)
			return nil
		}

		return nil
	})

	for _,info:=range this.GetAllResp {
		GetFile(info)
	}

	if err!=nil {
		panic(err)
	}
}