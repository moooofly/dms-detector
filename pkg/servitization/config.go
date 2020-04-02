package servitization

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime/debug"
	"time"

	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/pkg/version"
	"github.com/moooofly/dms-detector/probe"
	"github.com/moooofly/dms-detector/probe/highgo"
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

var dbg *bool
var prof *bool

var Pbi *probe.ProbeItem
var Prober string
var Output io.Writer

func Init() (err error) {

	// 定制 logrus 日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006/01/02 - 15:04:05",
	})

	app = kingpin.New("detector", "This is a component of dms called detector.")
	app.Author("moooofly").Version(version.Version)

	// global settings
	dbg = app.Flag("debug", "output all kinds of logs to os.Stdout").Default("false").Bool()
	prof = app.Flag("prof", "generate all kinds of profile into files").Default("false").Bool()
	daemon := app.Flag("daemon", "run detector in background").Default("false").Bool()
	forever := app.Flag("forever", "run detector in forever, fail and retry").Default("false").Bool()
	logfile := app.Flag("log-file", "log file, e.g. '/opt/log/dms/xxx.log'").Default("").String()
	confPath := app.Flag("conf-path", "config file path, e.g. '/opt/config/dms'").Default("conf").String()
	nolog := app.Flag("nolog", "turn off logging").Default("false").Bool()

	// sub command
	_ = app.Command("mysql", "prober for mysql")
	_ = app.Command("highgo", "prober for highgo")
	_ = app.Command("redis", "prober for redis")
	_ = app.Command("redis_nms", "prober for redis_nms")
	_ = app.Command("radar", "prober for radar_server")
	_ = app.Command("zookeeper", "prober for zookeeper")

	Prober = kingpin.MustParse(app.Parse(os.Args[1:]))

	// ini 配置解析
	parser.Load(Prober, *confPath)

	var pb probe.Probe
	switch Prober {
	case "mysql":
		pb = mysql.NewMySQLProbe()
	case "highgo":
		pb = highgo.NewHighgoProbe()
	case "redis":
		pb = redis.NewRedisProbe()
	case "redis_nms":
		pb = redis_nms.NewRedisNmsProbe()
	case "radar":
		pb = radar_server.NewRadarProbe()
	case "zookeeper":
		pb = zookeeper.NewZkProbe()
	default:
		logrus.Fatal("not match any of [mysql|highgo|redis|redis_nms|radar_server|zookeeper].")
	}
	probe.Regist(Prober, pb, nil, logrus.StandardLogger())

	// log setting
	if *dbg {
		Output = os.Stdout
		logrus.SetOutput(Output)
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		if *nolog {
			Output = ioutil.Discard
			logrus.SetOutput(Output)
		} else if *logfile != "" {
			f, err := os.OpenFile(*logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				logrus.Fatal(err)
			}
			Output = f
			logrus.SetOutput(Output)
		} else if parser.DetectorSetting.LogFile != "" {
			f, err := os.OpenFile(parser.DetectorSetting.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				logrus.Fatal(err)
			}
			Output = f
			logrus.SetOutput(Output)
		} else {
			Output = os.Stdout
			logrus.SetOutput(Output)
		}
		l, err := logrus.ParseLevel(parser.DetectorSetting.LogLevel)
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Warningf("Update output log level to [%s]", parser.DetectorSetting.LogLevel)
		logrus.SetLevel(l)
	}

	// pprof setting
	if *prof {
		startProfiling()
	}

	// daemon setting
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

	return
}

func Teardown() {
	if cmd != nil {
		logrus.Infof("clean process %d", cmd.Process.Pid)
		cmd.Process.Kill()
	}
	if *prof {
		saveProfiling()
	}
}
