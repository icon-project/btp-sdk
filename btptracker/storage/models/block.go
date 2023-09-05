package models

import "time"

type Block struct {
	Id             int       `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	NetworkAddress string    `gorm:"column:networkAddress"`
	BlockHash      string    `gorm:"column:blockHash"`
	Height         int64     `gorm:"column:height"`
	Finalized      bool      `gorm:"column:finalized"`
	CreatedAt      time.Time `gorm:"column:createdAt;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updatedAt;autoUpdateTime"`
}

func NewBlock(networkAddress string, height int64, blockHash string, finalized bool) *Block {
	return &Block{
		NetworkAddress: networkAddress,
		BlockHash:      blockHash,
		Height:         height,
		Finalized:      finalized,
	}
}
