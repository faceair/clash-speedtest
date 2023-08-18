package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"io"
	"net/http"
	"strconv"
)

func main() {
	StartServ()
}

func StartServ() {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, `
			<h1>SpeedTest Working....</h1>
		`)
	})
	e.Any("/liveness", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	e.GET("/_down", GenBigSizeHandler)
	e.Logger.Fatal(e.Start(":8080"))
}

func GenBigSizeHandler(c echo.Context) error {
	u := c.QueryParams()
	b := u.Get("bytes")
	bytes, err := strconv.ParseInt(b, 10, 64)
	if err != nil {
		return err
	}

	c.Response().Header().Set("Content-Disposition", "attachment; filename=largefile")
	c.Response().Header().Set("Content-Type", "application/octet-stream")

	zeroReader := &ZeroReader{}

	_, err = io.CopyN(c.Response().Writer, zeroReader, bytes)
	if err != nil {
		return err
	}

	c.Response().Flush()
	return c.NoContent(http.StatusNoContent)
}

// ZeroReader 是一个只生成零值字节的 Reader
type ZeroReader struct{}

func (z *ZeroReader) Read(p []byte) (int, error) {
	// 把 p 填充满零值字节
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
