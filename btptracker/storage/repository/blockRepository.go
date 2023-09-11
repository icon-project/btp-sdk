package repository

import (
	"gorm.io/gorm"
	"time"
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

func InsertBlock(db *gorm.DB, block Block) (Block, error) {
	result := db.Create(&block)
	if result.Error != nil {
		return block, result.Error
	}
	return block, nil
}

func SelectLastBlockBySrc(db *gorm.DB, networkAddress string) Block {
	var block Block
	result := db.Order("height desc").Where(Block{
		NetworkAddress: networkAddress,
	}).First(&block)
	if result.Error != nil {
		println("Failed to get Block by last height")
		return Block{}
	}
	return block
}

func FindBlocksByHeight(db *gorm.DB, networkAddress string, height int64, finalized bool) []Block {
	var blocks []Block
	result := db.Order("height desc").Where("network_address = ? AND height <= ? AND finalized = ?", networkAddress, height, finalized).Find(&blocks)
	if result.Error != nil {
		println("Failed to get Block by height")
		return []Block{}
	}
	return blocks
}

func UpdateBlockBySelective(db *gorm.DB, block Block, chg Block) Block {
	result := db.Model(&block).Updates(chg)
	if result.Error != nil {
		return Block{}
	}
	return block
}

func SelectBlockBy(db *gorm.DB, block Block) Block {
	var b Block
	result := db.Where(block).First(&b)
	if result.Error != nil {
		return Block{}
	}
	return b
}
