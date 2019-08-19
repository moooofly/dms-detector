package radar_server

import (
	"errors"

	"github.com/sirupsen/logrus"
	"gitlab.com/kedacom-dms/detector-go/probes"
	"gitlab.com/kedacom-dms/detector-go/util/setting"
)

type RadarProbeArgs struct {
}

type RadarProbe struct {
	cfg    RadarProbeArgs
	log    *logrus.Logger
	isStop bool
}

func NewRadarProbe() probes.Probe {
	return &RadarProbe{
		cfg:    RadarProbeArgs{},
		log:    nil,
		isStop: false,
	}
}

func (s *RadarProbe) StopProbe() {
	defer func() {
		e := recover()
		if e != nil {
			s.log.Printf("Stop RadarProbe crashed, %s", e)
		} else {
			s.log.Printf("probe RadarProbe stopped")
		}
		s.cfg = RadarProbeArgs{}
		s.log = nil
		s = nil
	}()
	s.isStop = true
}

func (s *RadarProbe) Start(args interface{}, log *logrus.Logger) (err error) {
	s.log = log
	//s.cfg = args.(RadarProbeArgs)

	if ok := s.detect(); ok {
		return nil
	} else {
		return errors.New("detect failed")
	}

	return
}

func (s *RadarProbe) Clean() {
	s.StopProbe()
}

// 判定条件

// 代码逻辑
func (s *RadarProbe) detect() bool {
	s.log.Println("[detector/radar] --> probe start")
	defer s.log.Println("[detector/radar] <-- probe done")

	s.log.Println("[detector/radar] try to connect elector")
	if true {
		s.log.Printf("[detector/radar] connect elector[%s] success", setting.DetectorSetting.ElectorHost)
		if true {
			s.log.Println("[detector/radar] elector role -> [leader]")
			return true
		} else {
			s.log.Println("[detector/radar] elector role -> [follower]")
			return false
		}
	} else {
		s.log.Printf("[detector/radar] connect elector[%s] failed", setting.DetectorSetting.ElectorHost)
		return false
	}

	return true
}