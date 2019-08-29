package redis

import (
	"context"
	"errors"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/probe"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	pb "github.com/moooofly/dms-detector/proto"
)

type RedisProbeArgs struct {
}

type RedisProbe struct {
	cfg    RedisProbeArgs
	log    *logrus.Logger
	isStop bool
}

func NewRedisProbe() probe.Probe {
	return &RedisProbe{
		cfg:    RedisProbeArgs{},
		log:    nil,
		isStop: false,
	}
}

func (s *RedisProbe) StopProbe() {
	defer func() {
		e := recover()
		if e != nil {
			s.log.Printf("Stop RedisProbe crashed, %s", e)
		} else {
			s.log.Printf("probe RedisProbe stopped")
		}
		s.cfg = RedisProbeArgs{}
		s.log = nil
		s = nil
	}()
	s.isStop = true
}

func (s *RedisProbe) Start(args interface{}, log *logrus.Logger) error {
	s.log = log
	//s.cfg = args.(RedisProbeArgs)

	return s.detect()
}

func (s *RedisProbe) Clean() {
	s.StopProbe()
}

// 判定条件
// 1. Redis 的 replication role 为 master
// 2. 若 strict 为 ON 且 detector 所连接 elector 的 role 为 leader

// 代码逻辑
// 1. 建立 redis 连接
// 2. 查看 INFO replication 中的 role
// 3. 如果 role 为 master 则根据 strict 的值进行判定
//   3.1 如果 strict 为 on ，则向 elector 建立连接
//     3.1.1 如果连接建立失败，则直接判定 detect 失败
//     3.1.2 如果连接建立成功，则判定所连接的 elector 的 role ，如果是 leader 则认为 detect 成功，否则认为 detect 失败
//   3.2 如果 strict 为 off ，则直接判定 detect 失败
// 4. 如果 role 为 slave ，则直接判定 detect 失败
func (s *RedisProbe) detect() error {
	s.log.Println("[detector/redis] --> probe start")
	defer s.log.Println("[detector/redis] <-- probe done")

	s.log.Println("[detector/redis]   --> try to connect Redis")

	c, err := redis.Dial("tcp", parser.RedisSetting.Target)
	if err != nil {
		s.log.Println("[detector/redis]", err)
		return err
	} else {
		s.log.Printf("[detector/redis] connect Redis[%s] success\n", parser.RedisSetting.Target)
	}

	if parser.RedisSetting.Password != "" {
		if _, err := c.Do("AUTH", parser.RedisSetting.Password); err != nil {
			c.Close()
			s.log.Println("[detector/redis]", err)
			return err
		}
	}
	defer c.Close()

	if isMaster(c) {
		s.log.Println("[detector/redis] redis role => [master]")

		if parser.RedisSetting.Strict == "ON" {
			s.log.Println("[detector/redis]     --> try to connect elector (Strict=ON)")

			// TODO: 连接复用问题
			conn, err := grpc.Dial(
				parser.DetectorSetting.ElectorHost,
				grpc.WithInsecure(),
				grpc.WithBlock(),
				grpc.WithTimeout(time.Second),
			)
			if err != nil {
				s.log.Printf("[detector/redis] connect elector[%s] failed, err => [%v]", parser.DetectorSetting.ElectorHost, err)
				return errors.New("connect elector failed")
			}
			defer conn.Close()

			s.log.Printf("[detector/redis] connect elector[%s] success", parser.DetectorSetting.ElectorHost)

			client := pb.NewRoleServiceClient(conn)
			obRsp, err := client.Obtain(context.Background(), &pb.ObtainReq{})

			if err != nil {
				s.log.Infof("[detector/redis] Obtain role failed: %v", err)
				return errors.New("obtain role from elector failed")
			}

			s.log.Infof("[detector/redis] role => [%s]", obRsp.GetRole())

			if pb.EnumRole_Leader == obRsp.GetRole() {
				return nil
			} else {
				return errors.New("role of elector is not Leader")
			}
		} else {
			return errors.New("redis role == master && strict == OFF")
		}
	} else {
		s.log.Println("[detector/redis] redis role => [slave], can not be detected")
		return errors.New("redis role == slave")
	}
}

func isMaster(conn redis.Conn) bool {
	role, err := getRole(conn)
	if err != nil || role != "master" {
		return false
	}
	return true
}

// getRole is a convenience function supplied to query an instance (master or
// slave) for its role. It attempts to use the ROLE command introduced in
// redis 2.8.12.
func getRole(c redis.Conn) (string, error) {
	res, err := c.Do("ROLE")
	if err != nil {
		return "", err
	}
	rres, ok := res.([]interface{})
	if ok {
		return redis.String(rres[0], nil)
	}
	return "", errors.New("redigo: can not transform ROLE reply to string")
}
