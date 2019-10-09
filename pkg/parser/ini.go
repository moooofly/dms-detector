package parser

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

var (
	cfg *ini.File

	DetectorSetting = &detector{}
	MySQLSetting    = &mysql{}
	RedisSetting    = &redis{}
	RedisNmsSetting = &redisNms{}
	ZkSetting       = &zk{}
	RadarSetting    = &radar{}
)

// [detector] section in .ini
type detector struct {
	Port        int    `ini:"port"`
	ElectorHost string `ini:"elector-host"`
	LogPath     string `ini:"log-path"`
	LogLevel    string `ini:"log-level"`
}

// [mysql] section in .ini
type mysql struct {
	Target      string `ini:"target"`
	User        string `ini:"user"`
	Password    string `ini:"password"`
	ConnTimeout int    `ini:"connect-timeout"`
	Strict      string `ini:"strict"`
}

// [redis] section in .ini
type redis struct {
	Target   string `ini:"target"`
	Password string `ini:"password"`
	Strict   string `int:"strict"`
}

// [redis_nms] section in .ini
type redisNms struct {
	Target   string `ini:"target"`
	Password string `ini:"password"`
	Strict   string `int:"strict"`
}

// [zookeeper] section in .ini
type zk struct {
	Target string `ini:"target"`
}

// [radar_server] section in .ini
type radar struct {
	Target string `ini:"target"`
}

func Load(prober string) {
	// TODO: 路径问题
	var err error
	cfg, err = ini.Load(fmt.Sprintf("conf/detector.%s.ini", prober))
	if err != nil {
		logrus.Fatalf("Fail to parse 'conf/detector.%s.ini': %v", prober, err)
	}

	mapTo("detector", DetectorSetting)

	switch prober {
	case "mysql":
		mapTo(prober, MySQLSetting)
	case "redis":
		mapTo(prober, RedisSetting)
	case "redis_nms":
		mapTo(prober, RedisNmsSetting)
	case "radar":
		mapTo(prober, RadarSetting)
	case "zookeeper":
		mapTo(prober, ZkSetting)
	default:
		logrus.Fatal("not match any of [mysql|redis|redis_nms|radar_server|zookeeper].")
	}
}

func mapTo(section string, v interface{}) {
	err := cfg.Section(section).MapTo(v)
	if err != nil {
		logrus.Fatalf("mapto err: %v", err)
	}
}
