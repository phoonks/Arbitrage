package handler

import (
	"net/http"
	"sync"
)

type HttpServer struct {
	server    *http.Server
	port      int16
	isStarted bool
	mtx       *sync.Mutex
}

func NewHttpsServer() *HttpServer {
	return &HttpServer{
		isStarted: false,
		mtx:       &sync.Mutex{},
	}
}
