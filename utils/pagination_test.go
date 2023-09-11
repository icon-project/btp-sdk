package utils

import (
	"fmt"
	"github.com/icon-project/btp-sdk/btptracker/storage/repository"
	"gorm.io/gorm"
	"testing"
)

func Test_Pagination(t *testing.T) {
	db := getDB()

	p := Pageable{
		Limit: 10,
		Page:  1,
		Sort:  "created_at asc",
	}
	var btpStatus []*repository.BTPStatus

	page, _ := Paginate(db, p, btpStatus, repository.BTPStatus{})
	fmt.Println("Page: ", page)
}

func Test_Summary(t *testing.T) {
	db := getDB()

	summaries, _ := repository.SummaryOfBtpStatusByNetworks(db)
	for _, s := range summaries {
		fmt.Println(s)
	}
}

func getDB() *gorm.DB {
	cfg := &StorageConfig{
		DBType:   "mysql",
		DBName:   "btpTracker",
		UserName: "by.kim",
		Password: "11732188",
		HostName: "localhost:3306",
	}

	db, _ := NewStorage(cfg)
	return db
}
