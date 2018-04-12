package main

import (
	"log"
	"github.com/dingdayu/golangtools/config"
	"os"
	"github.com/judwhite/go-svc/svc"
	"flag"
	"path/filepath"
	"runtime/debug"
)

type Config struct {
	Watchs   map[string]*ModuleWatch
	Exclude  []string
	Log      string
	Addr     string
	Client   ClientCfg
	excludeMap map[string]int
}
type ModuleWatch struct {
	Path string
	Exclude  []string
}
type ClientCfg struct {
	Watch string
	Path string
	Exclude  []string
}
type program struct {
	conf Config
	ser *server
	cli *Client
}

var prg  program
var IsServer   bool
var cfgPath string
func main() {
	flag.BoolVar(&IsServer,"s",false,"")
	flag.StringVar(&cfgPath,"c","config.toml","")
	flag.Parse()

	defer func() {
		if err:=recover();err!=nil{
			log.Println(err)
			debug.Stack()
		}
	}()
	// call svc.Run to start your program/service
	// svc.Run will call Init, Start, and Stop
	if err := svc.Run(&prg); err != nil {
		log.Fatal(err)
	}


}
func LoadConfig(Path string,p *program)  {
	err :=config.New(Path,&p.conf)
	if err!=nil {
		log.Fatal(err)
	}
	p.conf.excludeMap=make(map[string]int)
	for i,ex:=range p.conf.Exclude {
		p.conf.excludeMap[ex]=i+1
	}
	for _,ex:=range p.conf.Watchs {
		ex.Path=filepath.ToSlash(ex.Path)
	}

	p.conf.Client.Path=filepath.ToSlash(p.conf.Client.Path)

	log.Println("config:", p.conf)

	if !IsServer {
		_, err := os.Stat(p.conf.Client.Path)
		if err != nil && os.IsNotExist(err) {
			err=os.Mkdir(p.conf.Client.Path, os.ModePerm)
			if err!=nil {
				log.Fatal(err)
			}
		}
	}
}
func (p *program) Init(env svc.Environment) error {

	LoadConfig(cfgPath,p)
	if p.conf.Log!="" {
		logfile,_:=os.Create(p.conf.Log)
		log.SetOutput(logfile)
	}

	log.Printf("is win service? %v\n", env.IsWindowsService())
	// write to "example.log" when running as a Windows Service
	if env.IsWindowsService() {

	}

	return nil
}

func (p *program) Start() error {
	log.Printf("Starting...\n")
	if IsServer {
		p.ser = NewServer(&p.conf)
	}else {
		p.cli = NewClient(&p.conf)
		var msg Proto
		msg.GetAll = &p.conf.Client.Watch
		p.cli.send<-msg.toByte()
	}

	return nil
}

func (p *program) Stop() error {
	log.Printf("Stopping...\n")
	if p.ser!=nil {
		p.ser.Close()
	}
	if p.cli!=nil {
		p.cli.Close()
	}

	log.Printf("Stopped.\n")
	return nil
}
