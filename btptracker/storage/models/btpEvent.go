package models

import "time"

type BTPEvent struct {
	Id          int       `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	Src         string    `gorm:"column:src"`
	Nsn         int64     `gorm:"column:nsn"`
	Next        string    `gorm:"column:next"`
	Event       string    `gorm:"column:event"`
	BlockId     int       `gorm:"column:blockId"`
	Block       Block     `gorm:"foreignKey:blockId;references:id"`
	BtpStatusId int       `gorm:"column:btpStatusId"`
	BtpStatus   BTPStatus `gorm:"foreignKey:btpStatusId;references:id"`
	TxHash      string    `gorm:"column:txHash"`
	EventId     []byte    `gorm:"column:eventId"`
	OccurredIn  string    `gorm:"column:occurredIn"`
	CreatedAt   time.Time `gorm:"column:createdAt;autoCreateTime"`
}

func NewBTPEvent(src, next, event, occurredIn string, nsn int64, blockId, btpStatusId int, txHash string, eventId []byte) *BTPEvent {
	return &BTPEvent{
		Src:         src,
		Nsn:         nsn,
		Next:        next,
		Event:       event,
		BlockId:     blockId,
		BtpStatusId: btpStatusId,
		TxHash:      txHash,
		EventId:     eventId,
		OccurredIn:  occurredIn,
	}
}
