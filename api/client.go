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
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
)

type Client struct {
	*http.Client
	baseUrl string
	l       log.Logger
}

func NewClient(url string, transportLogLevel log.Level, l log.Logger) *Client {
	l = Logger(l)
	return &Client{
		Client:  contract.NewHttpClient(transportLogLevel, l),
		baseUrl: url,
		l:       l,
	}
}

func (c *Client) do(method, endpoint string, reqPtr, respPtr interface{}) (resp *http.Response, err error) {
	var reqBody io.Reader
	if reqPtr != nil {
		var b []byte
		if b, err = json.Marshal(reqPtr); err != nil {
			c.l.Debugf("fail to encode Request err:%+v", err)
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseUrl+endpoint, reqBody)
	if err != nil {
		c.l.Debugf("fail to NewRequest err:%+v", err)
		return nil, err
	}
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
	_, err := c.do(http.MethodGet, fmt.Sprintf("/%s/result/%s", network, id), nil, &txr)
	return txr, err
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
	if s != nil && err != nil && contract.RequireSignatureErrorCode.Equals(err) {
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

func (c *Client) Invoke(network string, addr contract.Address, req *ContractRequest, s service.Signer) (contract.TxID, error) {
	return c.invoke(fmt.Sprintf("/%s/%s/invoke", network, addr), req, s)
}

func (c *Client) ServiceInvoke(network, svc string, req *Request, s service.Signer) (contract.TxID, error) {
	return c.invoke(fmt.Sprintf("/%s/%s/invoke", network, svc), req, s)
}

func (c *Client) Call(network string, addr contract.Address, req *ContractRequest, resp interface{}) (*http.Response, error) {
	return c.do(http.MethodGet, fmt.Sprintf("/%s/%s/call", network, addr), req, resp)
}

func (c *Client) ServiceCall(network, svc string, req *Request, resp interface{}) (*http.Response, error) {
	return c.do(http.MethodGet, fmt.Sprintf("/%s/%s/call", network, svc), req, resp)
}
