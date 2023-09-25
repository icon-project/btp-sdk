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

import "github.com/icon-project/btp2/common/errors"

const (
	ErrorCodeNotFoundMethod errors.Code = errors.CodeGeneral + iota
	ErrorCodeMismatchReadonly
	ErrorCodeNotFoundEvent
	ErrorCodeInvalidParam
	ErrorCodeInvalidOption
	ErrorCodeRequireSignature
	ErrorCodeNotFoundTransaction
)

var (
	errRequireSignature = errors.NewBase(ErrorCodeRequireSignature, "RequireSignatureError")
)

type RequireSignatureError interface {
	errors.ErrorCoder
	Data() []byte
	Options() Options
}
type requireSignatureError struct {
	errors.ErrorCoder
	data    []byte
	options Options
}

func (e *requireSignatureError) Data() []byte {
	return e.data
}

func (e *requireSignatureError) Options() Options {
	return e.options
}

func NewRequireSignatureError(data []byte, options Options) RequireSignatureError {
	return &requireSignatureError{
		ErrorCoder: errRequireSignature,
		data:       data,
		options:    options,
	}
}

type EstimateError interface {
	error
	ErrorCode() int
	ErrorData() interface{}
	Reason() string
}
