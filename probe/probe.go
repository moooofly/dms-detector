package probe

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/sirupsen/logrus"
)

type Probe interface {
	Start(args interface{}, log *logrus.Logger) (err error)
	Clean()
}
type ProbeItem struct {
	S    Probe
	Args interface{}
	Name string
	Log  *logrus.Logger
}

var probesMap = sync.Map{}

func Regist(name string, s Probe, args interface{}, log *logrus.Logger) {
	Stop(name)
	probesMap.Store(name, &ProbeItem{
		S:    s,
		Args: args,
		Name: name,
		Log:  log,
	})
}
func GetProbeItem(name string) *ProbeItem {
	if s, ok := probesMap.Load(name); ok && s.(*ProbeItem).S != nil {
		return s.(*ProbeItem)
	}
	return nil

}
func Stop(name string) {
	if s, ok := probesMap.Load(name); ok && s.(*ProbeItem).S != nil {
		s.(*ProbeItem).S.Clean()
		probesMap.Delete(name)
	}
}
func Run(name string, args interface{}) (probe *ProbeItem, err error) {
	_service, ok := probesMap.Load(name)
	if ok {
		defer func() {
			e := recover()
			if e != nil {
				err = fmt.Errorf("crashed, reason: %s\ntrace: %s", e, string(debug.Stack()))
			}
		}()
		probe = _service.(*ProbeItem)
		if args != nil {
			err = probe.S.Start(args, probe.Log)
		} else {
			err = probe.S.Start(probe.Args, probe.Log)
		}
		if err != nil {
			err = fmt.Errorf("failed, reason: %s", err)
		}
	} else {
		err = fmt.Errorf("not found")
	}
	return
}
