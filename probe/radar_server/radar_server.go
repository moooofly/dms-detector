package radar_server

import (
	"context"
	"errors"
	"time"

	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/probe"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	pb "github.com/moooofly/dms-detector/proto"
)

type RadarProbeArgs struct {
}

type RadarProbe struct {
	cfg    RadarProbeArgs
	log    *logrus.Logger
	isStop bool
}

func NewRadarProbe() probe.Probe {
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

func (s *RadarProbe) Start(args interface{}, log *logrus.Logger) error {
	s.log = log
	//s.cfg = args.(RadarProbeArgs)

	return s.detect()
}

func (s *RadarProbe) Clean() {
	s.StopProbe()
}

// 判定条件

// 代码逻辑
func (s *RadarProbe) detect() error {
	s.log.Println("[detector/radar] --> probe start")
	defer s.log.Println("[detector/radar] <-- probe done")

	s.log.Println("[detector/radar]   --> try to connect elector")

	// TODO: 连接复用问题
	conn, err := grpc.Dial(
		parser.DetectorSetting.ElectorHost,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(time.Second),
	)
	if err != nil {
		s.log.Printf("[detector/radar] connect elector[%s] failed, err => [%v]", parser.DetectorSetting.ElectorHost, err)
		return errors.New("connect elector failed")
	}
	defer conn.Close()

	s.log.Printf("[detector/radar] connect elector[%s] success", parser.DetectorSetting.ElectorHost)

	client := pb.NewRoleServiceClient(conn)
	obRsp, err := client.Obtain(context.Background(), &pb.ObtainReq{})

	if err != nil {
		s.log.Infof("[detector/radar] Obtain role failed: %v", err)
		return errors.New("obtain role from elector failed")
	}

	s.log.Infof("[detector/radar] role => [%s]", obRsp.GetRole())

	if pb.EnumRole_Leader == obRsp.GetRole() {
		return nil
	} else {
		return errors.New("role of elector is not Leader")
	}
}
