package service

import (
	"log"
	"sync"

	"github.com/nidorx/orqen/pkg/service/http"
	"github.com/nidorx/orqen/pkg/service/project"
)

var (
	_services  []Service
	_onceStart sync.Once
	_onceStop  sync.Once
)

type Service interface {
	Name() string
	OnStart() error
	OnStop() error
}

// Start start all services
func Start() []Service {
	_onceStart.Do(func() {

		_services = []Service{
			project.New(),
			http.New(),
		}

		for _, service := range _services {
			if err := service.OnStart(); err != nil {
				log.Fatalf("start %s service error: %s", service.Name(), err)
			}
		}
	})

	return _services
}

// Stop stop all services
func Stop() {
	_onceStop.Do(func() {
		for index := len(_services) - 1; index >= 0; index-- {
			if err := _services[index].OnStop(); err != nil {
				log.Fatalf("start %s service error: %s", _services[index].Name(), err)
			}
		}
	})
}
