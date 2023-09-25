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
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/tracker"
)

const (
	CallMethodGetNetworkAddress = "getNetworkAddress"
	EventBTP                    = "BTPEvent"
	BTPInDelivery               = "inDelivery"
	BTPCompleted                = "completed"

	SEND    = "SEND"
	ROUTE   = "ROUTE"
	RECEIVE = "RECEIVE"
	REPLY   = "REPLY"
	DROP    = "DROP"
	ERROR   = "ERROR"
)

const (
	subscriptionBufferSize = 5
)

func init() {
	tracker.RegisterFactory(bmc.ServiceName, NewTracker)
}

type Tracker struct {
	db        *gorm.DB
	s         service.Service
	nMap      map[string]Network
	fmMap     map[string]contract.FinalityMonitor
	runCancel context.CancelFunc
	runMtx    sync.RWMutex
	l         log.Logger

	br *BlockRepository
	sr *BTPStatusRepository
	er *BTPEventRepository
	ah *TrackerAPIHandler
}

type Network struct {
	NetworkType  string
	Adaptor      contract.Adaptor
	Options      TrackerOptions
	EventFilters []contract.EventFilter
}

type TrackerOptions struct {
	InitHeight     int64              `json:"init_height"`
	NetworkAddress string             `json:"network_address"`
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
		for toNetwork, tn := range nMap {
			if toNetwork == fromNetwork {
				continue
			} else {
				srcParams = append(srcParams, contract.Params{
					"_src": tn.Options.NetworkAddress,
				})
			}
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

	fmMap := make(map[string]contract.FinalityMonitor)
	for network, n := range nMap {
		fmMap[network] = n.Adaptor.FinalityMonitor()
	}

	//TODO set repository
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
	ah, err := NewTrackerAPIHandler(sr)

	return &Tracker{
		db:    db,
		s:     s,
		nMap:  nMap,
		fmMap: fmMap,
		l:     l,

		br: br,
		sr: sr,
		er: er,
		ah: ah,
	}, nil
}

func (r *Tracker) DB() *gorm.DB {
	return r.db
}

func (r *Tracker) Name() string {
	return r.s.Name()
}

func (r *Tracker) Start() error {
	r.runMtx.Lock()
	defer r.runMtx.Unlock()
	if r.runCancel != nil {
		return errors.Errorf("already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	for network, n := range r.nMap {
		height, err := r.getMonitorHeight(network)
		if err != nil {
			cancel()
			return err
		}
		go r.monitorEvent(ctx, network, n.EventFilters, height)

		go r.finalityMonitor(ctx, network)
	}
	r.runCancel = cancel
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

func (r *Tracker) monitorEvent(ctx context.Context, network string, efs []contract.EventFilter, height int64) {
	r.l.Debugf("monitorEvent network:%s height:%d", network, height)
	for {
		select {
		case <-time.After(time.Second):
			if err := r.s.MonitorEvent(ctx, network, func(e contract.Event) error {
				r.l.Tracef("monitorEvent callback network:%s event:%s height:%s", network, e.Name(), e.BlockHeight())
				switch e.Name() {
				case EventBTP:
					return r.handleBTPEvent(network, e) // TODO handle BTP Event
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

func (r *Tracker) handleBTPEvent(network string, e contract.Event) error {
	na := r.getNetworkAddress(network)
	hash, err := getBlockId(e)
	height := e.BlockHeight()

	b, err := r.storeBlock(na, hash, height)
	if err != nil {
		return err
	}
	
	_, err = r.storeBtpStatus(na, b.Id, e)
	if err != nil {
		return err
	}
	return nil
}

func getSrc(e contract.Event) (string, error) {
	p := e.Params()["_src"]
	log.Debugf("_src type: %T value: %v", p, p)
	src, err := contract.StringOf(eventIndexedValue(p))
	if err != nil {
		return "", err
	}
	return string(src), nil
}

func getBlockId(e contract.Event) (string, error) {
	p := e.BlockID()
	log.Debugf("_blockId type: %T value: %v", p, p)
	bId, err := contract.StringOf(eventIndexedValue(p))
	if err != nil {
		return "", err
	}
	return string(bId), nil
}

func getTxHash(e contract.Event) (string, error) {
	p := e.TxID()
	log.Debugf("_txId type: %T value: %v", p, p)
	tId, err := contract.StringOf(eventIndexedValue(p))
	if err != nil {
		return "", err
	}
	return string(tId), nil
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

func (r *Tracker) storeBlock(na, hash string, height int64) (*Block, error) {
	b, err := r.br.FindOneByNetworkAddressAndHeightAndHash(na, hash, height)
	if b == nil {
		err = r.br.SaveBlock(Block{
			NetworkAddress: na,
			BlockHash:      hash,
			Height:         height,
			Finalized:      false,
		})
		if err == nil {
			b, _ = r.br.FindOneByNetworkAddressAndHeightAndHash(na, hash, height)
		}
	}
	return b, nil
}

func (r *Tracker) storeBtpStatus(na string, bId int, e contract.Event) (BTPStatus, error) {
	src, err := getSrc(e)
	if err != nil {
		return BTPStatus{}, errors.Errorf("invalid _src type:%T", e.Params()["_src"])
	}
	nsn, _ := e.Params()["_nsn"].(contract.Integer).AsInt64()

	// If btpStatus does not exist, it is created first.
	bs, _ := r.sr.FindOneBySrcAndNsn(src, nsn)
	if bs == nil {
		err = r.sr.SaveBtpStatus(BTPStatus{
			Src:         src,
			LastNetwork: sql.NullString{String: na, Valid: true},
			Status:      sql.NullString{String: BTPInDelivery, Valid: true},
			Links:       sql.NullString{String: "", Valid: true},
			Nsn:         nsn,
			Finalized:   false,
		})
		if err == nil {
			bs, _ = r.sr.FindOneBySrcAndNsn(src, nsn)
		}
	}

	// Insert BTP Event with block id and btp status id
	next := string(e.Params()["_next"].(contract.String))
	be := string(e.Params()["_event"].(contract.String))

	txHash, err := getTxHash(e)
	if err != nil {
		return BTPStatus{}, errors.Errorf("invalid _src type:%T", e.Params()["_src"])
	}
	identifier := []byte(strconv.Itoa(e.Identifier()))
	err = r.er.SaveBtpEvent(BTPEvent{
		Src:         src,
		Nsn:         nsn,
		Next:        next,
		Event:       be,
		BlockId:     bId,
		BtpStatusId: bs.Id,
		TxHash:      txHash,
		EventId:     identifier,
		OccurredIn:  na,
	})
	if err != nil {
		return BTPStatus{}, nil
	}
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
	bs.LastNetwork = sql.NullString{
		String: na,
		Valid:  true,
	}
	err = r.sr.SaveBtpStatus(*bs)
	if err != nil {
		return BTPStatus{}, err
	}
	return *bs, nil
}

func (r *Tracker) getNetworkAddress(network string) string {
	rv, _ := r.s.Call(network, CallMethodGetNetworkAddress, nil, nil)
	return string(rv.(contract.String))
}

func (r *Tracker) getLinks(src string, nsn int64) ([]int, string) {
	links := make([]int, 0)
	status := BTPInDelivery
	//Find BTPEvents by src and nsn, order by createdAt(time) asc
	events, err := r.er.FindBySrcAndNsn(src, nsn)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Debugf("No results were found, src: %s, nsn: %s", src, nsn)
		}
	}
	if len(events) == 0 {
		return links, status
	}

	//Find first link that BTP event value is "SEND", The starting point
	curLink, startingPoint := findStartingEvent(events)
	if reflect.DeepEqual(BTPEvent{}, curLink) {
		return links, status
	}
	links = append(links, curLink.Id)
	events, _ = trimBTPEvents(events, startingPoint)

	//Find next links, after find "SEND" event
	for i := 0; i < len(events); i++ {
		//Find next link by BTPEvent.Next and BTPEvent.Event
		//If next link does not exist, break loop and return links
		candidates := findCandidatesForNextEvent(events, curLink)
		if candidates == nil {
			break
		}

		if len(candidates) == 1 {
			curLink = candidates[0]
		} else if len(candidates) > 1 {
			//If there is multiple next events cause same occurredAt(networkAddress),
			//Check which event happened first.
			curLink = findNextEvent(candidates, links)
		}
		links = append(links, curLink.Id)
		events, i = trimBTPEvents(events, i)

		// Reset tmpEvents
		candidates = nil

		// It means the transfer closed. so return links
		if isTransferClosed(curLink) {
			status = BTPCompleted
			break
		}
	}
	return links, status
}

func trimBTPEvents(events []BTPEvent, index int) ([]BTPEvent, int) {
	events = append(events[:index], events[index+1:]...)
	index--
	return events, index
}

// If current link's event is "DROP" or "RECEIVE", BTPEvent.Next is empty string.
func isTransferClosed(event BTPEvent) bool {
	isClosed := false
	if event.Next == "" {
		if event.Event == DROP || (event.Event == RECEIVE && event.Next == "") {
			isClosed = true
		}
	}
	return isClosed
}

func findStartingEvent(events []BTPEvent) (BTPEvent, int) {
	var curLink BTPEvent
	var index int
	for i := 0; i < len(events); i++ {
		if events[i].Event == SEND {
			curLink = events[i]
			index = i
			break
		}
	}
	return curLink, index
}

func findCandidatesForNextEvent(events []BTPEvent, curLink BTPEvent) []BTPEvent {
	var nextEvents []BTPEvent = nil
	for _, event := range events {
		if curLink.Event == RECEIVE {
			if event.Event == REPLY || event.Event == ERROR || event.Event == DROP {
				nextEvents = append(nextEvents, event)
				break
			}
			continue
		} else if curLink.Next == event.OccurredIn {
			nextEvents = append(nextEvents, event)
		}
	}
	return nextEvents
}

func findNextEvent(candidates []BTPEvent, links []int) BTPEvent {
	//If Event is "ROUTE" or "RECEIVE", compare createdAt(time)
	if candidates[0].Event == ROUTE || candidates[0].Event == RECEIVE {
		checkVal := false
		for _, link := range links {
			if link == candidates[0].Id {
				checkVal = true
			}
		}

		if checkVal {
			return candidates[1]
		} else {
			return candidates[0]
		}
	}
	return BTPEvent{}
}

func (r *Tracker) finalityMonitor(ctx context.Context, network string) {
	r.l.Debugf("FinalityBlock network: %s", network)
	n := r.nMap[network]
	fm := r.fmMap[network]

	subscribe:
	s := fm.Subscribe(subscriptionBufferSize)
	for {
		select {
		case bi, opened := <-s.C():
			r.l.Debugf("Subscribe bi:%+v, opened:%v network: %s", bi, opened, network)
			if !opened {
				r.l.Debugf("Subscribe closed, network: %s", network)
				goto subscribe
			}
			err := r.handleFinalizeBlock(n, fm, bi)
			if err != nil {
				return
			}
		case <-ctx.Done():
			s.Unsubscribe()
			r.l.Debugf("FinalityBlock context done, unsubscribe: %s", network)
			return
		}
	}
}

func (r *Tracker) handleFinalizeBlock(network Network, fm contract.FinalityMonitor, info contract.BlockInfo) error {
	na := network.Options.NetworkAddress
	blocks, err := r.br.FindByNetworkAddressOrderByHeightDesc(
		fmt.Sprintf("network_address = '%s' AND height <= %d AND finalized = %t", na, info.Height(), false))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Debugf("No results were found, network: %s, height: %s", network, info.Height())
		}
	}
	for _, block := range blocks {
		err = r.finalizeBlock(fm, block)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Tracker) finalizeBlock(fm contract.FinalityMonitor, block Block) error {
	result, err := fm.IsFinalized(block.Height, block.BlockHash)
	if err != nil {
		if err == contract.ErrMismatchBlockID {
			//TODO handle drop block and btp event
		}
		return err
	}
	if result {
		block.Finalized = true
		err = r.br.SaveBlock(block)
	}
	return err
}

func (r *Tracker) APIHandler() *TrackerAPIHandler {
	return r.ah
}