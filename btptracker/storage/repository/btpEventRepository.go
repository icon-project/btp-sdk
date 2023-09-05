package repository

import (
	"github.com/icon-project/btp-sdk/btptracker/storage/models"
	"gorm.io/gorm"
)

func InsertBtpEvent(db *gorm.DB, btpEvent models.BTPEvent) models.BTPEvent {
	result := db.Create(&btpEvent)
	if result.Error != nil {
		println("Failed to add BTPEvent ", btpEvent.Id)
		return models.BTPEvent{}
	}
	return btpEvent
}

func SelectBtpEventBySrcAndNsn(db *gorm.DB, src string, nsn int64) []models.BTPEvent {
	var btpEvents []models.BTPEvent
	result := db.Order("createdAt asc").Where("src = ? AND nsn = ?", src, nsn).Find(&btpEvents)
	if result.Error != nil {
		println("Failed to get BTPEvents by Src: ? and Nsn: ?", src, nsn)
		return []models.BTPEvent{}
	}
	return btpEvents
}
