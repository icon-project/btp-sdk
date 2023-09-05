package utils

import (
	"gorm.io/gorm"
	"math"
)

type Pageable struct {
	Limit int    `json:"limit,omitempty;query:limit"`
	Page  int    `json:"page,omitempty;query:page"`
	Sort  string `json:"sort,omitempty;query:sort"`
}

type Page struct {
	Pageable   Pageable    `json:"pageable"`
	TotalRows  int64       `json:"total_rows"`
	TotalPages int         `json:"total_pages"`
	Contents   interface{} `json:"contents"`
}

func Paginate(db *gorm.DB, p Pageable, value, condition interface{}) Page {
	var totalRows int64
	db.Model(value).Where(condition).Count(&totalRows)
	totalPages := int(math.Ceil(float64(totalRows) / float64(p.Limit)))
	db.Scopes().Offset(getOffset(p)).Limit(p.Limit).Order(p.Sort).Where(condition).Find(&value)
	return Page{
		Pageable:   p,
		TotalRows:  totalRows,
		TotalPages: totalPages,
		Contents:   value,
	}
}
func getOffset(p Pageable) int {
	return (p.Page - 1) * p.Limit
}
