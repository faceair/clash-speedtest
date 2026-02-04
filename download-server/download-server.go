package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/faceair/clash-speedtest/speedtester"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<h1>SpeedTest Server</h1>`))
	})

	http.HandleFunc("/__down", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		byteSize, err := strconv.Atoi(r.URL.Query().Get("bytes"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=speedtest-%d.bin", byteSize))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}

		reader := speedtester.NewZeroReader(byteSize)
		io.Copy(w, reader)
	})

	http.HandleFunc("/__up", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		io.Copy(io.Discard, r.Body)

		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
