package servitization

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime/debug"
	"time"

	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/pkg/version"
	"github.com/moooofly/dms-detector/probe"
	"github.com/moooofly/dms-detector/probe/mysql"
	"github.com/moooofly/dms-detector/probe/radar_server"
	"github.com/moooofly/dms-detector/probe/redis"
	"github.com/moooofly/dms-detector/probe/redis_nms"
	"github.com/moooofly/dms-detector/probe/zookeeper"
	"github.com/sirupsen/logrus"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// CLI facilities
var (
	app *kingpin.Application
	cmd *exec.Cmd
)

// custom
var isDebug *bool
var Pbi *probe.ProbeItem
var Prober string

func Init() (err error) {

	// TODO: 定制 logrus 日志格式

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	//mysqlProbeArgs := mysql.MySQLProbeArgs{}

	app = kingpin.New("detector", "This is a component of dms called detector.")
	app.Author("moooofly").Version(version.Version)

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

	Prober = kingpin.MustParse(app.Parse(os.Args[1:]))

	// ini 配置解析
	parser.Load(Prober)

	var pb probe.Probe
	switch Prober {
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
	probe.Regist(Prober, pb, nil, logrus.StandardLogger())

	if *isDebug {
		startProfiling()
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

	return
}

func Teardown() {
	if cmd != nil {
		logrus.Infof("clean process %d", cmd.Process.Pid)
		cmd.Process.Kill()
	}
	if *isDebug {
		saveProfiling()
	}
}
