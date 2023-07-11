package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/service/xcall"
)

func init() {
	_ = bmc.ServiceName
	_ = xcall.ServiceName
}

const (
	ParamNetwork          = "network"
	ParamTxID             = "txID"
	ParamServiceOrAddress = "serviceOrAddress"
	ParamMethod           = "method"
	ContextAdaptor        = "adaptor"
	ContextService        = "service"
	ContextRequest        = "request"
	GroupUrlApi           = "/api"
	GroupUrlMonitor       = "/monitor"
	WsHandshakeTimeout    = time.Second * 3
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
	u    websocket.Upgrader
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
	s.RegisterMonitorHandler(s.e.Group(GroupUrlMonitor))
	return s.e.Start(s.addr)
}

type Request struct {
	Params  contract.Params  `json:"params" query:"params"`
	Options contract.Options `json:"options" query:"options"`
}

type ContractRequest struct {
	Request
	Spec json.RawMessage `json:"spec,omitempty" query:"spec"`
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

	serviceApi := generalApi.Group("/:" + ParamServiceOrAddress + "/:" + ParamMethod)
	serviceApi.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := &ContractRequest{}
			if err := BindQueryParamsAndUnmarshalBody(c, req); err != nil {
				s.l.Debugf("fail to BindQueryParamsAndUnmarshalBody err:%+v", err)
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

			pm := c.Param(ParamMethod)
			m, found := svc.Spec().Methods[pm]
			if !found {
				return echo.NewHTTPError(http.StatusNotFound,
					fmt.Sprintf("Method(%s) not found", pm))
			}
			hm := c.Request().Method
			if m.Readonly {
				if hm != http.MethodGet {
					return echo.NewHTTPError(http.StatusMethodNotAllowed,
						fmt.Sprintf("HttpMethod(%s) not allowed, use GET", hm))
				}
			} else {
				if hm != http.MethodPost {
					return echo.NewHTTPError(http.StatusNotFound,
						fmt.Sprintf("HttpMethod(%s) not allowed, use POST", hm))
				}
			}
			return next(c)
		}
	})
	serviceApi.POST("", func(c echo.Context) error {
		var (
			txID contract.TxID
			err  error
		)
		req := c.Get(ContextRequest).(*Request)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(ParamNetwork)
		method := c.Param(ParamMethod)
		txID, err = svc.Invoke(network, method, req.Params, req.Options)
		if err != nil {
			s.l.Errorf("fail to Invoke err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, txID)
	})
	serviceApi.GET("", func(c echo.Context) error {
		var (
			ret contract.ReturnValue
			err error
		)
		req := c.Get(ContextRequest).(*Request)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(ParamNetwork)
		method := c.Param(ParamMethod)
		ret, err = svc.Call(network, method, req.Params, req.Options)
		if err != nil {
			s.l.Errorf("fail to Call err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
}

type MonitorRequest struct {
	NameToParams map[string][]contract.Params `json:"nameToParams"`
	Height       int64                        `json:"height"`
}

func (s *Server) wsID(conn *websocket.Conn) string {
	return conn.RemoteAddr().String()
}

func (s *Server) wsConnect(c echo.Context) (*websocket.Conn, error) {
	conn, err := s.u.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.l.Debugf("fail to Upgrade err:%+v", err)
		return nil, err
	}
	s.l.Debugf("[%s]wsConnect", s.wsID(conn))
	return conn, nil
}

func (s *Server) wsHandshake(conn *websocket.Conn, req interface{}, onSuccess func() error) error {
	var err error
	id := s.wsID(conn)
	ctx, cancel := context.WithTimeout(context.Background(), WsHandshakeTimeout)
	defer func() {
		cancel()
		er := &ErrorResponse{
			Code: errors.Success,
		}
		if err != nil {
			er.Code = errors.UnknownError
			er.Message = err.Error()
			if ec, ok := errors.CoderOf(err); ok {
				er.Code = ec.ErrorCode()
			}
		}
		if err = s.wsWrite(conn, er); err != nil {
			s.l.Debugf("[%s]fail to wsWrite err:%+v", id, err)
		}
	}()
	if err = s.wsRead(ctx, conn, req); err != nil {
		s.l.Debugf("[%s]fail to wsRead err:%+v", id, err)
		return err
	}
	err = onSuccess()
	return err
}

func (s *Server) wsClose(conn *websocket.Conn) {
	s.l.Debugf("[%s]wsClose", s.wsID(conn))
	conn.Close()
}

func (s *Server) wsRead(ctx context.Context, conn *websocket.Conn, v interface{}) error {
	id := s.wsID(conn)
	ch := make(chan interface{}, 1)
	go func() {
		_, b, err := conn.ReadMessage()
		if err != nil {
			ch <- err
		} else {
			ch <- b
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case inf := <-ch:
		switch t := inf.(type) {
		case error:
			return t
		case []byte:
			if err := json.Unmarshal(t, v); err != nil {
				return err
			}
			s.l.Logf(s.lv, "[%s]wsRead=%s", id, t)
			return nil
		default:
			s.l.Panicln("unreachable code")
			return nil
		}
	}
}

func (s *Server) wsWrite(conn *websocket.Conn, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.l.Logf(s.lv, "[%s]wsWrite=%s", s.wsID(conn), b)
	return conn.WriteMessage(websocket.TextMessage, b)
}

func (s *Server) wsReadLoop(ctx context.Context, conn *websocket.Conn, cb func(b []byte) error) error {
	id := s.wsID(conn)
	ech := make(chan error, 1)
	go func() {
		defer func() {
			s.l.Debugf("[%s]wsReadLoop finish", id)
		}()
		for {
			_, b, err := conn.ReadMessage()
			if err != nil {
				ech <- err
				break
			}
			s.l.Logf(s.lv, "[%s]wsReadLoop=%s", id, b)
			if err = cb(b); err != nil {
				ech <- err
				break
			}
		}
	}()

	select {
	case <-ctx.Done():
		s.l.Debugf("[%s]wsReadLoop context Done", id)
		return ctx.Err()
	case err := <-ech:
		s.l.Debugf("[%s]wsReadLoop err:%+v", id, err)
		return err
	}
}

func (s *Server) RegisterMonitorHandler(g *echo.Group) {
	monitorApi := g.Group("/:"+ParamNetwork+"/:"+ParamServiceOrAddress,
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				p := c.Param(ParamServiceOrAddress)
				svc := s.GetService(p)
				if svc == nil {
					network, address := c.Param(ParamNetwork), contract.Address(p)
					if svc = s.GetService(ContractServiceName(network, address)); svc == nil {
						return echo.NewHTTPError(http.StatusNotFound,
							fmt.Sprintf("Service(%s) not found", p))
					}
				}
				c.Set(ContextService, svc)
				return next(c)
			}
		})
	monitorApi.GET("/event", func(c echo.Context) error {
		conn, err := s.wsConnect(c)
		if err != nil {
			return err
		}
		defer s.wsClose(conn)
		id := s.wsID(conn)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(ParamNetwork)
		var efs []contract.EventFilter
		req := &MonitorRequest{}
		onSuccessHandshake := func() error {
			if err = c.Validate(req); err != nil {
				s.l.Debugf("[%s]fail to Validate err:%+v", id, err)
				return err
			}
			if efs, err = svc.EventFilters(network, req.NameToParams); err != nil {
				s.l.Debugf("[%s]fail to EventFilters err:%+v", id, err)
				return err
			}
			return nil
		}
		if err = s.wsHandshake(conn, req, onSuccessHandshake); err != nil {
			s.l.Debugf("[%s]fail to wsHandshake err:%+v", id, err)
			return nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			_ = s.wsReadLoop(ctx, conn, func(b []byte) error {
				return nil
			})
		}()
		onEvent := func(e contract.Event) error {
			return s.wsWrite(conn, e)
		}
		if err = svc.MonitorEvent(ctx, network, onEvent, efs, req.Height); err != nil {
			s.l.Debugf("[%s]fail to MonitorEvent req:%+v err:%+v", id, req, err)
			return nil
		}
		return nil
	})
}

func (s *Server) Stop() error {
	s.l.Infoln("shutting down the server")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return s.e.Shutdown(ctx)
}

func BindQueryParamsAndUnmarshalBody(c echo.Context, v interface{}) error {
	if ContainsMapTypeInStructType(reflect.TypeOf(v)) {
		if err := UnmarshalQueryParams(c, v); err != nil {
			return err
		}
	} else {
		if err := c.Bind(v); err != nil && err != echo.ErrUnsupportedMediaType {
			return err
		}
	}
	return UnmarshalRequestBody(c, v)
}

func QueryParamsToMap(c echo.Context) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for k, v := range c.QueryParams() {
		tm := m
		if start := strings.IndexByte(k, '['); start > 0 && k[len(k)-1] == ']' {
			l := []string{k[:start]}
			l = append(l, strings.Split(k[start+1:len(k)-1], "][")...)
			var (
				elem interface{}
				ok   = false
				last = len(l) - 1
			)
			for i, p := range l {
				if i < last {
					if elem, ok = tm[p]; !ok {
						cm := make(map[string]interface{})
						tm[p] = cm
						tm = cm
					} else if tm, ok = elem.(map[string]interface{}); ok {
						continue
					} else {
						return nil, errors.Errorf("fail cast k:%s i:%d p:%s", k, i, p)
					}
				} else {
					k = p
				}
			}
		}
		switch len(v) {
		case 0:
			tm[k] = nil
		case 1:
			tm[k] = v[0]
		default:
			tm[k] = v
		}
	}
	return m, nil
}

func ContainsMapTypeInStructType(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Type.Kind() == reflect.Map {
				return true
			} else if t.Field(i).Type.Kind() == reflect.Struct {
				if ContainsMapTypeInStructType(t.Field(i).Type) {
					return true
				}
			}
		}
	}
	return false
}

func UnmarshalQueryParams(c echo.Context, v interface{}) error {
	m, err := QueryParamsToMap(c)
	if err != nil {
		return err
	}
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func UnmarshalRequestBody(c echo.Context, v interface{}) error {
	if c.Request().ContentLength == 0 {
		return nil
	}
	return UnmarshalBody(c.Request().Body, v)
}

func UnmarshalBody(b io.ReadCloser, v interface{}) error {
	defer b.Close()
	if err := json.NewDecoder(b).Decode(v); err != nil {
		return err
	}
	return nil
}
