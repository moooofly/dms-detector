package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moooofly/dms-detector/pkg/setting"
	"github.com/moooofly/dms-detector/probes"
	"github.com/moooofly/dms-detector/probes/mysql"
	"github.com/moooofly/dms-detector/probes/radar_server"
	"github.com/moooofly/dms-detector/probes/redis"
	"github.com/moooofly/dms-detector/probes/redis_nms"
	"github.com/moooofly/dms-detector/probes/zookeeper"
	"github.com/sirupsen/logrus"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	// CLI facilities
	app *kingpin.Application
	cmd *exec.Cmd

	// profiling
	cpuProfFile          *os.File
	memProfFile          *os.File
	blockProfFile        *os.File
	goroutineProfFile    *os.File
	threadcreateProfFile *os.File

	isDebug *bool

	pbi    *probes.ProbeItem
	prober string
)

type customLogFormat struct {
	logrus.JSONFormatter
}

func (f *customLogFormat) Format(entry *logrus.Entry) ([]byte, error) {
	json_out, err := f.JSONFormatter.Format(entry)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	b.WriteByte('[')
	b.WriteString(strings.TrimRight(string(json_out), "\n"))
	b.WriteByte(']')
	b.WriteByte('\n')

	return b.Bytes(), nil
}

func initConfig() (err error) {

	// 定制 logrus 日志格式
	/*
		logrus.SetFormatter(&logrus.JSONFormatter{
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "_timestamp",
				logrus.FieldKeyLevel: "_level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	*/

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	//mysqlProbeArgs := mysql.MySQLProbeArgs{}

	app = kingpin.New("detector", "This is a component of dms called detector.")
	app.Author("moooofly").Version(APP_VERSION)

	// global settings
	isDebug = app.Flag("debug", "debug log output").Default("false").Bool()
	daemon := app.Flag("daemon", "run detector in background").Default("false").Bool()
	forever := app.Flag("forever", "run detector in forever, fail and retry").Default("false").Bool()
	logfile := app.Flag("log", "log file path").Default("").String()
	nolog := app.Flag("nolog", "turn off logging").Default("false").Bool()

	// sub command
	_ = app.Command("mysql", "prober for mysql")
	_ = app.Command("redis", "prober for redis")
	_ = app.Command("redis_nms", "prober for redis_nms")
	_ = app.Command("radar", "prober for radar_server")
	_ = app.Command("zookeeper", "prober for zookeeper")

	prober = kingpin.MustParse(app.Parse(os.Args[1:]))

	// ini 配置解析
	setting.Load(prober)

	var pb probes.Probe
	switch prober {
	case "mysql":
		pb = mysql.NewMySQLProbe()
	case "redis":
		pb = redis.NewRedisProbe()
	case "redis_nms":
		pb = redis_nms.NewRedisNmsProbe()
	case "radar":
		pb = radar_server.NewRadarProbe()
	case "zookeeper":
		pb = zookeeper.NewZkProbe()
	default:
		logrus.Fatal("not match any of [mysql|redis|redis_nms|radar_server|zookeeper].")
	}
	probes.Regist(prober, pb, nil, logrus.StandardLogger())

	if *isDebug {
		cpuProfFile, _ = os.Create("cpu.prof")
		memProfFile, _ = os.Create("memory.prof")
		blockProfFile, _ = os.Create("block.prof")
		goroutineProfFile, _ = os.Create("goroutine.prof")
		threadcreateProfFile, _ = os.Create("threadcreate.prof")
		pprof.StartCPUProfile(cpuProfFile)
	}

	if *nolog {
		logrus.SetOutput(ioutil.Discard)
	} else if *logfile != "" {
		f, e := os.OpenFile(*logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if e != nil {
			logrus.Fatal(e)
		}
		logrus.SetOutput(f)
	}
	if *daemon {
		args := []string{}
		for _, arg := range os.Args[1:] {
			if arg != "--daemon" {
				args = append(args, arg)
			}
		}
		cmd = exec.Command(os.Args[0], args...)
		cmd.Start()
		f := ""
		if *forever {
			f = "forever "
		}
		logrus.Printf("%s%s [PID] %d running...\n", f, os.Args[0], cmd.Process.Pid)
		os.Exit(0)
	}
	if *forever {
		args := []string{}
		for _, arg := range os.Args[1:] {
			if arg != "--forever" {
				args = append(args, arg)
			}
		}
		go func() {
			defer func() {
				if e := recover(); e != nil {
					fmt.Printf("crashed, err: %s\nstack:%s", e, string(debug.Stack()))
				}
			}()
			for {
				if cmd != nil {
					cmd.Process.Kill()
					time.Sleep(time.Second * 5)
				}
				cmd = exec.Command(os.Args[0], args...)
				cmdReaderStderr, err := cmd.StderrPipe()
				if err != nil {
					logrus.Printf("ERR: %s, restarting...\n", err)
					continue
				}
				cmdReader, err := cmd.StdoutPipe()
				if err != nil {
					logrus.Printf("ERR: %s, restarting...\n", err)
					continue
				}
				scanner := bufio.NewScanner(cmdReader)
				scannerStdErr := bufio.NewScanner(cmdReaderStderr)
				go func() {
					defer func() {
						if e := recover(); e != nil {
							fmt.Printf("crashed, err: %s\nstack:%s", e, string(debug.Stack()))
						}
					}()
					for scanner.Scan() {
						fmt.Println(scanner.Text())
					}
				}()
				go func() {
					defer func() {
						if e := recover(); e != nil {
							fmt.Printf("crashed, err: %s\nstack:%s", e, string(debug.Stack()))
						}
					}()
					for scannerStdErr.Scan() {
						fmt.Println(scannerStdErr.Text())
					}
				}()
				if err := cmd.Start(); err != nil {
					logrus.Printf("ERR: %s, restarting...\n", err)
					continue
				}
				pid := cmd.Process.Pid
				logrus.Printf("worker %s [PID] %d running...\n", os.Args[0], pid)
				if err := cmd.Wait(); err != nil {
					logrus.Printf("ERR: %s, restarting...", err)
					continue
				}
				logrus.Printf("worker %s [PID] %d unexpected exited, restarting...\n", os.Args[0], pid)
			}
		}()
		return
	}
	if *logfile == "" {
		if *isDebug {
			logrus.Println("[profiling] cpu profiling save to file: cpu.prof")
			logrus.Println("[profiling] memory profiling save to file: memory.prof")
			logrus.Println("[profiling] block profiling save to file: block.prof")
			logrus.Println("[profiling] goroutine profiling save to file: goroutine.prof")
			logrus.Println("[profiling] threadcreate profiling save to file: threadcreate.prof")
		}
	}

	router := InitRouter()
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", setting.DetectorSetting.Port),
		Handler: router,
		//ReadTimeout:    setting.ReadTimeout,
		//WriteTimeout:   setting.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	if err := s.ListenAndServe(); err != nil {
		log.Printf("Listen: %s\n", err)
	}

	return
}

func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	//gin.SetMode(setting.ServerSetting.RunMode)

	// 鉴权 API
	r.HEAD("/", headCallback)

	return r
}

func headCallback(c *gin.Context) {
	logrus.Infof("probe [%s] triggered by HaProxy HEAD request.", prober)
	_, err := probes.Run(prober, nil)
	if err != nil {
		logrus.Infof("probe [%s] %s", prober, err)
		c.String(http.StatusServiceUnavailable, "")
	} else {
		logrus.Infof("probe [%s] success", prober)
		c.String(http.StatusOK, "")
	}
	// self.send_header('Content-type', 'text/html')
}

func saveProfiling() {
	goroutine := pprof.Lookup("goroutine")
	goroutine.WriteTo(goroutineProfFile, 1)

	heap := pprof.Lookup("heap")
	heap.WriteTo(memProfFile, 1)

	block := pprof.Lookup("block")
	block.WriteTo(blockProfFile, 1)

	threadcreate := pprof.Lookup("threadcreate")
	threadcreate.WriteTo(threadcreateProfFile, 1)

	pprof.StopCPUProfile()
}
