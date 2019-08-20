package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/moooofly/dms-detector/pkg/version"
	"github.com/moooofly/dms-detector/probes"
)

//var APP_VERSION = "No Version Provided"
var APP_VERSION = fmt.Sprintf("%s\n%s\n| % -20s | % -40s |\n| % -20s | % -40s |\n| % -20s | % -40s |\n| % -20s | % -40s |\n| % -20s | % -40s |\n| % -20s | % -40s |\n%s\n",
	version.Logo,
	version.Mark,
	"Client Version", version.ClientVersion,
	"Go Version", version.GoVersion,
	"UTC Build Time", version.UTCBuildTime,
	"Git Branch", version.GitBranch,
	"Git Tag", version.GitTag,
	"Git Hash", version.GitHash,
	version.Mark)

func main() {
	err := initConfig()
	if err != nil {
		log.Fatalf("err : %s", err)
	}
	if pbi != nil && pbi.S != nil {
		Clean(&pbi.S)
	} else {
		Clean(nil)
	}
}

func Clean(s *probes.Probe) {
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				fmt.Printf("crashed, err: %s\nstack:\n%s", e, string(debug.Stack()))
			}
		}()
		for range signalChan {
			log.Println("Received an interrupt, stopping probes...")
			if s != nil && *s != nil {
				(*s).Clean()
			}
			if cmd != nil {
				log.Printf("clean process %d", cmd.Process.Pid)
				cmd.Process.Kill()
			}
			if *isDebug {
				saveProfiling()
			}
			cleanupDone <- true
		}
	}()
	<-cleanupDone
	os.Exit(0)
}
