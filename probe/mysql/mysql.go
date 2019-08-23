package mysql

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/probe"
	"github.com/sirupsen/logrus"

	_ "github.com/go-sql-driver/mysql"
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

func (s *MySQLProbe) Start(args interface{}, log *logrus.Logger) (err error) {
	s.log = log
	//s.cfg = args.(MySQLProbeArgs)

	if ok := s.detect(); ok {
		return nil
	} else {
		return errors.New("detect failed")
	}

	return
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
func (s *MySQLProbe) detect() bool {
	s.log.Println("[detector/mysql] --> probe start")
	defer s.log.Println("[detector/mysql] <-- probe done")

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/?timeout=%ds&charset=utf8&parseTime=True&loc=Local",
		parser.MySQLSetting.User,
		parser.MySQLSetting.Password,
		parser.MySQLSetting.Target,
		parser.MySQLSetting.ConnTimeout))
	if err != nil {
		s.log.Println(err)
		return false
	} else {
		s.log.Printf("[detector/mysql] connect MySQL[%s] success\n", parser.MySQLSetting.Target)
	}

	// TODO: is it necessary?
	defer db.Close()

	var name, value string
	err = db.QueryRow("show global variables like 'read_only'").Scan(&name, &value)
	if err != nil {
		s.log.Println(err)
		return false
	} else {
		s.log.Printf("[detector/mysql] read_only -> [%s]\n", value)
	}

	if value == "OFF" {
		if parser.MySQLSetting.Strict == "ON" {
			s.log.Println("[detector/mysql] try to connect elector")
			if true {
				s.log.Printf("[detector/mysql] connect elector[%s] success", parser.DetectorSetting.ElectorHost)
				if true {
					s.log.Println("[detector/mysql] elector role -> [leader]")
					return true
				} else {
					s.log.Println("[detector/mysql] elector role -> [follower]")
					return false
				}
			} else {
				s.log.Printf("[detector/mysql] connect elector[%s] failed", parser.DetectorSetting.ElectorHost)
				return false
			}
		} else {
			return false
		}
	} else {
		s.log.Println("[detector/mysql] This instance is in 'read_only' mode, can not be detected")
		return false
	}

	return true
}
