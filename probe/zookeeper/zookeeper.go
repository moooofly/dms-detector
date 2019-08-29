package zookeeper

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

func (s *ZkProbe) Start(args interface{}, log *logrus.Logger) error {
	s.log = log
	//s.cfg = args.(ZkProbeArgs)

	return s.detect()
}

func (s *ZkProbe) Clean() {
	s.StopProbe()
}

// 判定条件

// 代码逻辑
func (s *ZkProbe) detect() error {
	s.log.Println("[detector/zookeeper] --> probe start")
	defer s.log.Println("[detector/zookeeper] <-- probe done")

	s.log.Println("[detector/zookeeper]   --> try to connect elector")

	// TODO: 连接复用问题
	conn, err := grpc.Dial(
		parser.DetectorSetting.ElectorHost,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(time.Second),
	)
	if err != nil {
		s.log.Printf("[detector/zookeeper] connect elector[%s] failed, err => [%v]", parser.DetectorSetting.ElectorHost, err)
		return errors.New("connect elector failed")
	}
	defer conn.Close()

	s.log.Printf("[detector/zookeeper] connect elector[%s] success", parser.DetectorSetting.ElectorHost)

	client := pb.NewRoleServiceClient(conn)
	obRsp, err := client.Obtain(context.Background(), &pb.ObtainReq{})

	if err != nil {
		s.log.Infof("[detector/zookeeper] Obtain role failed: %v", err)
		return errors.New("obtain role from elector failed")
	}

	s.log.Infof("[detector/zookeeper] role => [%s]", obRsp.GetRole())

	if pb.EnumRole_Leader == obRsp.GetRole() {
		return nil
	} else {
		return errors.New("role of elector is not Leader")
	}
}
