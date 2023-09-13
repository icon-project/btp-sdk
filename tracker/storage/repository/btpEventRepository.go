package repository

import (
	"gorm.io/gorm"
	"time"
)

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
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	Block       Block     `gorm:"foreignKey:block_id;references:id"`
	BtpStatus   BTPStatus `gorm:"foreignKey:btp_status_id;references:id"`
}

func InsertBtpEvent(db *gorm.DB, btpEvent BTPEvent) BTPEvent {
	result := db.Create(&btpEvent)
	if result.Error != nil {
		println("Failed to add BTPEvent ", btpEvent.Id)
		return BTPEvent{}
	}
	return btpEvent
}

func SelectBtpEventBySrcAndNsn(db *gorm.DB, src string, nsn int64) []BTPEvent {
	var btpEvents []BTPEvent
	result := db.Order("created_at asc").Where("src = ? AND nsn = ?", src, nsn).Find(&btpEvents)
	if result.Error != nil {
		println("Failed to get BTPEvents by Src: ? and Nsn: ?", src, nsn)
		return []BTPEvent{}
	}
	return btpEvents
}
