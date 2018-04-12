// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"time"
	"github.com/gorilla/websocket"
	"encoding/json"
	"reflect"
	"fmt"
	"net/url"
	"bytes"
	"net/http"
	"os"
	"io"
	"path/filepath"
	"syscall"
	"net"
	"runtime/debug"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 100 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 600 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512000
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)


// Client is a middleman between the websocket connection and the hub.
type Client struct {
	//ctx
	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
	recon bool
}

type Context struct {
	sendQueue [][]byte
}


func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("error: %v", err)
			//if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			//	log.Printf("error: %v", err)
			//}
			break
		}
		var CTX Context
		CTX.sendQueue=make([][]byte,0)
		c.HandleMsg(&CTX,message)
		if CTX.sendQueue!=nil {
			for _,v:=range CTX.sendQueue  {
				c.send<-bytes.TrimSpace(bytes.Replace(v, newline, space, -1))
			}
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			log.Println("send Msg=",string(message))
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			//n := len(c.send)
			//for i := 0; i < n; i++ {
			//	w.Write(newline)
			//	w.Write(<-c.send)
			//}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.

func (c *Client) HandleMsg(ctx *Context,msg []byte)  {
	log.Printf("recv msg[%s]",string(msg))
	defer func() {
		if err:=recover();err!=nil{
			if !IsServer {
				log.Println(err)
				debug.PrintStack()
				return
			}
			ctx.sendQueue=nil
			var p Proto
			p.Err=fmt.Sprint(err)
			bt,_:=json.Marshal(p)
			c.send<-bt
		}
	}()
	var p Proto
	err:=json.Unmarshal(msg,&p)
	if err!=nil {
		log.Println(err)
		return
	}
	tv :=reflect.ValueOf(&p)
	tt:= reflect.TypeOf(&p)
	for i:=0 ;i< tt.Elem().NumField();i++ {
		tmp:= tt.Elem().Field(i)
		k := tmp.Type.Kind()
		if !(k== reflect.Chan || k== reflect.Func||
			k==reflect.Map||k==reflect.Ptr ||
			k==reflect.Interface||k==reflect.Slice){
			continue
		}
		if tv.Elem().Field(i).IsNil() {
			continue
		}
		fix:="Req"
		if !IsServer {
			fix="Resp"
		}
		me:=tv.MethodByName(fmt.Sprintf("%s%s",fix,tmp.Name))
		if me.Kind()!=reflect.Func {
			continue
		}
		me.Call([]reflect.Value{reflect.ValueOf(ctx)})
	}

}
func (c *Context) Send(msg []byte){
	c.sendQueue=append(c.sendQueue,msg)
}

func NewClient(config *Config) *Client {
	cl:= Client{
		conn:nil,
		send:make(chan []byte),
	}
	go func() {
		for  {
			u := url.URL{Scheme: "ws", Host: config.Addr, Path: "/ws"}
			log.Printf("connecting to %s", u.String())

			c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				log.Println("dial:", err)
				cl.conn=nil
			}else {
				cl.conn=c
				go cl.writePump()
				cl.readPump()
			}
		}
	}()
	return &cl
}
func (c *Client)Close(){
	if c.conn.Close() !=nil{
		log.Println("a Client close")
	}
}


func GetFile(info FileState){

	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*2)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * 2))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * 2,
		},
	}
	url := fmt.Sprintf("http://%s/%s",prg.conf.Addr,filepath.ToSlash(info.Url))
	res,err:=client.Get(url)
	if err!=nil {
		log.Println("get url err =",err,"file=",info.Url)
		return
	}
    defer res.Body.Close()
	if res.StatusCode!=200 {
		log.Println("res.StatusCode=", res.StatusCode)
		return
	}
	info.Url=info.Url[len(prg.conf.Client.Watch)+1:]
	pathDir:=filepath.Join(prg.conf.Client.Path,info.Url)
	os.MkdirAll(filepath.Dir(pathDir),os.ModePerm)
	filePath:=filepath.Join(prg.conf.Client.Path,info.Url)
	file,err:=os.Create(filePath)
	if err!=nil {
		log.Println(err)
		return
	}

	//文件 修改时间
	file.Chmod(info.Mod)
	io.Copy(file,res.Body)
	file.Close()

	//关闭后再修改
	modTime:= syscall.NsecToTimeval(info.ModTime.UnixNano())
	syscall.Utimes(filePath,[]syscall.Timeval{modTime,modTime})

}

func IsExcludeClient(file string) bool {

	watch:=prg.conf.Client.Exclude
	for _,ex :=range watch {
		match,err:=filepath.Match(ex,file)
		if err==nil {
			return match
		}
	}
	return false
}