package servitization

import (
	"bytes"
	"strings"

	"github.com/sirupsen/logrus"
)

type customLogFormat struct {
	logrus.JSONFormatter
}

func (f *customLogFormat) Format(entry *logrus.Entry) ([]byte, error) {
	json_out, err := f.JSONFormatter.Format(entry)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	b.WriteByte('[')
	b.WriteString(strings.TrimRight(string(json_out), "\n"))
	b.WriteByte(']')
	b.WriteByte('\n')

	return b.Bytes(), nil
}

// 定制 logrus 日志格式
/*
	logrus.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "_timestamp",
			logrus.FieldKeyLevel: "_level",
			logrus.FieldKeyMsg:   "message",
		},
	})
*/
