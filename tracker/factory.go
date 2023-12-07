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

package tracker

import (
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service"
)

type Tracker interface {
	Name() string
	// Start TODO xCallEvent[CallMessage, CallMessageSent, ResponseMessage, CallExecuted, RollbackExecuted], default monitorBTPEvent
	Start() error
	Stop() error
	Tasks() []string
	Find(FindParam) (*database.Page[any], error)
	FindOne(FindOneParam) (any, error)
	Summary() ([]any, error)
	Networks() []NetworkOfTracker

	Relink() error
}

type FindParam struct {
	Task     string                 `json:"task"`
	Pageable database.Pageable      `json:"pageable"`
	Query    map[string]interface{} `json:"query"`
}

type FindOneParam struct {
	Task     string                 `json:"task"`
	Query    map[string]interface{} `json:"query"`
}

type Network struct {
	NetworkType string
	Adaptor     contract.Adaptor
	Options     contract.Options
}

type NetworkOfTracker struct {
	Name 		string 		`json:"name"`
	Address	 	string		`json:"address"`
	Type 		string		`json:"type"`
	Image 		string		`json:"imageBase64"`
}

type Options map[string]interface{}
type Factory func(service.Service, map[string]Network, *gorm.DB, log.Logger) (Tracker, error)

var (
	fMap = make(map[string]Factory)
)

func RegisterFactory(trackerName string, sf Factory) {
	if _, ok := fMap[trackerName]; ok {
		log.Panicln("already registered tracker:" + trackerName)
	}
	fMap[trackerName] = sf
	log.Tracef("RegisterFactory tracker:%s", trackerName)
}

func NewTracker(name string, s service.Service, networks map[string]Network, db *gorm.DB, l log.Logger) (Tracker, error) {
	if f, ok := fMap[name]; ok {
		l = l.WithFields(log.Fields{log.FieldKeyChain: name, log.FieldKeyModule: "tracker"})
		return f(s, networks, db, l)
	}
	return nil, errors.Errorf("not found tracker name:%s", name)
}
