package models

import (
	"database/sql"
	"time"
)

type BTPStatus struct {
	Id          int            `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	Src         string         `gorm:"column:src"`
	Nsn         int64          `gorm:"column:nsn"`
	LastNetwork sql.NullString `gorm:"column:lastNetwork"`
	Status      sql.NullString `gorm:"column:status"`
	Finalized   bool           `gorm:"column:finalized"`
	Links       sql.NullString `gorm:"column:links"`
	CreatedAt   time.Time      `gorm:"column:createdAt;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"column:updatedAt;autoUpdateTime"`
}

func NewBTPStatus(src, lastNetwork, status string, links string, nsn int64, finalized bool) *BTPStatus {
	return &BTPStatus{
		Src:         src,
		Nsn:         nsn,
		LastNetwork: sql.NullString{String: lastNetwork, Valid: true},
		Status:      sql.NullString{String: status, Valid: true},
		Finalized:   finalized,
		Links:       sql.NullString{String: links, Valid: true},
	}
}
