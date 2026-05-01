package conf

import (
	"time"
)

var SetHttpServer, GetHttpServer = create[HttpServer]()

type HttpServer struct {
	IP           string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}
