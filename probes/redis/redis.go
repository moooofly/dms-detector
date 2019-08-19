package redis

import (
	"errors"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"gitlab.com/kedacom-dms/detector-go/probes"
	"gitlab.com/kedacom-dms/detector-go/util/setting"
)

type RedisProbeArgs struct {
}

type RedisProbe struct {
	cfg    RedisProbeArgs
	log    *logrus.Logger
	isStop bool
}

func NewRedisProbe() probes.Probe {
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

func (s *RedisProbe) Start(args interface{}, log *logrus.Logger) (err error) {
	s.log = log
	//s.cfg = args.(RedisProbeArgs)

	if ok := s.detect(); ok {
		return nil
	} else {
		return errors.New("detect failed")
	}

	return
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
func (s *RedisProbe) detect() bool {
	s.log.Println("[detector/redis] --> probe start")
	defer s.log.Println("[detector/redis] <-- probe done")

	c, err := redis.Dial("tcp", setting.RedisSetting.Target)
	if err != nil {
		s.log.Println("[detector/redis]", err)
		return false
	} else {
		s.log.Printf("[detector/redis] connect Redis[%s] success\n", setting.RedisSetting.Target)
	}

	if setting.RedisSetting.Password != "" {
		if _, err := c.Do("AUTH", setting.RedisSetting.Password); err != nil {
			c.Close()
			s.log.Println("[detector/redis]", err)
			return false
		}
	}
	defer c.Close()

	if isMaster(c) {
		s.log.Println("[detector/redis] redis role -> [master]")
		if setting.RedisSetting.Strict == "ON" {
			s.log.Println("[detector/redis] try to connect elector")
			if true {
				s.log.Printf("[detector/redis] connect elector[%s] success", setting.DetectorSetting.ElectorHost)
				if true {
					s.log.Println("[detector/redis] elector role -> [leader]")
					return true
				} else {
					s.log.Println("[detector/redis] elector role -> [follower]")
					return false
				}
			} else {
				s.log.Printf("[detector/redis] connect elector[%s] failed", setting.DetectorSetting.ElectorHost)
				return false
			}
		} else {
			return false
		}
	} else {
		s.log.Println("[detector/redis] redis role -> [non-master]")
	}

	return true
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
