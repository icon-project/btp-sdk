package repository

import (
	"github.com/icon-project/btp-sdk/btptracker/storage/models"
	"gorm.io/gorm"
)

func InsertBtpStatus(db *gorm.DB, btpStatus models.BTPStatus) models.BTPStatus {
	result := db.Create(&btpStatus)
	if result.Error != nil {
		println("Failed to add BTPStatus ", btpStatus.Id)
		return models.BTPStatus{}
	}
	return btpStatus
}

func SelectBtpStatusBy(db *gorm.DB, status models.BTPStatus) models.BTPStatus {
	var btpStatus models.BTPStatus
	result := db.Where(status).First(&btpStatus)
	if result.Error != nil {
		return models.BTPStatus{}
	}
	return btpStatus
}

// UpdateBtpStatusSelective : Update LastNetwork, Status, Links and Finalized
// If the value is empty or nil, that column is not updated
func UpdateBtpStatusSelective(db *gorm.DB, btpStatus models.BTPStatus) {
	db.Model(&btpStatus).Updates(models.BTPStatus{
		LastNetwork: btpStatus.LastNetwork,
		Status:      btpStatus.Status,
		Finalized:   btpStatus.Finalized,
		Links:       btpStatus.Links,
	})
}
