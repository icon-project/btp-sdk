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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/icon-project/btp2/common/errors"
	"github.com/labstack/echo/v4"

	"github.com/icon-project/btp-sdk/contract"
)

type ErrorResponse struct {
	Code    errors.Code     `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("code:%d, message:%s", e.Code, e.Message)
}

func (e *ErrorResponse) ErrorCode() errors.Code {
	return e.Code
}

func (e *ErrorResponse) MarshalData(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	e.Data = b
	return nil
}

func (e *ErrorResponse) UnmarshalData(v interface{}) error {
	return json.Unmarshal(e.Data, v)
}

type RequireSignatureError struct {
	Data    []byte
	Options contract.Options
}

func HttpErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		if e, ok := he.Message.(error); ok {
			err = e
		}
	}
	er := &ErrorResponse{
		Code:    errors.CodeOf(err),
		Message: err.Error(),
	}
	if rse, ok := err.(contract.RequireSignatureError); ok {
		if er.Data, err = json.Marshal(&RequireSignatureError{
			Data:    rse.Data(),
			Options: rse.Options(),
		}); err != nil {
			c.Echo().Logger.Error(err)
		}
	}
	if !c.Response().Committed {
		if err = c.JSON(code, er); err != nil {
			c.Echo().Logger.Error(err)
		}
	}
}
