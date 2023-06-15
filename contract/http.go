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

package contract

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
)

const (
	DefaultTransportLogLevel = log.TraceLevel
	TransportLogLevelLimit   = log.InfoLevel
)

type HttpTransport struct {
	*http.Transport
	lv log.Level
	l  log.Logger
}

func (t *HttpTransport) log(rc io.ReadCloser) (io.ReadCloser, error) {
	if rc == nil {
		return nil, nil
	}
	b, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, errors.Wrapf(err, "fail to ioutil.ReadAll err:%s", err.Error())
	}
	t.l.Logln(t.lv, string(b))
	return ioutil.NopCloser(bytes.NewBuffer(b)), nil
}

func (t *HttpTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if req.Body, err = t.log(req.Body); err != nil {
		return nil, err
	}
	if resp, err = t.Transport.RoundTrip(req); err != nil {
		return nil, errors.Wrapf(err, "fail to RoundTrip err:%s", err.Error())
	}
	if resp.Body, err = t.log(resp.Body); err != nil {
		return nil, err
	}
	return resp, err
}

func NewHttpTransport(lv log.Level, l log.Logger) *HttpTransport {
	return &HttpTransport{
		Transport: &http.Transport{},
		lv:        EnsureTransportLogLevel(lv),
		l:         l,
	}
}

func NewHttpClient(lv log.Level, l log.Logger) *http.Client {
	return &http.Client{
		Transport: NewHttpTransport(lv, l),
	}
}

func EnsureTransportLogLevel(lv log.Level) log.Level {
	if lv < TransportLogLevelLimit {
		return DefaultTransportLogLevel
	}
	return lv
}
