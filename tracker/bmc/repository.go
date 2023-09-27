package bmc

import (
	"database/sql"
	"time"

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
)

type Block struct {
	Id             int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	NetworkAddress string    `gorm:"column:network_address" json:"network_address"`
	BlockHash      string    `gorm:"column:block_hash" json:"block_hash"`
	Height         int64     `gorm:"column:height" json:"height"`
	Finalized      bool      `gorm:"column:finalized" json:"finalized"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

type BlockRepository struct {
	*database.DefaultRepository[Block]
}

func NewBlockRepository(db *gorm.DB) (*BlockRepository, error) {
	r, err := database.NewDefaultRepository[Block](db, BlockTable)
	if err != nil {
		return nil, err
	}
	return &BlockRepository{
		DefaultRepository: r,
	}, nil
}

func (r *BlockRepository) SaveBlock(block Block) error {
	return r.Save(&block)
}

func (r *BlockRepository) FindOneByNetworkAddressOrderByHeightDesc(na string) (*Block, error) {
	return r.FindOneWithOrder(orderByHeightDesc, &Block{
		NetworkAddress: na,
	})
}

func (r *BlockRepository) FindByNetworkAddressOrderByHeightDesc(query interface{}) ([]Block, error) {
	return r.FindWithOrder(orderByHeightDesc, query)
}

func (r *BlockRepository) FindOneByNetworkAddressAndHeightAndHash(na, hash string, height int64) (*Block, error) {
	return r.FindOne(&Block{
		NetworkAddress: na,
		BlockHash: hash,
		Height: height,
	})
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
	Id          int            `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	Src         string         `gorm:"column:src" json:"src"`
	Nsn         int64          `gorm:"column:nsn" json:"nsn"`
	LastNetwork sql.NullString `gorm:"column:last_network" json:"last_network"`
	Status      sql.NullString `gorm:"column:status" json:"status"`
	Finalized   bool           `gorm:"column:finalized" json:"finalized"`
	Links       sql.NullString `gorm:"column:links" json:"links"`
	CreatedAt   time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

type BTPStatusCount struct {
	Src    string
	Status string
	Count  int64
}

type NetworkSummary struct {
	Address    string `json:"network_address"`
	Name       string `json:"network_name"`
	Total      int64  `json:"status_total"`
	InDelivery int64  `json:"status_in_delivery"`
	Completed  int64  `json:"status_completed"`
}

func (r *BTPStatusRepository) SaveBtpStatus(btpStatus BTPStatus) error {
	return r.Save(&btpStatus)
}

func (r *BTPStatusRepository) FindOneBySrcAndNsn(src string, nsn int64) (*BTPStatus, error) {
	return r.FindOne(BTPStatus{
		Src: src,
		Nsn: nsn,
	})
}

func (r *BTPStatusRepository) SummaryOfBtpStatusByNetworks() ([]NetworkSummary, error) {
	//TODO summary table or view??
	counts := make([]BTPStatusCount, 0)
	result := r.db.Table(StatusTable).Select(
		"src, status, count(*) as count").Group("src, status").Order(orderBySrcAsc).Scan(&counts)
	if result.Error != nil {
		return nil, result.Error
	}
	//TODO More specific level about btp status
	nSummaries := make(map[string]NetworkSummary, 0)
	for _, count := range counts {
		var inDelivery, completed int64
		switch count.Status {
		case BTPInDelivery:
			inDelivery = count.Count
		case BTPCompleted:
			completed = count.Count
		case "ERROR", "DONE":
		}
		summary, ok := nSummaries[count.Src]
		if ok {
			summary.Total += count.Count
			summary.InDelivery += inDelivery
			summary.Completed += completed
		} else {
			nSummaries[count.Src] = NetworkSummary{
				Address:    count.Src,
				Name:       count.Src,
				Total:      count.Count,
				InDelivery: inDelivery,
				Completed:  completed,
			}
		}
	}

	sArr := make([]NetworkSummary, 0, len(nSummaries))
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
	Id          int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	Src         string    `gorm:"column:src" json:"src"`
	Nsn         int64     `gorm:"column:nsn" json:"nsn"`
	Next        string    `gorm:"column:next" json:"next"`
	Event       string    `gorm:"column:event" json:"event"`
	BlockId     int       `gorm:"column:block_id" json:"block_id"`
	BtpStatusId int       `gorm:"column:btp_status_id" json:"btp_status_id"`
	TxHash      string    `gorm:"column:tx_hash" json:"tx_hash"`
	EventId     []byte    `gorm:"column:event_id" json:"event_id"`
	OccurredIn  string    `gorm:"column:occurred_in" json:"occurred_in"`
	CreatedAt time.Time   `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	//Block     Block       `gorm:"foreignKey:block_id;references:id"`
	//BtpStatus BTPStatus   `gorm:"foreignKey:btp_status_id;references:id"`
}

func (r *BTPEventRepository) SaveBtpEvent(btpEvent BTPEvent) error {
	return r.Save(&btpEvent)
}

func (r *BTPEventRepository) FindBySrcAndNsn(src string, nsn int64) ([]BTPEvent, error) {
	return r.Find(BTPEvent{
		Src: src,
		Nsn: nsn,
	})
}

