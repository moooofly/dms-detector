package highgo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/probe"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	_ "github.com/lib/pq"

	pb "github.com/moooofly/dms-detector/proto"
)

type HighgoProbeArgs struct {
}

type HighgoProbe struct {
	cfg    HighgoProbeArgs
	log    *logrus.Logger
	isStop bool
}

func NewHighgoProbe() probe.Probe {
	return &HighgoProbe{
		cfg:    HighgoProbeArgs{},
		log:    nil,
		isStop: false,
	}
}

func (s *HighgoProbe) StopProbe() {
	defer func() {
		e := recover()
		if e != nil {
			s.log.Printf("Stop HighgoProbe crashed, %s", e)
		} else {
			s.log.Printf("probe HighgoProbe stopped")
		}
		s.cfg = HighgoProbeArgs{}
		s.log = nil
		s = nil
	}()
	s.isStop = true
}

func (s *HighgoProbe) Start(args interface{}, log *logrus.Logger) error {
	s.log = log
	//s.cfg = args.(HighgoProbeArgs)

	return s.detect()
}

func (s *HighgoProbe) Clean() {
	s.StopProbe()
}

// 判定条件
// 1. highgo 的 pg_is_in_recovery 设置为 false
// 2. 若 strict 为 ON 且 detector 所连接 elector 的 role 为 leader

// 代码逻辑
// 1. 建立 highgo 连接
// 2. 调用 select pg_is_in_recovery()
// 3. 如果 pg_is_in_recovery 为 false 则根据 strict 的值进行判定
//   3.1 如果 strict 为 on ，则向 elector 建立连接
//     3.1.1 如果连接建立失败，则直接判定 detect 失败
//     3.1.2 如果连接建立成功，则判定所连接的 elector 的 role ，如果是 leader 则认为 detect 成功，否则认为 detect 失败
//   3.2 如果 strict 为 off ，则直接判定 detect 失败
// 4. 如果 pg_is_in_recovery 为 true 则直接判定 detect 失败
func (s *HighgoProbe) detect() error {
	s.log.Println("[detector/highgo] --> probe start")
	defer s.log.Println("[detector/highgo] <-- probe done")

	s.log.Println("[detector/highgo]   --> try to connect highgo")

	// NOTE: another format
	/*
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?connect_timeout=%d",
			parser.HighgoSetting.User,
			parser.HighgoSetting.Password,
			parser.HighgoSetting.Target,
			parser.HighgoSetting.Database,
			parser.HighgoSetting.ConnTimeout))
	*/

	db, err := sql.Open("postgres", fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s connect_timeout=%d",
		parser.HighgoSetting.User,
		parser.HighgoSetting.Password,
		strings.Split(parser.HighgoSetting.Target, ":")[0],
		strings.Split(parser.HighgoSetting.Target, ":")[1],
		parser.HighgoSetting.Database,
		parser.HighgoSetting.ConnTimeout))

	if err != nil {
		s.log.Printf("[detector/highgo] connect highgo[%s] failed, %v", parser.HighgoSetting.Target, err)
		return err
	} else {
		s.log.Printf("[detector/highgo] connect highgo[%s] success", parser.HighgoSetting.Target)
	}

	// TODO: is it necessary?
	defer db.Close()

	var value string
	err = db.QueryRow("select pg_is_in_recovery()").Scan(&value)
	if err != nil {
		s.log.Println(err)
		return err
	} else {
		s.log.Printf("[detector/highgo] pg_is_in_recovery => [%s]", value)
	}

	if value == "false" {
		if parser.HighgoSetting.Strict == "ON" {
			s.log.Println("[detector/highgo]     --> try to connect elector (Strict=ON)")

			// FIXME: 连接复用问题
			conn, err := grpc.Dial(
				parser.DetectorSetting.ElectorHost,
				grpc.WithInsecure(),
				grpc.WithBlock(),
				grpc.WithTimeout(time.Second),
			)
			if err != nil {
				s.log.Printf("[detector/highgo] connect elector[%s] failed, err => [%v]", parser.DetectorSetting.ElectorHost, err)
				return errors.New("connect elector failed")
			}
			defer conn.Close()

			s.log.Printf("[detector/highgo] connect elector[%s] success", parser.DetectorSetting.ElectorHost)

			client := pb.NewRoleServiceClient(conn)
			obRsp, err := client.Obtain(context.Background(), &pb.ObtainReq{})

			if err != nil {
				s.log.Infof("[detector/highgo] Obtain role failed: %v", err)
				return errors.New("obtain role from elector failed")
			}

			s.log.Infof("[detector/highgo] role => [%s]", obRsp.GetRole())

			if pb.EnumRole_Leader == obRsp.GetRole() {
				return nil
			} else {
				return errors.New("role of elector is not Leader")
			}
		} else {
			return errors.New("pg_is_in_recovery == false && strict == OFF")
		}
	} else {
		s.log.Println("[detector/highgo] This instance is in 'standby' mode, can not be detected")
		return errors.New("pg_is_in_recovery == true")
	}

	return nil
}
