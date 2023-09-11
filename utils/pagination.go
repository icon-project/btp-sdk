package utils

import (
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"math"
	"strconv"
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

func Paginate(db *gorm.DB, p Pageable, value, condition interface{}) (Page, error) {
	var totalRows int64
	db.Model(value).Where(condition).Count(&totalRows)
	totalPages := int(math.Ceil(float64(totalRows) / float64(p.Limit)))
	db.Scopes().Offset(getOffset(p)).Limit(p.Limit).Order(p.Sort).Where(condition).Find(&value)
	return Page{
		Pageable:   p,
		TotalRows:  totalRows,
		TotalPages: totalPages,
		Contents:   value,
	}, nil
}
func getOffset(p Pageable) int {
	return (p.Page - 1) * p.Limit
}

func DefaultPageable() Pageable {
	return Pageable{
		Limit: 10,
		Page:  1,
		Sort:  "created_at desc",
	}
}

func GetPageableFromRequest(c echo.Context) Pageable {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 10
	}
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	sort := c.QueryParam("sort")
	if sort == "" {
		sort = "created_at desc"
	}
	return Pageable{
		Limit: limit,
		Page:  page,
		Sort:  sort,
	}
}
