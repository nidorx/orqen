package service

import (
	"log"
	"sync"

	"github.com/nidorx/orqen/pkg/service/engine"
	httpservice "github.com/nidorx/orqen/pkg/service/http"
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

		// Build service list with only non-nil services
		_services = []Service{
			engine.New(),
			httpservice.New(),
		}

		for _, s := range _services {
			if err := s.OnStart(); err != nil {
				log.Fatalf("[service] start %s service error: %s", s.Name(), err)
			}
		}
	})

	return _services
}

// Stop stop all services
func Stop() {
	_onceStop.Do(func() {
		for index := len(_services) - 1; index >= 0; index-- {
			if _services[index] == nil {
				continue
			}
			if err := _services[index].OnStop(); err != nil {
				log.Fatalf("[service] stop %s service error: %s", _services[index].Name(), err)
			}
		}
	})
}
