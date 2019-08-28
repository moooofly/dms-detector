package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/probe"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	_ "github.com/go-sql-driver/mysql"

	pb "github.com/moooofly/dms-detector/proto"
)

type MySQLProbeArgs struct {
}

type MySQLProbe struct {
	cfg    MySQLProbeArgs
	log    *logrus.Logger
	isStop bool
}

func NewMySQLProbe() probe.Probe {
	return &MySQLProbe{
		cfg:    MySQLProbeArgs{},
		log:    nil,
		isStop: false,
	}
}

func (s *MySQLProbe) StopProbe() {
	defer func() {
		e := recover()
		if e != nil {
			s.log.Printf("Stop MySQLProbe crashed, %s", e)
		} else {
			s.log.Printf("probe MySQLProbe stopped")
		}
		s.cfg = MySQLProbeArgs{}
		s.log = nil
		s = nil
	}()
	s.isStop = true
}

func (s *MySQLProbe) Start(args interface{}, log *logrus.Logger) error {
	s.log = log
	//s.cfg = args.(MySQLProbeArgs)

	return s.detect()
}

func (s *MySQLProbe) Clean() {
	s.StopProbe()
}

// 判定条件
// 1. MySQL 的 readonly 设置为 OFF
// 2. 若 strict 为 ON 且 detector 所连接 elector 的 role 为 leader

// 代码逻辑
// 1. 建立 mysql 连接
// 2. show global variables like \'read_only\'
// 3. 如果 read_only 为 off 则根据 strict 的值进行判定
//   3.1 如果 strict 为 on ，则向 elector 建立连接
//     3.1.1 如果连接建立失败，则直接判定 detect 失败
//     3.1.2 如果连接建立成功，则判定所连接的 elector 的 role ，如果是 leader 则认为 detect 成功，否则认为 detect 失败
//   3.2 如果 strict 为 off ，则直接判定 detect 失败
// 4. 如果 read_only 为 on 则直接判定 detect 失败
func (s *MySQLProbe) detect() error {
	s.log.Println("[detector/mysql] --> probe start")
	defer s.log.Println("[detector/mysql] <-- probe done")

	s.log.Println("[detector/mysql]   --> try to connect MySQL")

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/?timeout=%ds&charset=utf8&parseTime=True&loc=Local",
		parser.MySQLSetting.User,
		parser.MySQLSetting.Password,
		parser.MySQLSetting.Target,
		parser.MySQLSetting.ConnTimeout))
	if err != nil {
		s.log.Println(err)
		return err
	} else {
		s.log.Printf("[detector/mysql] connect MySQL[%s] success\n", parser.MySQLSetting.Target)
	}

	// TODO: is it necessary?
	defer db.Close()

	var name, value string
	err = db.QueryRow("show global variables like 'read_only'").Scan(&name, &value)
	if err != nil {
		s.log.Println(err)
		return err
	} else {
		s.log.Printf("[detector/mysql] read_only => [%s]\n", value)
	}

	if value == "OFF" {
		if parser.MySQLSetting.Strict == "ON" {
			s.log.Println("[detector/mysql]     --> try to connect elector (Strict=ON)")

			// TODO: 连接复用问题
			conn, err := grpc.Dial(
				parser.DetectorSetting.ElectorHost,
				grpc.WithInsecure(),
				grpc.WithBlock(),
				grpc.WithTimeout(time.Second),
			)
			if err != nil {
				s.log.Printf("[detector/mysql] connect elector[%s] failed, err => [%v]", parser.DetectorSetting.ElectorHost, err)
				return errors.New("connect elector failed")
			}
			defer conn.Close()

			s.log.Printf("[detector/mysql] connect elector[%s] success", parser.DetectorSetting.ElectorHost)

			client := pb.NewRoleServiceClient(conn)
			obRsp, err := client.Obtain(context.Background(), &pb.ObtainReq{})

			if err != nil {
				s.log.Infof("[detector/mysql] Obtain role failed: %v", err)
				return errors.New("obtain role from elector failed")
			}

			s.log.Infof("[detector/mysql] role => [%s]", obRsp.GetRole())

			if pb.EnumRole_Leader == obRsp.GetRole() {
				return nil
			} else {
				return errors.New("role of elector is not Leader")
			}
		} else {
			return errors.New("read_only == OFF && strict == OFF")
		}
	} else {
		s.log.Println("[detector/mysql] This instance is in 'read_only' mode, can not be detected")
		return errors.New("read_only == ON")
	}

	return nil
}
