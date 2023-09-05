package repository

import (
	"github.com/icon-project/btp-sdk/btptracker/storage/models"
	"gorm.io/gorm"
)

func InsertBlock(db *gorm.DB, block models.Block) (models.Block, error) {
	result := db.Create(&block)
	if result.Error != nil {
		return block, result.Error
	}
	return block, nil
}

func SelectLastBlockBySrc(db *gorm.DB, networkAddress string) models.Block {
	var block models.Block
	result := db.Order("height desc").Where(models.Block{
		NetworkAddress: networkAddress,
	}).First(&block)
	if result.Error != nil {
		println("Failed to get Block by last height")
		return models.Block{}
	}
	return block
}

func FindBlocksByHeight(db *gorm.DB, networkAddress string, height int64, finalized bool) []models.Block {
	var blocks []models.Block
	result := db.Order("height desc").Where("networkAddress = ? AND height <= ? AND finalized = ?", networkAddress, height, finalized).Find(&blocks)
	if result.Error != nil {
		println("Failed to get Block by height")
		return []models.Block{}
	}
	return blocks
}

func UpdateBlockBySelective(db *gorm.DB, block models.Block, chg models.Block) models.Block {
	result := db.Model(&block).Updates(chg)
	if result.Error != nil {
		return models.Block{}
	}
	return block
}

func SelectBlockBy(db *gorm.DB, block models.Block) models.Block {
	var b models.Block
	result := db.Where(block).First(&b)
	if result.Error != nil {
		return models.Block{}
	}
	return b
}
