package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/icon-project/btp2/common/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
)

const (
	ParamNetwork          = "network"
	ParamTxID             = "txID"
	ParamServiceOrAddress = "serviceOrAddress"
	ContextAdaptor        = "adaptor"
	ContextService        = "service"
	ContextRequest        = "request"
	GroupUrlApi           = "/api"
)

func Logger(l log.Logger) log.Logger {
	return l.WithFields(log.Fields{log.FieldKeyModule: "api"})
}

type Server struct {
	e    *echo.Echo
	addr string
	aMap map[string]contract.Adaptor
	sMap map[string]service.Service
	mtx  sync.RWMutex
	lv   log.Level
	l    log.Logger
}

func NewServer(addr string, transportLogLevel log.Level, l log.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = NewValidator()
	e.HTTPErrorHandler = HttpErrorHandler
	return &Server{
		e:    e,
		addr: addr,
		aMap: make(map[string]contract.Adaptor),
		sMap: make(map[string]service.Service),
		lv:   contract.EnsureTransportLogLevel(transportLogLevel),
		l:    Logger(l),
	}
}

func (s *Server) AddAdaptor(network string, a contract.Adaptor) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.aMap[network] = a
}

func (s *Server) GetAdaptor(network string) contract.Adaptor {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.aMap[network]
}

func (s *Server) AddService(svc service.Service) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.sMap[svc.Name()] = svc
}

func (s *Server) GetService(name string) service.Service {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.sMap[name]
}

func (s *Server) Start() error {
	s.l.Infoln("starting the server")
	// CORS middleware
	s.e.Use(
		middleware.CORSWithConfig(middleware.CORSConfig{
			MaxAge: 3600,
		}),
		middleware.Recover())
	s.RegisterAPIHandler(s.e.Group(GroupUrlApi))
	return s.e.Start(s.addr)
}

type Request struct {
	Method  string           `json:"method" validate:"required"`
	Params  contract.Params  `json:"params"`
	Options contract.Options `json:"options"`
}

type ContractRequest struct {
	Request
	Spec json.RawMessage `json:"spec,omitempty"`
}

func (s *Server) RegisterAPIHandler(g *echo.Group) {
	generalApi := g.Group("/:" + ParamNetwork)
	generalApi.Use(middleware.BodyDump(func(c echo.Context, reqBody []byte, resBody []byte) {
		s.l.Debugf("url=%s", c.Request().RequestURI)
		s.l.Logf(s.lv, "request=%s", reqBody)
		s.l.Logf(s.lv, "response=%s", resBody)
	}), func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p := c.Param(ParamNetwork)
			a := s.GetAdaptor(p)
			if a == nil {
				return echo.NewHTTPError(http.StatusNotFound,
					fmt.Sprintf("Network(%s) not found", p))
			}
			c.Set(ContextAdaptor, a)
			return next(c)
		}
	})
	generalApi.GET("/result/:"+ParamTxID, func(c echo.Context) error {
		a := c.Get(ContextAdaptor).(contract.Adaptor)
		p := c.Param(ParamTxID)
		ret, err := a.GetResult(p)
		if err != nil {
			s.l.Debugf("fail to GetResult err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})

	serviceApi := generalApi.Group("/:" + ParamServiceOrAddress)
	serviceApi.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := &ContractRequest{}
			if err := BindOrUnmarshalBody(c, req); err != nil {
				s.l.Debugf("fail to BindOrUnmarshalBody err:%+v", err)
				return echo.ErrBadRequest
			}
			if err := c.Validate(req); err != nil {
				s.l.Debugf("fail to Validate err:%+v", err)
				return err
			}
			c.Set(ContextRequest, &req.Request)

			p := c.Param(ParamServiceOrAddress)
			svc := s.GetService(p)
			if svc == nil {
				network, address := c.Param(ParamNetwork), contract.Address(p)
				if len(req.Spec) > 0 {
					a := c.Get(ContextAdaptor).(contract.Adaptor)
					var err error
					if svc, err = NewContractService(a, req.Spec, address, network, s.l); err != nil {
						s.l.Debugf("fail to NewContractService err:%+v", err)
						return err
					}
					s.AddService(svc)
				} else if svc = s.GetService(ContractServiceName(network, address)); svc == nil {
					return echo.NewHTTPError(http.StatusNotFound,
						fmt.Sprintf("Service(%s) not found", p))
				}
			}
			c.Set(ContextService, svc)
			return next(c)
		}
	})
	serviceApi.POST("/invoke", func(c echo.Context) error {
		var (
			txID contract.TxID
			err  error
		)
		req := c.Get(ContextRequest).(*Request)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(ParamNetwork)
		txID, err = svc.Invoke(network, req.Method, req.Params, req.Options)
		if err != nil {
			s.l.Errorf("fail to Invoke err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, txID)
	})
	serviceApi.GET("/call", func(c echo.Context) error {
		var (
			ret contract.ReturnValue
			err error
		)
		req := c.Get(ContextRequest).(*Request)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(ParamNetwork)
		ret, err = svc.Call(network, req.Method, req.Params, req.Options)
		if err != nil {
			s.l.Errorf("fail to Call err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
}

func (s *Server) Stop() error {
	s.l.Infoln("shutting down the server")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return s.e.Shutdown(ctx)
}

func BindOrUnmarshalBody(c echo.Context, v interface{}) error {
	if err := c.Bind(v); err != nil {
		if c.Request().ContentLength > 0 {
			return UnmarshalBody(c.Request().Body, v)
		}
		return err
	}
	return nil
}

func UnmarshalBody(b io.ReadCloser, v interface{}) error {
	defer b.Close()
	if err := json.NewDecoder(b).Decode(v); err != nil {
		return err
	}
	return nil
}
