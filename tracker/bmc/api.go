package bmc

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/icon-project/btp-sdk/database"
)

const (
	PathStatus = "/status"
	PathLatest = "/latest"
	PathSummary = "/summary"
	PathSearch = "/search"
	PathParamId = "id"
	QueryStringSrc = "src"
	QueryStringNsn = "nsn"
	QueryStringSize = "size"
	QueryStringPage = "page"
	QueryStringSort = "sort"

	CreatedAtDesc = "created_at desc"
)

type TrackerAPIHandler struct {
	sr *BTPStatusRepository
}

func NewTrackerAPIHandler(sr *BTPStatusRepository) (*TrackerAPIHandler, error) {
	return &TrackerAPIHandler{
		sr: sr,
	}, nil
}

func (a *TrackerAPIHandler)RegisterTrackerHandler(g *echo.Group) {

	g.GET(PathSummary, func(c echo.Context) error {
		ret, err := a.sr.SummaryOfBtpStatusByNetworks()
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
	g.GET(PathSearch, func(c echo.Context) error {
		src := c.QueryParam(QueryStringSrc)
		strNsn := c.QueryParam(QueryStringNsn)
		nsn, _ := strconv.ParseInt(strNsn, 10, 64)
		ret, err := a.sr.FindOneBySrcAndNsn(src, nsn)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
	g.GET(fmt.Sprint(PathStatus+PathLatest), func(c echo.Context) error {
		p := database.Pageable{
			Page: 1,
			Size: 10,
			Sort: CreatedAtDesc,
		}
		result, err := a.sr.Page(p, nil)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, result)
	})
	g.GET(PathStatus, func(c echo.Context) error {
		size, _ := strconv.Atoi(c.QueryParam(QueryStringSize))
		if size <= 0 {
			size = 10
		}
		page, _ := strconv.Atoi(c.QueryParam(QueryStringPage))
		if page <= 0 {
			page = 1
		}
		sort := c.QueryParam(QueryStringSort)
		if sort == "" {
			sort = CreatedAtDesc
		}
		p := database.Pageable{
			Size: uint(size),
			Page: uint(page),
			Sort: sort,
		}
		result, err := a.sr.Page(p, nil)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, result)
	})
	g.GET(fmt.Sprint(PathStatus+"/:"+PathParamId), func(c echo.Context) error {
		id, _ := strconv.Atoi(c.Param(PathParamId))
		ret, err := a.sr.FindOne(BTPStatus{
			Id: id,
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, ret)
	})
}