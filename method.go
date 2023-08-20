package main

import (
	"context"
	"fmt"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	CanNotDownload = iota
	CanDownload
	Unknown
)

func GenerateAcquireRequestClient(proxy C.Proxy, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				return proxy.DialContext(ctx, &C.Metadata{
					Host:    host,
					DstPort: port,
				})
			},
		},
	}
}

func IsStreamBlock(objectURL string, proxy C.Proxy, timeout time.Duration) func() (bool, bool) {
	var res *http.Response
	var err error
	var canDownload = Unknown

	fmt.Println("speed object : ", objectURL)
	client := GenerateAcquireRequestClient(proxy, timeout)
	res, err = client.Get(objectURL)
	if err != nil {
		// 如果这个代理获取错误，则转移给下一个代理进行判断
		res = &http.Response{}
		return func() (bool, bool) {
			return false, false
		}
	}

	// 如果对象地址为以下几个状态，则意味着探测完全无法进行
	for _, hitStatusCode := range []int{
		http.StatusNotFound,
		http.StatusBadGateway,
		http.StatusMethodNotAllowed,
		http.StatusRequestURITooLong,
	} {
		if res.StatusCode == hitStatusCode {
			log.Fatalln("the target address is abnormal, please change the object again")
		}
	}

	canDownload = CanNotDownload
	hValue := func(hKey string) string {
		return strings.Join(res.Header[hKey], ",")
	}
	for _, v := range []string{
		"application/octet-stream",
		"application/x-iso9660-image",
	} {
		// go 的 http 原生库会将 header 的 key 转换为大小写
		if strings.Contains(hValue("Content-Type"), v) {
			canDownload = CanDownload
		}
	}

	return func() (bool, bool) {
		if canDownload == Unknown {
			return IsStreamBlock(objectURL, proxy, timeout)()
		}
		return true, canDownload == CanDownload
	}
}

func TestProxyConcurrent(
	name string, proxy C.Proxy, objectURL string, timeout time.Duration, concurrentCount int, isStreamBlock bool) *Result {
	if concurrentCount <= 0 {
		concurrentCount = 1
	}

	totalTTFB := int64(0)
	downloaded := int64(0)

	var wg sync.WaitGroup
	result := &Result{name, 0, 0}

	start := time.Now()
	if !isStreamBlock {
		_, meanDelay, err := proxy.URLTest(context.Background(), objectURL)
		if err != nil {
			return result
		}
		result.TTFB = time.Duration(meanDelay) * time.Millisecond
	} else {
		// 块下载测速方式采用正常的多线程
		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(i int) {
				childResult, w := TestProxy(name, proxy, objectURL, timeout)
				if w != 0 {
					atomic.AddInt64(&downloaded, w)
					atomic.AddInt64(&totalTTFB, int64(childResult.TTFB))
				}
				wg.Done()
			}(i)
		}
		wg.Wait()

		downloadTime := time.Since(start)

		result = &Result{
			Name:      name,
			Bandwidth: float64(downloaded) / downloadTime.Seconds(),
			TTFB:      time.Duration(totalTTFB / int64(concurrentCount)),
		}
	}

	return result
}

func TestProxy(name string, proxy C.Proxy, objectURL string, timeout time.Duration) (*Result, int64) {
	client := GenerateAcquireRequestClient(proxy, timeout)
	start := time.Now()

	resp, err := client.Get(objectURL)
	if err != nil {
		return &Result{name, -1, -1}, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode-http.StatusOK > 100 {
		return &Result{name, -1, -1}, 0
	}
	ttfb := time.Since(start)

	written, _ := io.Copy(io.Discard, resp.Body)
	if written == 0 {
		return &Result{name, -1, -1}, 0
	}
	downloadTime := time.Since(start) - ttfb
	bandwidth := float64(written) / downloadTime.Seconds()

	return &Result{name, bandwidth, ttfb}, written
}

func WithUseDownloadBlock(name string, proxy C.Proxy, objectURL string, timeout time.Duration) {

}
