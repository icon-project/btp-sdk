package repository

import (
	"database/sql"
	"gorm.io/gorm"
	"time"
)

const (
	BTP_STATUSES  = "btp_statuses"
	BTPInDelivery = "inDelivery"
	BTPCompleted  = "completed"
)

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

func InsertBtpStatus(db *gorm.DB, btpStatus BTPStatus) BTPStatus {
	result := db.Create(&btpStatus)
	if result.Error != nil {
		println("Failed to add BTPStatus ", btpStatus.Id)
		return BTPStatus{}
	}
	return btpStatus
}

func SelectBtpStatusBy(db *gorm.DB, status BTPStatus) (BTPStatus, error) {
	var btpStatus BTPStatus
	result := db.Where(status).First(&btpStatus)
	if result.Error != nil {
		return BTPStatus{}, result.Error
	}
	return btpStatus, nil
}

// UpdateBtpStatusSelective : Update LastNetwork, Status, Links and Finalized
// If the value is empty or nil, that column is not updated
func UpdateBtpStatusSelective(db *gorm.DB, btpStatus BTPStatus) {
	db.Model(&btpStatus).Updates(BTPStatus{
		LastNetwork: btpStatus.LastNetwork,
		Status:      btpStatus.Status,
		Finalized:   btpStatus.Finalized,
		Links:       btpStatus.Links,
	})
}

func SummaryOfBtpStatusByNetworks(db *gorm.DB) ([]NetworkSummary, error) {
	counts := make([]BTPStatusCount, 0)
	result := db.Table(BTP_STATUSES).Select(
		"src, status, count(*) as count").Group("src, status").Order("src asc").Scan(&counts)
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

func GetBtpStatusBySrcAndNsn(db *gorm.DB, src string, nsn int64) (BTPStatus, error) {
	var btpStatus BTPStatus
	result := db.Where("src = ? AND nsn = ?", src, nsn).First(&btpStatus)
	if result.Error != nil {
		println("Failed to get BTPStatus by ? and ?", src, nsn)
		return BTPStatus{}, result.Error
	}
	return btpStatus, nil
}
