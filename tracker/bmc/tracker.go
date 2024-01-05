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

package bmc

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/tracker"
)

const (
	EventBTP                    = "BTPEvent"

	SEND    = "SEND"
	ROUTE   = "ROUTE"
	RECEIVE = "RECEIVE"
	REPLY   = "REPLY"
	DROP    = "DROP"
	ERROR   = "ERROR"
	TaskStatus = "status"
	TaskEvent = "event"
	TaskSearch = "search"
	DefaultFinalitySubscribeBuffer = 1
)

func init() {
	tracker.RegisterFactory(bmc.ServiceName, NewTracker)
}

type Tracker struct {
	s         service.Service
	nMap      map[string]Network
	runCtx 	  context.Context
	runCancel context.CancelFunc
	runMtx    sync.RWMutex
	l         log.Logger

	br *BlockRepository
	sr *BTPStatusRepository
	er *BTPEventRepository
}

type Network struct {
	NetworkType  string
	Adaptor      contract.Adaptor
	Options      TrackerOptions
	EventFilters []contract.EventFilter
	Ctx 		 context.Context
	Cancel 		 context.CancelFunc
}

type TrackerOptions struct {
	InitHeight      int64              	`json:"init_height"`
	NetworkAddress  string             	`json:"network_address"`
	NetworkIcon		string				`json:"network_icon"`
}

func NewTracker(s service.Service, networks map[string]tracker.Network, db *gorm.DB, l log.Logger) (tracker.Tracker, error) {
	if s.Name() != bmc.ServiceName {
		return nil, errors.Errorf("invalid service name:%s", s.Name())
	}

	nMap := make(map[string]Network)
	for network, n := range networks {
		opt := &TrackerOptions{}
		if err := contract.DecodeOptions(n.Options, &opt); err != nil {
			return nil, err
		}
		nMap[network] = Network{
			NetworkType: n.NetworkType,
			Adaptor:     n.Adaptor,
			Options:     *opt,
		}
	}
	for fromNetwork, fn := range nMap {
		srcParams := make([]contract.Params, 0)
		for _, tn := range nMap {
			srcParams = append(srcParams, contract.Params{
				"_src": tn.Options.NetworkAddress,
			})
		}
		nameToParams := map[string][]contract.Params{
			EventBTP: srcParams,
		}
		l.Debugf("fromNetwork:%s nameToParams:%s", fromNetwork, nameToParams)
		efs, err := s.EventFilters(fromNetwork, nameToParams)
		if err != nil {
			return nil, err
		}
		fn.EventFilters = efs[:]
		nMap[fromNetwork] = fn
	}

	br, err := NewBlockRepository(db)
	if err != nil {
		return nil, err
	}
	sr, err := NewBTPStatusRepository(db)
	if err != nil {
		return nil, err
	}
	er, err := NewBTPEventRepository(db)
	if err != nil {
		return nil, err
	}

	return &Tracker{
		s:     s,
		nMap:  nMap,
		l:     l,
		br: br,
		sr: sr,
		er: er,
	}, nil
}

func (r *Tracker) Name() string {
	return r.s.Name()
}

func (r *Tracker) Networks() []tracker.NetworkOfTracker {
	values := make([]tracker.NetworkOfTracker, 0, len(r.nMap))
	for k, v := range r.nMap {
		values = append(values, tracker.NetworkOfTracker{
			Name: k,
			Address: v.Options.NetworkAddress,
			Type: v.NetworkType,
			Image: v.Options.NetworkIcon,
		})
	}
	return values
}

func (r *Tracker) Tasks() []string {
	return []string{TaskStatus, TaskEvent, TaskSearch}
}

func (r *Tracker) Relink() error {
	for _, n := range r.nMap {
		statuses := r.getUncompletedBtpStatus(n.Options.NetworkAddress)
		for _, status := range statuses {
			err := r.updateReLink(&status, status.Src, status.Nsn)
			if err != nil {
				r.l.Errorf("Fail to Relink BTP Events src:%s nsn:%+v nsn:%+v", status.Src, status.Nsn, err)
				return err
			}
		}
	}
	return nil
}

func (r *Tracker)getUncompletedBtpStatus(src string) []BTPStatus {
	 statuses, err := r.sr.FindUncompletedBySrc(src)
	 if err != nil {
		 return []BTPStatus{}
	 }
	return statuses
}

func (r *Tracker) Start() error {
	r.runMtx.Lock()
	defer r.runMtx.Unlock()
	if r.runCancel != nil {
		return errors.Errorf("already started")
	}
	r.runCtx, r.runCancel = context.WithCancel(context.Background())
	for network, n := range r.nMap {
		go r.monitorFinality(r.runCtx, network)
		n.Ctx, n.Cancel = context.WithCancel(r.runCtx)
		go r.monitorEvent(n.Ctx, network, n.EventFilters, n.Options.InitHeight)
	}
	return nil
}

func (r *Tracker) Stop() error {
	r.runMtx.Lock()
	defer r.runMtx.Unlock()
	if r.runCancel == nil {
		return errors.Errorf("already stopped")
	}
	r.runCancel()
	r.runCancel = nil
	r.runCtx = nil
	return nil
}

func (r *Tracker) getMonitorHeight(network string) (int64, error) {
	//TODO when tracking xcall event, have to get monitor height from btp and xcall event table
	na := r.nMap[network].Options.NetworkAddress
	ih := r.nMap[network].Options.InitHeight
	var lb *Block
	lb, err := r.br.FindOneByNetworkAddressOrderByHeightDesc(na)
	if err != nil {
		return ih, err
	}
	if lb != nil {
		if lb.Height >= ih {
			return lb.Height + 1, nil
		}
	}
	return ih, nil
}

func (r *Tracker) monitorEvent(ctx context.Context, network string, efs []contract.EventFilter, initHeight int64) {
	for {
		select {
		case <-time.After(time.Second):
			height, err := r.getMonitorHeight(network)
			if err != nil {
				r.l.Errorf("monitorEvent fail to getMonitorHeight network:%s err:%+v", network, err)
				continue
			}
			if height < 1 {
				height = initHeight
			}
			r.l.Debugf("monitorEvent network:%s height:%v", network, height)
			if err := r.s.MonitorEvent(ctx, network, func(e contract.Event) error {
				switch e.Name() {
				case EventBTP:
					return r.onBTPEvent(network, e)
				}
				return nil
			}, efs, height); err != nil {
				r.l.Debugf("MonitorEvent stopped network:%s err:%v", network, err)
			}
		case <-ctx.Done():
			r.l.Debugln("MonitorEvent context done")
			return
		}
	}
}

func (r *Tracker) onBTPEvent(network string, e contract.Event) error {
	na := r.nMap[network].Options.NetworkAddress
	hash, err := getParamString(e, "blockId")
	height := e.BlockHeight()

	b, err := r.storeBlock(network, na, hash, height)
	if err != nil {
		return err
	}
	src, err := getParamString(e, EpSrc)
	if err != nil {
		return errors.Errorf("invalid _src type:%T", e.Params()["_src"])
	}
	nsn, err := getParamInt64(e, EpNsn)
	if err != nil {
		return errors.Errorf("invalid _nsn type:%T", e.Params()["_nsn"])
	}

	bs, err := r.storeBtpStatus(src, network, na, nsn)

	err = r.storeBtpEvent(e, src, nsn, b.ID, bs.ID, network, na)

	err = r.updateLink(bs, src, network, na, nsn)

	return err
}

func getParamString(e contract.Event, param string) (string , error){
	var p interface{}
	var err error
	switch param {
	case EpSrc:
		p = e.Params()[EpSrc]
	case EpNext:
		p = e.Params()[EpNext]
	case EpEvent:
		p = e.Params()[EpEvent]
	case EpBlockId:
		p = e.BlockID()
	case EpTxId:
		p = e.TxID()
	}
	value, err := contract.StringOf(eventIndexedValue(p))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func getParamInt64(e contract.Event, param string) (int64, error){
	var p interface{}
	var err error
	switch param {
	case EpNsn:
		p, err = e.Params()[EpNsn].(contract.Integer).AsInt64()
	}
	return p.(int64), err
}

func eventIndexedValue(p interface{}) interface{} {
	if eivp, ok := p.(contract.EventIndexedValueWithParam); ok {
		return eivp.Param()
	}
	if eiv, ok := p.(contract.EventIndexedValue); ok {
		return fmt.Sprintf("%s", eiv)
	}
	return p
}

func (r *Tracker) storeBlock(network, na, hash string, height int64) (*Block, error) {
	b, err := r.br.FindOneByNetworkAddressAndHeightAndHash(na, hash, height)
	if err != nil {
		return nil, err
	}
	if b != nil {
		return b, nil
	}
	b = &Block{
		NetworkName: network,
		NetworkAddress: na,
		BlockId:      	hash,
		Height:         height,
		Finalized:      false,
	}
	err = r.br.Save(b)
	if err != nil {
		return nil, err
	}
	return r.br.FindOneByNetworkAddressAndHeightAndHash(na, hash, height)
}

func (r *Tracker) storeBtpStatus(src, network, na string, nsn int64) (*BTPStatus, error) {
	bs, err := r.sr.FindOneBySrcAndNsn(src, nsn)
	if err != nil {
		return nil, err
	}
	if bs != nil {
		return bs, nil
	}
	bs = &BTPStatus{
		Src:         		src,
		LastNetworkName: 	sql.NullString{String: network, Valid: true},
		LastNetworkAddress: sql.NullString{String: na, Valid: true},
		Status:      		sql.NullString{String: Unknown, Valid: true},
		Links:       		sql.NullString{String: "", Valid: true},
		Nsn:         		nsn,
	}
	err = r.sr.Save(bs)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return r.sr.FindOneBySrcAndNsn(src, nsn)
}

func (r *Tracker)storeBtpEvent(e contract.Event, src string, nsn int64, bId, bsId uint, network, na string) error {
	// Insert BTP Event with block id and btp status id
	next, err := getParamString(e, EpNext)
	event, err := getParamString(e, EpEvent)
	txHash, err := getParamString(e, EpTxId)
	if err != nil {
		return errors.Errorf("invalid _txId type:%T", e.Params()["_txId"])
	}

	identifier := []byte(strconv.Itoa(e.Identifier()))
	be := &BTPEvent{
		Src:         src,
		Nsn:         nsn,
		Next:        next,
		Event:       event,
		BlockId:     bId,
		BtpStatusId: bsId,
		TxHash:      txHash,
		EventId:     identifier,
		NetworkName: network,
		NetworkAddress:  na,
		Finalized: 	 false,
	}
	r.l.Tracef("BTPEvent: %v", be)
	return r.er.Save(be)
}

func (r *Tracker) updateLink(bs *BTPStatus, src, network, na string, nsn int64) error {
	links, status := r.getLinks(src, nsn)
	b, err := json.Marshal(links)
	if err != nil {
		log.Errorf("JSON marshaling failed: %s", err)
	}
	bs.Links = sql.NullString{
		String: string(b),
		Valid:  true,
	}
	bs.Status = sql.NullString{
		String: status,
		Valid:  true,
	}
	bs.LastNetworkName = sql.NullString{
		String: network,
		Valid:  true,
	}
	bs.LastNetworkAddress = sql.NullString{
		String: na,
		Valid:  true,
	}
	err = r.sr.Save(bs)
	return err
}

func (r *Tracker) updateReLink(bs *BTPStatus, src string, nsn int64) error {
	links, status := r.getLinks(src, nsn)
	b, err := json.Marshal(links)
	if err != nil {
		log.Errorf("JSON marshaling failed: %s", err)
	}
	bs.Links = sql.NullString{
		String: string(b),
		Valid:  true,
	}
	bs.Status = sql.NullString{
		String: status,
		Valid:  true,
	}
	err = r.sr.Save(bs)
	return err
}

func (r *Tracker) getLinks(src string, nsn int64) ([]uint, string) {
	links := make([]uint, 0)
	status := Unknown
	//Find BTPEvents by src and nsn, order by createdAt(time) asc
	events, err := r.er.FindBySrcAndNsn(src, nsn)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		log.Debugf("No results were found, src: %s, nsn: %s", src, nsn)
	}
	if len(events) == 0 { return links, status }

	//Find starting point that BTPEvent.Event is "SEND"
	curLink, location := findStartingEvent(events)
	if location == -1 { return links, status }
	links = append(links, curLink.ID)
	status = Sending
	events, _ = trimBTPEvents(events, location)

	//Find next links, after find "SEND" event
	for i := 0; i <= len(events); i++ {
		//Find next link by BTPEvent.Next and BTPEvent.Event
		//If next link does not exist, break loop and return links
		curLink = findNextLink(events, curLink)
		if reflect.DeepEqual(curLink, BTPEvent{}) {
			return links, status
		}
		links = append(links, curLink.ID)
		events, i = trimBTPEventsById(events, curLink.ID, i)

		// It means the transfer closed. so return links
		if status = inferStatus(curLink, status); status == Completed { break }
	}
	return links, status
}

func trimBTPEvents(events []BTPEvent, index int) ([]BTPEvent, int) {
	events = append(events[:index], events[index+1:]...)
	index--
	return events, index
}

func trimBTPEventsById(events []BTPEvent, id uint, index int) ([]BTPEvent, int) {
	found := false
	for i := 0; i < len(events); i++ {
		if id == events[i].ID {
			found = true
			index = i
			break
		}
	}
	if found {
		events = append(events[:index], events[index+1:]...)
		index--
	}
	return events, index
}

// If current link's event is "DROP" or "RECEIVE", BTPEvent.Next is Empty.
func inferStatus(event BTPEvent, status string) string {
	if event.Event == DROP { return Completed }
	switch status {
	case Sending:
		if event.Event == RECEIVE {
			if len(event.Next) > 0 {
				status = WaitReply
			} else {
				status = Completed
			}
		}
	case WaitReply:
		if event.Event == REPLY || event.Event == ERROR {
			status = Replying
		}
	case Replying:
		if event.Event == RECEIVE {
			status = Completed
		}
	}
	return status
}

func findStartingEvent(events []BTPEvent) (BTPEvent, int) {
	var curLink BTPEvent
	for i, event := range events {
		if event.Event == SEND {
			return event, i
		}
	}
	return curLink, -1
}

func findNextLink(events []BTPEvent, curLink BTPEvent) BTPEvent {
	for _, event := range events {
		if curLink.Event == RECEIVE {
			if event.Event == REPLY || event.Event == ERROR || event.Event == DROP {
				return event
			}
		} else {
			if curLink.Next == event.NetworkAddress {
				if event.Next == curLink.NetworkAddress {
					if event.Event == RECEIVE {
						return event
					}
					continue
				}
				return event
			}
		}
	}
	return BTPEvent{}
}

func (r *Tracker) monitorFinality(ctx context.Context, network string) {
	fm := r.nMap[network].Adaptor.FinalityMonitor()
	s := fm.Subscribe(DefaultFinalitySubscribeBuffer)
	for {
		select {
		case bi, ok := <-s.C():
			if !ok {
				r.l.Debugf("monitorFinality close channel network:%s", network)
				s = fm.Subscribe(DefaultFinalitySubscribeBuffer)
				continue
			}
			if err := r.finalizeBlock(fm, network, bi); err != nil {
				r.l.Errorf("monitorFinality fail finalizeEvent network:%s err:%+v", network, err)
			}
		case <-ctx.Done():
			r.l.Debugf("monitorFinality done network:%s", network)
			return
		}
	}
}

func (r *Tracker) finalizeBlock(fm contract.FinalityMonitor, network string, bi contract.BlockInfo) error {
	na := r.nMap[network].Options.NetworkAddress //Get NetworkAddress
	return r.br.TransactionWithLock(func(tx *BlockRepository) error {
		var (
			bl []Block
			err error
			finalized bool
			ids []uint
		)
		if bl, err = tx.Find(QueryNetworkAddressAndFinalizedAndHeightBetween,
			na, false, 1, bi.Height()); err != nil {
			return err
		}
		if len(bl) > 0 {
			for _, t := range bl {
				if finalized, err = fm.IsFinalized(t.Height, t.BlockId); err != nil {
					if contract.ErrorCodeNotFoundBlock.Equals(err) {
						return r.dropBlock(tx.DB(), network, t.Height, t.ID)
					}
					return err
				}
				if !finalized {
					return errors.Errorf("invalid FinalityMonitor state for height:%v", t.Height)
				}
				ids = append(ids, t.ID)
			}
			if err = tx.Update(ColumnFinalized, true,
				QueryIdIn, ids); err != nil {
				return err
			}
			return r.finalizeEvent(tx.DB(), ids)
		}
		return nil
	}, network)
}

func (r *Tracker) finalizeEvent(db database.DB, ids []uint) error {
	er, err := r.er.WithDB(db)
	if err != nil {
		return err
	}
	if err = er.Update(ColumnFinalized, true, QueryBlockIdIn, ids); err != nil {
		return err
	}
	return nil
}

func (r *Tracker) dropBlock(db database.DB, network string, height int64, id uint) error {
	r.runMtx.RLock()
	defer r.runMtx.Unlock()
	r.l.Infof("dropBlock network:%s height:%v", network, height)
	n := r.nMap[network]
	if n.Cancel != nil {
		n.Cancel()
	}
	er, err := r.er.WithDB(db)
	if err != nil {
		return err
	}
	if err = er.Delete(QueryBlockId, id); err != nil {
		return err
	}
	br, err := r.br.WithDB(db)
	if err != nil {
		return err
	}
	if err = br.Delete(QueryId, id); err != nil {
		return err
	}
	n.Ctx, n.Cancel = context.WithCancel(r.runCtx)
	go r.monitorEvent(n.Ctx, network, n.EventFilters, n.Options.InitHeight)
	return nil
}

func (r *Tracker) Find(fp tracker.FindParam) (*database.Page[any], error) {
	switch fp.Task {
	case TaskStatus:
		page, err := r.sr.Page(fp.Pageable, fp.Query)
		if err != nil {
			return nil, err
		}
		return page.ToAny(), err
	case TaskEvent:
		page, err := r.er.Page(fp.Pageable, fp.Query)
		if err != nil {
			return nil, err
		}
		return page.ToAny(), err
	case TaskSearch:
		page, err := r.sr.Page(fp.Pageable, fp.Query)
		if err != nil {
			return nil, err
		}
		return page.ToAny(), err
	default:
		return nil, errors.Errorf("not found task:%s", fp.Task)
	}
}

func (r *Tracker) FindOne(fp tracker.FindOneParam) (any, error) {
	switch fp.Task {
	case TaskStatus:
		id := fp.Query[RpId]
		ret, err := r.sr.FindOneByIdWithBtpEvents(id)
		if err != nil {
			return nil, err
		}
		return ret, err
	case TaskSearch:
		src := fp.Query[RpSrc]
		nsn := fp.Query[RpNsn]
		ret, err := r.sr.FindOneBySrcAndNsnWithBtpEvents(src, nsn)
		if err != nil {
			return nil, err
		}
		return ret, err
	default:
		return nil, errors.Errorf("not found task:%s", fp.Task)
	}
}

func (r *Tracker) Summary() ([]any, error) {
	return r.sr.SummaryOfBtpStatusByNetworks()
}

/** Param Name */
const (
	EpSrc   = "_src"
	EpNsn    = "_nsn"
	EpNext     = "_next"
	EpEvent   = "_event"
	EpBlockId = "blockId"
	EpTxId    = "txId"
	RpId = "id"
	RpSrc = "src"
	RpNsn = "nsn"
)
/** Status of BTP Message */
const (
	Unknown = "Unknown"
	Sending = "Sending"
	WaitReply = "WaitReply"
	Replying = "Replying"
	Completed = "Completed"
)