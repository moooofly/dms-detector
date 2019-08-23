package zookeeper

import (
	"errors"

	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/probe"
	"github.com/sirupsen/logrus"
)

type ZkProbeArgs struct {
}

type ZkProbe struct {
	cfg    ZkProbeArgs
	log    *logrus.Logger
	isStop bool
}

func NewZkProbe() probe.Probe {
	return &ZkProbe{
		cfg:    ZkProbeArgs{},
		log:    nil,
		isStop: false,
	}
}

func (s *ZkProbe) StopProbe() {
	defer func() {
		e := recover()
		if e != nil {
			s.log.Printf("Stop ZkProbe crashed, %s", e)
		} else {
			s.log.Printf("probe ZkProbe stopped")
		}
		s.cfg = ZkProbeArgs{}
		s.log = nil
		s = nil
	}()
	s.isStop = true
}

func (s *ZkProbe) Start(args interface{}, log *logrus.Logger) (err error) {
	s.log = log
	//s.cfg = args.(ZkProbeArgs)

	if ok := s.detect(); ok {
		return nil
	} else {
		return errors.New("detect failed")
	}

	return
}

func (s *ZkProbe) Clean() {
	s.StopProbe()
}

// 判定条件

// 代码逻辑
func (s *ZkProbe) detect() bool {
	s.log.Println("[detector/zookeeper] --> probe start")
	defer s.log.Println("[detector/zookeeper] <-- probe done")

	s.log.Println("[detector/zookeeper] try to connect elector")
	if true {
		s.log.Printf("[detector/zookeeper] connect elector[%s] success", parser.DetectorSetting.ElectorHost)
		if true {
			s.log.Println("[detector/zookeeper] elector role -> [leader]")
			return true
		} else {
			s.log.Println("[detector/zookeeper] elector role -> [follower]")
			return false
		}
	} else {
		s.log.Printf("[detector/zookeeper] connect elector[%s] failed", parser.DetectorSetting.ElectorHost)
		return false
	}

	return true
}
