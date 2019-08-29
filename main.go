package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	srv "github.com/moooofly/dms-detector/pkg/servitization"
	"github.com/moooofly/dms-detector/probe"
	"github.com/moooofly/dms-detector/router"
)

func main() {
	if err := srv.Init(); err != nil {
		log.Fatalf("err : %s", err)
	}
	go func() {
		if err := router.Launch(); err != nil {
			log.Fatalf("err : %s", err)
		}
	}()
	if srv.Pbi != nil && srv.Pbi.S != nil {
		Clean(&srv.Pbi.S)
	} else {
		Clean(nil)
	}
}

func Clean(s *probe.Probe) {
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
			srv.Teardown()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
	os.Exit(0)
}
