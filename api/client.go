/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/service"
)

type Client struct {
	*http.Client
	baseUrl        string
	baseApiUrl     string
	baseMonitorUrl string
	networkToType  map[string]string
	lv             log.Level
	l              log.Logger
}

func NewClient(url string, networkToType map[string]string, transportLogLevel log.Level, l log.Logger) *Client {
	l = Logger(l)
	return &Client{
		Client:         contract.NewHttpClient(transportLogLevel, l),
		baseUrl:        url,
		baseApiUrl:     url + GroupUrlApi,
		baseMonitorUrl: url + GroupUrlMonitor,
		networkToType:  networkToType,
		lv:             transportLogLevel,
		l:              l,
	}
}

func (c *Client) apiUrl(format string, args ...interface{}) string {
	return c.baseApiUrl + fmt.Sprintf(format, args...)
}

func (c *Client) do(method, url string, reqPtr, respPtr interface{}) (resp *http.Response, err error) {
	var reqBody io.Reader
	if reqPtr != nil {
		var b []byte
		if b, err = json.Marshal(reqPtr); err != nil {
			c.l.Debugf("fail to encode Request err:%+v", err)
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}
	if !strings.HasPrefix(url, c.baseApiUrl) {
		url = c.baseApiUrl + url
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		c.l.Debugf("fail to NewRequest err:%+v", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.l.Debugf("url=%s", req.URL)
	if resp, err = c.Client.Do(req); err != nil {
		return
	}
	if resp.StatusCode/100 != 2 {
		er := &ErrorResponse{}
		if err = UnmarshalBody(resp.Body, er); err != nil {
			c.l.Debugf("fail to decode ErrorResponse err:%+v", err)
			err = errors.Errorf("server response not success, StatusCode:%d",
				resp.StatusCode)
			return
		}
		err = er
		return
	}
	if respPtr != nil {
		if err = UnmarshalBody(resp.Body, respPtr); err != nil {
			c.l.Debugf("fail to decode resp err:%+v", err)
			return
		}
	}
	return
}

func (c *Client) GetResult(network string, id contract.TxID) (interface{}, error) {
	var txr interface{}
	nt, ok := c.networkToType[network]
	if ok {
		switch nt {
		case icon.NetworkTypeIcon:
			txr = &icon.TxResult{}
		case eth.NetworkTypeEth, eth.NetworkTypeEth2, eth.NetworkTypeBSC:
			txr = &eth.TxResult{}
		default:
			txr = new(interface{})
		}
	}
	_, err := c.do(http.MethodGet, c.apiUrl("/%s%s/%s", network, UrlGetResult, id), nil, txr)
	if err != nil {
		return nil, err
	}
	return txr, nil
}

func (c *Client) invoke(url string, req interface{}, s service.Signer) (contract.TxID, error) {
	var (
		p   *contract.Options
		opt contract.Options
		err error
	)
	if s != nil {
		switch t := req.(type) {
		case *Request:
			p = &t.Options
		case *ContractRequest:
			p = &t.Options
		}
		if opt, err = service.PrepareToSign(*p, s, true); err != nil {
			return nil, err
		}
		*p = opt
	}
	var txId contract.TxID
	_, err = c.do(http.MethodPost, url, req, &txId)
	if s != nil && err != nil && contract.ErrorCodeRequireSignature.Equals(err) {
		er := err.(*ErrorResponse)
		rse := &RequireSignatureError{}
		if err = er.UnmarshalData(rse); err != nil {
			return nil, err
		}
		if opt, err = service.Sign(rse.Data, rse.Options, s); err != nil {
			return nil, err
		}
		*p = opt
		_, err = c.do(http.MethodPost, url, req, &txId)
		return txId, err
	}
	return txId, err
}

func (c *Client) NetworkInfos() (NetworkInfos, error) {
	r := NetworkInfos{}
	if _, err := c.do(http.MethodGet, c.baseApiUrl, nil, &r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *Client) ServiceInfos(network string) (ServiceInfos, error) {
	r := ServiceInfos{}
	if _, err := c.do(http.MethodGet, c.apiUrl("/%s", network), nil, &r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *Client) RegisterContractService(network string, req *RegisterContractServiceRequest) error {
	_, err := c.do(http.MethodPost, c.apiUrl("/%s", network), req, nil)
	return err
}

func (c *Client) MethodInfos(network string, serviceOrAddress string) (MethodInfos, error) {
	r := MethodInfos{}
	if _, err := c.do(http.MethodGet, c.apiUrl("/%s/%s", network, serviceOrAddress), nil, &r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *Client) Invoke(network string, addr contract.Address, method string, req *ContractRequest, s service.Signer) (contract.TxID, error) {
	return c.invoke(c.apiUrl("/%s/%s/%s", network, addr, method), req, s)
}

func (c *Client) ServiceInvoke(network, svc, method string, req *Request, s service.Signer) (contract.TxID, error) {
	return c.invoke(c.apiUrl("/%s/%s/%s", network, svc, method), req, s)
}

func (c *Client) Call(network string, addr contract.Address, method string, req *ContractRequest, resp interface{}) (*http.Response, error) {
	return c.do(http.MethodGet, c.apiUrl("/%s/%s/%s", network, addr, method), req, resp)
}

func (c *Client) ServiceCall(network, svc, method string, req *Request, resp interface{}) (*http.Response, error) {
	return c.do(http.MethodGet, c.apiUrl("/%s/%s/%s", network, svc, method), req, resp)
}

func (c *Client) monitorUrl(format string, args ...interface{}) string {
	return c.baseMonitorUrl + fmt.Sprintf(format, args...)
}

func (c *Client) wsID(conn *websocket.Conn) string {
	return conn.LocalAddr().String()
}

func (c *Client) wsConnect(ctx context.Context, url string) (*websocket.Conn, error) {
	url = strings.Replace(url, "http", "ws", 1)
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		if err == websocket.ErrBadHandshake {
			er := &ErrorResponse{}
			if err = UnmarshalBody(resp.Body, er); err != nil {
				err = errors.Errorf("server response not success, StatusCode:%d",
					resp.StatusCode)
			}
			err = er
		}
		c.l.Debugf("fail to Dial url:%s err:%+v", url, err)
		return nil, err
	}
	id := c.wsID(conn)
	pingHandler := conn.PingHandler()
	conn.SetPingHandler(func(appData string) error {
		c.l.Logf(c.lv, "[%s]wsPing=%s", id, appData)
		return pingHandler(appData)
	})
	conn.SetPongHandler(func(appData string) error {
		c.l.Logf(c.lv, "[%s]unexpected wsPong %s", id, appData)
		return nil
	})
	c.l.Debugf("[%s]wsConnect", id)
	return conn, nil
}

func (c *Client) wsHandshake(ctx context.Context, conn *websocket.Conn, req interface{}) error {
	var err error
	id := c.wsID(conn)
	if err = c.wsWrite(conn, req); err != nil {
		c.l.Debugf("[%s]fail to wsWrite err:%+v", id, err)
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, WsHandshakeTimeout)
	defer cancel()
	er := &ErrorResponse{}
	if err = c.wsRead(tctx, conn, er); err != nil {
		c.l.Debugf("[%s]fail to wsRead err:%+v", id, err)
		return err
	}
	if !errors.Success.Equals(er) {
		err = er
		return err
	}
	return nil
}

func (c *Client) wsClose(conn *websocket.Conn) {
	c.l.Debugf("[%s]wsClose", c.wsID(conn))
	conn.Close()
}

func (c *Client) wsRead(ctx context.Context, conn *websocket.Conn, v interface{}) error {
	id := c.wsID(conn)
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
			c.l.Logf(c.lv, "[%s]wsRead=%s", id, t)
			return nil
		default:
			c.l.Panicln("unreachable code")
			return nil
		}
	}
}

func (c *Client) wsWrite(conn *websocket.Conn, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.l.Logf(c.lv, "[%s]wsWrite=%s", c.wsID(conn), b)
	return conn.WriteMessage(websocket.TextMessage, b)
}

func (c *Client) wsReadLoop(ctx context.Context, conn *websocket.Conn, cb func(b []byte) error) error {
	id := c.wsID(conn)
	ech := make(chan error, 1)
	go func() {
		defer func() {
			c.l.Debugf("[%s]wsReadLoop finish", id)
		}()
		for {
			_, b, err := conn.ReadMessage()
			if err != nil {
				ech <- err
				break
			}
			c.l.Logf(c.lv, "[%s]wsReadLoop=%s", id, b)
			if err = cb(b); err != nil {
				ech <- err
				break
			}
		}
	}()

	select {
	case <-ctx.Done():
		c.l.Debugf("[%s]wsReadLoop context Done", id)
		return ctx.Err()
	case err := <-ech:
		c.l.Debugf("[%s]wsReadLoop err:%+v", id, err)
		return err
	}
}

func (c *Client) monitorEvent(ctx context.Context, network, url string, req *EventMonitorRequest, cb contract.EventCallback) error {
	conn, err := c.wsConnect(ctx, url)
	if err != nil {
		return err
	}
	defer c.wsClose(conn)
	if err = c.wsHandshake(ctx, conn, req); err != nil {
		return err
	}
	type eventSupplier func() contract.Event
	var supplier eventSupplier
	nt, ok := c.networkToType[network]
	if ok {
		switch nt {
		case icon.NetworkTypeIcon:
			supplier = func() contract.Event { return &icon.Event{} }
		case eth.NetworkTypeEth, eth.NetworkTypeEth2, eth.NetworkTypeBSC:
			supplier = func() contract.Event { return &eth.Event{} }
		default:
			c.wsClose(conn)
			return errors.Errorf("not supported network %s", network)
		}
	}
	return c.wsReadLoop(ctx, conn, func(b []byte) error {
		v := supplier()
		if err = json.Unmarshal(b, v); err != nil {
			return err
		}
		return cb(v)
	})
}

func (c *Client) MonitorEvent(ctx context.Context, network string, addr contract.Address, req *EventMonitorRequest, cb contract.EventCallback) error {
	return c.monitorEvent(ctx, network, c.monitorUrl("/%s/%s%s", network, addr, UrlMonitorEvent), req, cb)
}

func (c *Client) ServiceMonitorEvent(ctx context.Context, network string, svc string, req *EventMonitorRequest, cb contract.EventCallback) error {
	return c.monitorEvent(ctx, network, c.monitorUrl("/%s/%s%s", network, svc, UrlMonitorEvent), req, cb)
}
