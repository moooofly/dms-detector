package router

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/moooofly/dms-detector/pkg/parser"
	"github.com/moooofly/dms-detector/pkg/servitization"
	"github.com/moooofly/dms-detector/probe"
	"github.com/sirupsen/logrus"
)

func Launch() error {
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", parser.DetectorSetting.Port),
		Handler: initRouter(),
		//ReadTimeout:    parser.ReadTimeout,
		//WriteTimeout:   parser.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	if err := s.ListenAndServe(); err != nil {
		logrus.Infof("ListenAndServe faled, %v", err)
		return err
	}
	return nil
}

func initRouter() *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	//gin.SetMode(parser.ServerSetting.RunMode)

	r.HEAD("/", headCb)

	return r
}

func headCb(c *gin.Context) {
	prober := servitization.Prober
	logrus.Infof("probe [%s] triggered by HaProxy HEAD request.", prober)
	_, err := probe.Run(prober, nil)
	if err != nil {
		logrus.Infof("probe [%s] %s", prober, err)
		c.String(http.StatusServiceUnavailable, "")
	} else {
		logrus.Infof("probe [%s] success", prober)
		c.String(http.StatusOK, "")
	}
	// self.send_header('Content-type', 'text/html')
}
