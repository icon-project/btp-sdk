package bmc

import (
	"database/sql"

	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service/bmc"
)

const (
	TablePrefix            = "tracker_" + bmc.ServiceName
	BlockTable              = TablePrefix + "_block"
	StatusTable          = TablePrefix + "_btp_status"
	EventTable = TablePrefix + "_btp_event"
	orderByHeightDesc = "height desc"
	orderBySrcAsc = "src asc"

	ColumnFinalized = "finalized"
	QueryNetworkAddressAndFinalizedAndHeightBetween = "network_address = ? AND finalized = ? AND height BETWEEN ? AND ?"
	QueryBlockIdIn = "block_id IN ?"
	QueryBlockId = "block_id = ?"
	QueryId = "id = ?"
	QueryIdIn = "id IN ?"
	QuerySrcAndStatusIsNot = "src = ? AND status != ?"
)

type Block struct {
	database.Model
	BlockId        string    `json:"block_id" gorm:"column:block_id"`
	Height         int64     `json:"height" gorm:"column:height"`
	NetworkName	   string    `json:"network_name" gorm:"column:network_name"`
	NetworkAddress string    `json:"network_address" gorm:"column:network_address"`
	Finalized      bool      `json:"finalized" gorm:"column:finalized"`
}

type BlockRepository struct {
	database.Repository[Block]
}

func NewBlockRepository(db *gorm.DB) (*BlockRepository, error) {
	r, err := database.NewDefaultRepository[Block](db, BlockTable)
	if err != nil {
		return nil, err
	}
	return &BlockRepository{
		Repository: r,
	}, nil
}

func (r *BlockRepository) FindOneByNetworkAddressOrderByHeightDesc(na string) (*Block, error) {
	return r.FindOneWithOrder(orderByHeightDesc, &Block{
		NetworkAddress: na,
	})
}

func (r *BlockRepository) FindOneByNetworkAddressAndHeightAndHash(na, blockId string, height int64) (*Block, error) {
	return r.FindOne(&Block{
		NetworkAddress: na,
		BlockId: blockId,
		Height: height,
	})
}

func (r *BlockRepository) TransactionWithLock(fc func(tx *BlockRepository) error, lock interface{}) error {
	return r.Repository.TransactionWithLock(func(tx database.Repository[Block]) error {
		return fc(r.tx(tx))
	}, lock)
}

func (r *BlockRepository) tx(tx database.Repository[Block]) *BlockRepository {
	return &BlockRepository{
		Repository: tx,
	}
}

type BTPStatusRepository struct {
	*database.DefaultRepository[BTPStatus]
	db *gorm.DB
}

func NewBTPStatusRepository(db *gorm.DB) (*BTPStatusRepository, error) {
	r, err := database.NewDefaultRepository[BTPStatus](db, StatusTable)
	if err != nil {
		return nil, err
	}
	return &BTPStatusRepository{
		DefaultRepository: r,
		db: db,
	}, nil
}

type BTPStatus struct {
	database.Model
	Src         		string         `json:"src" gorm:"column:src"`
	Nsn         		int64          `json:"nsn" gorm:"column:nsn"`
	Status      		sql.NullString `json:"status" gorm:"column:status"`
	LastNetworkName 	sql.NullString `json:"last_network_name" gorm:"column:last_network_name"`
	LastNetworkAddress 	sql.NullString `json:"last_network_address" gorm:"column:last_network_address"`
	Links       		sql.NullString `json:"links" gorm:"column:links"`
	BTPEvents   		[]BTPEvent     `json:"btp_events" gorm:"foreignKey:BtpStatusId;references:id"`
}

type BTPStatusCount struct {
	Src    string
	Status string
	Count  int64
}

type NetworkSummary struct {
	Address    string `json:"network_address"`
	Total      int64  `json:"status_total"`
	InDelivery int64  `json:"status_in_delivery"`
	Completed  int64  `json:"status_completed"`
}

func (r *BTPStatusRepository) FindOneBySrcAndNsn(src string, nsn int64) (*BTPStatus, error) {
	return r.FindOne(BTPStatus{
		Src: src,
		Nsn: nsn,
	})
}

func (r *BTPStatusRepository) FindOneByIdWithBtpEvents(id interface{}) (*BTPStatus, error) {
	var btpStatus *BTPStatus
	result := r.db.Preload("BTPEvents", func(db *gorm.DB) *gorm.DB {
		return db.Table(EventTable).Where("btp_status_id = ?", id).Order(EventTable+".created_at ASC")
	}).Table(StatusTable).Where("id = ?", id).First(&btpStatus)
	if result.Error != nil {
		return nil, result.Error
	}
	return btpStatus, nil
}

func (r *BTPStatusRepository) FindOneBySrcAndNsnWithBtpEvents(src, nsn interface{}) (*BTPStatus, error) {
	var btpStatus *BTPStatus
	result := r.db.Preload("BTPEvents", func(db *gorm.DB) *gorm.DB {
		return db.Table(EventTable).Order(EventTable+".created_at ASC")
	}).Table(StatusTable).Where("src = ? AND nsn = ?", src, nsn).First(&btpStatus)
	if result.Error != nil {
		return nil, result.Error
	}
	return btpStatus, nil
}

func (r *BTPStatusRepository) FindUncompletedBySrc(src string) ([]BTPStatus, error) {
	statuses, err := r.Find(QuerySrcAndStatusIsNot, src, Completed)
	return statuses, err
}

func (r *BTPStatusRepository) SummaryOfBtpStatusByNetworks() ([]any, error) {
	//TODO summary table or view??
	counts := make([]BTPStatusCount, 0)
	result := r.db.Table(StatusTable).Select(
		"src, status, count(*) as count").Group("src, status").Scan(&counts)
	if result.Error != nil {
		return nil, result.Error
	}
	nSummaries := make(map[string]NetworkSummary)
	for _, count := range counts {
		var total, inDelivery, completed int64
		total = count.Count
		switch count.Status {
		case Completed :
			completed = count.Count
		default:
			inDelivery = count.Count
		}
		if summary, ok := nSummaries[count.Src]; ok {
			summary.Total += total
			summary.InDelivery += inDelivery
			summary.Completed += completed
			nSummaries[count.Src] = summary
		} else {
			nSummaries[count.Src] = NetworkSummary{
				Address:    count.Src,
				Total:      total,
				InDelivery: inDelivery,
				Completed:  completed,
			}
		}
	}
	sArr := make([]any, 0, len(nSummaries))
	for _, s := range nSummaries {
		sArr = append(sArr, s)
	}
	return sArr, nil
}

type BTPEventRepository struct {
	*database.DefaultRepository[BTPEvent]
}

func NewBTPEventRepository(db *gorm.DB) (*BTPEventRepository, error) {
	r, err := database.NewDefaultRepository[BTPEvent](db, EventTable)
	if err != nil {
		return nil, err
	}
	return &BTPEventRepository{
		DefaultRepository: r,
	}, nil
}

type BTPEvent struct {
	database.Model
	Src         	string    `json:"src" gorm:"column:src"`
	Nsn         	int64     `json:"nsn" gorm:"column:nsn"`
	Next        	string    `json:"next" gorm:"column:next"`
	Event       	string    `json:"event" gorm:"column:event"`
	BlockId     	uint      `json:"block_id" gorm:"column:block_id"`
	BtpStatusId 	uint      `json:"btp_status_id" gorm:"column:btp_status_id"`
	TxHash      	string    `json:"tx_hash" gorm:"column:tx_hash"`
	EventId     	[]byte    `json:"event_id" gorm:"column:event_id"`
	NetworkName	   	string    `json:"network_name" gorm:"column:network_name"`
	NetworkAddress 	string    `json:"network_address" gorm:"column:network_address"`
	Finalized      	bool      `json:"finalized" gorm:"column:finalized"`
}

func (r *BTPEventRepository) FindBySrcAndNsn(src string, nsn int64) ([]BTPEvent, error) {
	return r.Find(BTPEvent{
		Src: src,
		Nsn: nsn,
	})
}