package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/icon-project/btp-sdk/btptracker/storage/repository"
	"github.com/icon-project/btp-sdk/utils"
	"gorm.io/gorm"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/invopop/yaml"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/icon-project/btp-sdk/autocaller"
	_ "github.com/icon-project/btp-sdk/autocaller/xcall"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/service/dappsample"
	"github.com/icon-project/btp-sdk/service/xcall"
	"github.com/icon-project/btp-sdk/web"
)

func init() {
	_ = bmc.ServiceName
	_ = xcall.ServiceName
	_ = dappsample.ServiceName
}

const (
	PathParamNetwork          = "network"
	PathParamTxID             = "txID"
	PathParamBlockID          = "blockID"
	PathParamServiceOrAddress = "serviceOrAddress"
	PathParamMethod           = "method"
	PathParamService          = "service"
	PathParamId               = "id"

	QueryParamService = "service"
	QueryParamHeight  = "height"

	ContextAdaptor = "adaptor"
	ContextService = "service"
	ContextRequest = "request"

	GroupUrlApi        = "/api"
	GroupUrlMonitor    = "/monitor"
	GroupUrlApiDocs    = "/api-docs"
	GroupUrlAutoCaller = "/autocaller"
	GroupUrlWeb        = "/web"
	GroupUrlTracker    = "/tracker"

	UrlGetResult    = "/result"
	UrlGetFinality  = "/finality"
	UrlMonitorEvent = "/event"

	WsHandshakeTimeout       = time.Second * 3
	DefaultWsPingIntervalSec = 30
)

func Logger(l log.Logger) log.Logger {
	return l.WithFields(log.Fields{log.FieldKeyModule: "api"})
}

type ServerConfig struct {
	Address           string               `json:"address"`
	TransportLogLevel contract.LogLevel    `json:"transport_log_level,omitempty"`
	PingIntervalSec   int                  `json:"ping_interval_sec,omitempty"`
	Storage           *utils.StorageConfig `json:"storage,omitempty"`
}

type Server struct {
	e    *echo.Echo
	db   *gorm.DB
	cfg  ServerConfig
	aMap map[string]contract.Adaptor
	sMap map[string]service.Service
	cMap map[string]autocaller.AutoCaller
	mtx  sync.RWMutex
	oasp *OpenAPISpecProvider
	u    websocket.Upgrader
	l    log.Logger

	Signers map[string]service.Signer //FIXME [TBD] signer management
}

func NewServer(cfg ServerConfig, l log.Logger) (*Server, error) {
	if len(cfg.Address) == 0 {
		return nil, errors.Errorf("require address")
	}
	cfg.TransportLogLevel = contract.LogLevel(contract.EnsureTransportLogLevel(cfg.TransportLogLevel.Level()))
	if cfg.PingIntervalSec == 0 {
		cfg.PingIntervalSec = DefaultWsPingIntervalSec
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = NewValidator()
	e.HTTPErrorHandler = HttpErrorHandler

	db, _ := utils.NewStorage(cfg.Storage)

	sl := Logger(l)
	return &Server{
		e:       e,
		db:      db,
		cfg:     cfg,
		aMap:    make(map[string]contract.Adaptor),
		sMap:    make(map[string]service.Service),
		cMap:    make(map[string]autocaller.AutoCaller),
		oasp:    NewOpenAPISpecProvider(sl),
		l:       sl,
		Signers: make(map[string]service.Signer),
	}, nil
}

func (s *Server) SetAdaptor(network string, a contract.Adaptor) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if _, ok := s.aMap[network]; ok {
		s.l.Debugf("overwrite adaptor network:%s", network)
	}
	s.aMap[network] = a
	s.oasp.PutNetworkToNetworkType(network, a.NetworkType())
	s.l.Debugf("SetAdaptor network:%s", network)
}

func (s *Server) GetAdaptor(network string) contract.Adaptor {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.aMap[network]
}

func (s *Server) SetService(svc service.Service) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.sMap[svc.Name()] = svc
	s.oasp.Merge(svc)
	s.l.Debugf("SetService %s", svc.Name())
}

func (s *Server) GetService(name string) service.Service {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.sMap[name]
}

func (s *Server) SetAutoCaller(c autocaller.AutoCaller) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.cMap[c.Name()] = c
	s.l.Debugf("SetAutoCaller %s", c.Name())
}

func (s *Server) GetAutoCaller(name string) autocaller.AutoCaller {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.cMap[name]
}

func (s *Server) Start() error {
	s.l.Infoln("starting the server")
	// CORS middleware
	s.e.Use(
		middleware.CORSWithConfig(middleware.CORSConfig{
			MaxAge: 3600,
		}),
		middleware.Recover())
	//s.RegisterAPIHandler(s.e.Group(GroupUrlApi))
	//s.RegisterAPIDocHandler(s.e.Group(GroupUrlApiDocs))
	//s.RegisterMonitorHandler(s.e.Group(GroupUrlMonitor))
	//s.RegisterAutoCallerHandler(s.e.Group(GroupUrlAutoCaller))
	s.RegisterTrackerHandler(s.e.Group(GroupUrlTracker))
	web.RegisterWebHandler(s.e.Group(GroupUrlWeb))
	return s.e.Start(s.cfg.Address)
}

type NetworkInfo struct {
	Network     string `json:"network"`
	NetworkType string `json:"type"`
}
type NetworkInfos []NetworkInfo

type RegisterContractServiceRequest struct {
	Address contract.Address `json:"address"`
	Spec    json.RawMessage  `json:"spec"`
}

type ServiceInfo struct {
	Name string `json:"name"`
}
type ServiceInfos []ServiceInfo

type MethodInfo struct {
	NetworkTypes []string            `json:"networkTypes"`
	Name         string              `json:"name"`
	Inputs       map[string]TypeInfo `json:"inputs"`
	Output       TypeInfo            `json:"output"`
	Payable      bool                `json:"payable"`
	Readonly     bool                `json:"readonly"`
}
type MethodInfos []MethodInfo

type NameAndTypeInfos map[string]TypeInfo

func NewNameAndTypeInfos(m map[string]*contract.NameAndTypeSpec) NameAndTypeInfos {
	r := make(NameAndTypeInfos)
	for _, s := range m {
		r[s.Name] = NewTypeInfo(s.Type)
	}
	return r
}

type TypeInfo struct {
	Type      string              `json:"type"`
	Dimension int                 `json:"dimension,omitempty"`
	Fields    map[string]TypeInfo `json:"fields,omitempty"`
}

func NewTypeInfo(s contract.TypeSpec) TypeInfo {
	r := TypeInfo{
		Type:      s.Name,
		Dimension: s.Dimension,
	}
	if s.Resolved != nil {
		r.Fields = NewNameAndTypeInfos(s.Resolved.FieldMap)
	}
	return r
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
	g.Use(middleware.BodyDump(func(c echo.Context, reqBody []byte, resBody []byte) {
		s.l.Debugf("url=%s", c.Request().RequestURI)
		s.l.Logf(s.cfg.TransportLogLevel.Level(), "request=%s", reqBody)
		s.l.Logf(s.cfg.TransportLogLevel.Level(), "response=%s", resBody)
	}))
	g.GET("", func(c echo.Context) error {
		s.mtx.RLock()
		defer s.mtx.RUnlock()
		r := make(NetworkInfos, 0)
		for n, a := range s.aMap {
			r = append(r, NetworkInfo{Network: n, NetworkType: a.NetworkType()})
		}
		return c.JSON(http.StatusOK, r)
	})
	networkApi := g.Group("/:" + PathParamNetwork)
	networkApi.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p := c.Param(PathParamNetwork)
			a := s.GetAdaptor(p)
			if a == nil {
				return echo.NewHTTPError(http.StatusNotFound,
					fmt.Sprintf("Network(%s) not found", p))
			}
			c.Set(ContextAdaptor, a)
			return next(c)
		}
	})
	networkApi.GET("", func(c echo.Context) error {
		s.mtx.RLock()
		defer s.mtx.RUnlock()
		r := make(ServiceInfos, 0)
		for _, v := range s.sMap {
			si := ServiceInfo{v.Name()}
			if cs, ok := v.(*ContractService); ok {
				si.Name = string(cs.Address())
			}
			r = append(r, si)
		}
		return c.JSON(http.StatusOK, r)
	})
	networkApi.POST("", func(c echo.Context) error {
		req := &RegisterContractServiceRequest{}
		if err := c.Bind(req); err != nil {
			return err
		}
		network := c.Param(PathParamNetwork)
		a := c.Get(ContextAdaptor).(contract.Adaptor)
		svc, err := NewContractService(a, req.Spec, req.Address, network, s.l)
		if err != nil {
			s.l.Debugf("fail to NewContractService err:%+v", err)
			return err
		}
		if _, ok := s.Signers[network]; ok {
			if svc, err = service.NewSignerService(svc, s.Signers, s.l); err != nil {
				s.l.Debugf("fail to NewSignerService err:%+v", err)
				return err
			}
		}
		s.SetService(svc)
		return c.NoContent(http.StatusOK)
	})

	networkApi.GET(UrlGetResult+"/:"+PathParamTxID, func(c echo.Context) error {
		a := c.Get(ContextAdaptor).(contract.Adaptor)
		p := c.Param(PathParamTxID)
		ret, err := a.GetResult(p)
		if err != nil {
			s.l.Debugf("fail to GetResult err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})

	networkApi.GET(UrlGetFinality+"/:"+PathParamBlockID, func(c echo.Context) error {
		fm := c.Get(ContextAdaptor).(contract.Adaptor).FinalityMonitor()
		id := c.Param(PathParamBlockID)
		p := c.QueryParam(QueryParamHeight)
		var (
			height int64
			err    error
		)
		if len(p) == 0 {
			if height, err = fm.HeightByID(id); err != nil {
				return err
			}
		} else {
			var ci contract.Integer
			if ci, err = contract.IntegerOf(p); err != nil {
				return err
			}
			if height, err = ci.AsInt64(); err != nil {
				return err
			}
		}
		ret, err := fm.IsFinalized(height, id)
		if err != nil {
			s.l.Debugf("fail to GetResult err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})

	serviceApi := networkApi.Group("/:" + PathParamServiceOrAddress)
	serviceInjection := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p := c.Param(PathParamServiceOrAddress)
			svc := s.GetService(p)
			if svc == nil {
				return echo.NewHTTPError(http.StatusNotFound,
					fmt.Sprintf("Service(%s) not found", p))
			}
			c.Set(ContextService, svc)
			return next(c)
		}
	}
	serviceApi.GET("", func(c echo.Context) error {
		svc := c.Get(ContextService).(service.Service)
		r := make(MethodInfos, 0)
		networkType := svc.Networks()[c.Param(PathParamNetwork)]
		for _, v := range svc.Spec().Methods {
			mi := MethodInfo{
				NetworkTypes: v.NetworkTypes,
				Name:         v.Name,
				Inputs:       NewNameAndTypeInfos(v.Inputs),
				Output:       NewTypeInfo(v.Output),
				Payable:      v.Payable,
				Readonly:     v.Readonly,
			}
			if !service.StringSetContains(v.NetworkTypes, networkType) {
				for _, o := range v.Overloads {
					if service.StringSetContains(o.NetworkTypes, networkType) {
						mi.NetworkTypes = o.NetworkTypes
						if o.Inputs != nil {
							mi.Inputs = NewNameAndTypeInfos(*o.Inputs)
						}
						if o.Output != nil {
							mi.Output = NewTypeInfo(*o.Output)
						}
						if o.Payable != nil {
							mi.Payable = *o.Payable
						}
						if o.Readonly != nil {
							mi.Readonly = *o.Readonly
						}
						break
					}
				}
			}
			r = append(r, mi)
		}
		return c.JSON(http.StatusOK, r)
	}, serviceInjection)

	methodApi := serviceApi.Group("/:" + PathParamMethod)
	methodApi.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
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

			p := c.Param(PathParamServiceOrAddress)
			svc := s.GetService(p)
			if svc == nil {
				network, address := c.Param(PathParamNetwork), contract.Address(p)
				if len(req.Spec) > 0 {
					a := c.Get(ContextAdaptor).(contract.Adaptor)
					var err error
					if svc, err = NewContractService(a, req.Spec, address, network, s.l); err != nil {
						s.l.Debugf("fail to NewContractService err:%+v", err)
						return err
					}
					if _, ok := s.Signers[network]; ok {
						if svc, err = service.NewSignerService(svc, s.Signers, s.l); err != nil {
							s.l.Debugf("fail to NewSignerService err:%+v", err)
							return err
						}
					}
					s.SetService(svc)
				} else if svc = s.GetService(ContractServiceName(network, address)); svc == nil {
					return echo.NewHTTPError(http.StatusNotFound,
						fmt.Sprintf("Service(%s) not found", p))
				}
			}
			c.Set(ContextService, svc)

			pm := c.Param(PathParamMethod)
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
	methodApi.POST("", func(c echo.Context) error {
		req := c.Get(ContextRequest).(*Request)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(PathParamNetwork)
		method := c.Param(PathParamMethod)
		txID, err := svc.Invoke(network, method, req.Params, req.Options)
		if err != nil {
			s.l.Errorf("fail to Invoke err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, txID)
	})
	methodApi.GET("", func(c echo.Context) error {
		var (
			ret contract.ReturnValue
			err error
		)
		req := c.Get(ContextRequest).(*Request)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(PathParamNetwork)
		method := c.Param(PathParamMethod)
		ret, err = svc.Call(network, method, req.Params, req.Options)
		if err != nil {
			s.l.Errorf("fail to Call err:%+v", err)
			return err
		}
		return c.JSON(http.StatusOK, StructToParams(ret))
	})
}

func StructToParams(v interface{}) interface{} {
	var p contract.Params
	switch t := v.(type) {
	case contract.Struct:
		p = t.Params()
	case contract.Params:
		p = t
	default:
		return v
	}
	for k, pv := range p {
		p[k] = StructToParams(pv)
	}
	return p
}

func (s *Server) RegisterAPIDocHandler(g *echo.Group) {
	g.GET("", func(c echo.Context) error {
		return c.JSON(http.StatusOK, s.oasp.Get(c.QueryParam(QueryParamService)))
	})
	g.GET(".yaml", func(c echo.Context) error {
		t := s.oasp.Get(c.QueryParam(QueryParamService))
		b, err := yaml.Marshal(t)
		if err != nil {
			return err
		}
		return c.Blob(http.StatusOK, "application/vnd.oai.openapi", b)
	})
}

type EventMonitorRequest struct {
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
	id := s.wsID(conn)
	pingHandler := conn.PingHandler()
	conn.SetPingHandler(func(appData string) error {
		s.l.Logf(s.cfg.TransportLogLevel.Level(), "[%s]wsPing received %s", id, appData)
		return pingHandler(appData)
	})
	conn.SetPongHandler(func(appData string) error {
		s.l.Logf(s.cfg.TransportLogLevel.Level(), "[%s]wsPong=%s", id, appData)
		return nil
	})
	s.l.Debugf("[%s]wsConnect", id)
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
			s.l.Logf(s.cfg.TransportLogLevel.Level(), "[%s]wsRead=%s", id, t)
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
	s.l.Logf(s.cfg.TransportLogLevel.Level(), "[%s]wsWrite=%s", s.wsID(conn), b)
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
			s.l.Logf(s.cfg.TransportLogLevel.Level(), "[%s]wsReadLoop=%s", id, b)
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

func (s *Server) wsPingLoop(ctx context.Context, conn *websocket.Conn) {
	if s.cfg.PingIntervalSec <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(s.cfg.PingIntervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.l.Logf(s.cfg.TransportLogLevel.Level(), "[%s]wsPing", s.wsID(conn))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.wsClose(conn)
				return
			}
		}
	}
}

func (s *Server) RegisterMonitorHandler(g *echo.Group) {
	monitorApi := g.Group("/:"+PathParamNetwork+"/:"+PathParamServiceOrAddress,
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				p := c.Param(PathParamServiceOrAddress)
				svc := s.GetService(p)
				if svc == nil {
					network, address := c.Param(PathParamNetwork), contract.Address(p)
					if svc = s.GetService(ContractServiceName(network, address)); svc == nil {
						return echo.NewHTTPError(http.StatusNotFound,
							fmt.Sprintf("Service(%s) not found", p))
					}
				}
				c.Set(ContextService, svc)
				return next(c)
			}
		})
	monitorApi.GET(UrlMonitorEvent, func(c echo.Context) error {
		conn, err := s.wsConnect(c)
		if err != nil {
			return err
		}
		defer s.wsClose(conn)
		id := s.wsID(conn)
		svc := c.Get(ContextService).(service.Service)
		network := c.Param(PathParamNetwork)
		var efs []contract.EventFilter
		req := &EventMonitorRequest{}
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
		go s.wsPingLoop(ctx, conn)
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

type AutoCallerInfo struct {
	Name string `json:"name"`
}
type AutoCallerInfos []AutoCallerInfo

func (s *Server) RegisterAutoCallerHandler(g *echo.Group) {
	g.GET("/:"+PathParamService, func(c echo.Context) error {
		p := c.Param(PathParamService)
		ac := s.GetAutoCaller(p)
		if ac == nil {
			return echo.NewHTTPError(http.StatusNotFound,
				fmt.Sprintf("AutoCaller(%s) not found", p))
		}
		fp := autocaller.FindParam{}
		if err := BindQueryParamsAndUnmarshalBody(c, &fp); err != nil {
			s.l.Debugf("fail to BindQueryParamsAndUnmarshalBody err:%+v", err)
			return echo.ErrBadRequest
		}
		s.l.Debugln("fp.Task:", fp.Task)
		if len(fp.Task) > 0 && !service.StringSetContains(ac.Tasks(), fp.Task) {
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("invalid task(%s), must be empty or one of {%s}", fp.Task, strings.Join(ac.Tasks(), ",")))
		}

		ret, err := ac.Find(fp)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
}

func (s *Server) RegisterTrackerHandler(g *echo.Group) {
	g.GET("/summary", func(c echo.Context) error {
		ret, err := repository.SummaryOfBtpStatusByNetworks(s.db)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
	g.GET("/search", func(c echo.Context) error {
		src := c.QueryParam("src")
		strNsn := c.QueryParam("nsn")
		nsn, _ := strconv.ParseInt(strNsn, 10, 64)
		ret, err := repository.GetBtpStatusBySrcAndNsn(s.db, src, nsn)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
	g.GET("/status/latest", func(c echo.Context) error {
		p := utils.DefaultPageable()

		var btpStatus []*repository.BTPStatus
		page, err := utils.Paginate(s.db, p, btpStatus, repository.BTPStatus{})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, page)
	})
	g.GET("/status", func(c echo.Context) error {
		p := utils.GetPageableFromRequest(c)
		var btpStatus []*repository.BTPStatus
		page, err := utils.Paginate(s.db, p, btpStatus, repository.BTPStatus{})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, page)
	})
	g.GET("/status/:"+PathParamId, func(c echo.Context) error {
		id, _ := strconv.Atoi(c.Param(PathParamId))
		ret, err := repository.SelectBtpStatusBy(s.db, repository.BTPStatus{
			Id: id,
		})
		if err != nil {
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
