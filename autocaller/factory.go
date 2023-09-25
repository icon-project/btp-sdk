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

package autocaller

import (
	"encoding/json"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service"
)

type AutoCaller interface {
	Name() string
	Tasks() []string
	Start() error
	Stop() error
	Find(FindParam) (*database.Page[any], error)
}

type FindParam struct {
	Task     string                 `json:"task"`
	Pageable database.Pageable      `json:"pageable"`
	Query    map[string]interface{} `json:"query"`
}

type Network struct {
	NetworkType string
	Adaptor     contract.Adaptor
	Signer      service.Signer
	Options     contract.Options
}

type Factory func(service.Service, map[string]Network, *gorm.DB, log.Logger) (AutoCaller, error)

var (
	fMap = make(map[string]Factory)
)

func RegisterFactory(serviceName string, sf Factory) {
	if _, ok := fMap[serviceName]; ok {
		log.Panicln("already registered autocaller:" + serviceName)
	}
	fMap[serviceName] = sf
	log.Tracef("RegisterFactory autocaller:%s", serviceName)
}

func NewAutoCaller(name string, s service.Service, networks map[string]Network, db *gorm.DB, l log.Logger) (AutoCaller, error) {
	if f, ok := fMap[name]; ok {
		l = l.WithFields(log.Fields{log.FieldKeyChain: name, log.FieldKeyModule: "autocaller"})
		return f(s, networks, db, l)
	}
	return nil, errors.Errorf("not found autocaller name:%s", name)
}

type TaskState int

const (
	TaskStateNone TaskState = iota
	TaskStateSending
	TaskStateSkip
	TaskStateDone
	TaskStateError
)

func (s TaskState) String() string {
	switch s {
	case TaskStateNone:
		return "none"
	case TaskStateSending:
		return "sending"
	case TaskStateSkip:
		return "skip"
	case TaskStateDone:
		return "done"
	case TaskStateError:
		return "error"
	default:
		return ""
	}
}

func ParseTaskState(s string) (TaskState, error) {
	switch s {
	case "none":
		return TaskStateNone, nil
	case "sending":
		return TaskStateSending, nil
	case "skip":
		return TaskStateSkip, nil
	case "done":
		return TaskStateDone, nil
	case "error":
		return TaskStateError, nil
	default:
		return TaskStateNone, errors.Errorf("invalid TaskState str:%s", s)
	}
}

func (s *TaskState) UnmarshalJSON(input []byte) error {
	var str string
	if err := json.Unmarshal(input, &str); err != nil {
		return err
	}
	v, err := ParseTaskState(str)
	if err != nil {
		return err
	}
	*s = v
	return nil
}

func (s TaskState) MarshalJSON() ([]byte, error) {
	if s < TaskStateNone || s > TaskStateError {
		return nil, errors.New("invalid TaskState")
	}
	return json.Marshal(s.String())
}

type Task struct {
	database.Model
	Name        string    `json:"name"`
	Network     string    `json:"network" gorm:"index"`
	State       TaskState `json:"state"`
	TxID        string    `json:"tx_id"`
	BlockHeight int64     `json:"block_height"`
	BlockID     string    `json:"block_id"`
	Failure     string    `json:"failure"`
}
