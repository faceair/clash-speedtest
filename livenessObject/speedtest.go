package main

import (
	"net/http"
	"strconv"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/html")
		w.Write([]byte(`<h1>SpeedTest Works</h1>`))
	})
	http.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	zeroBytes := make([]byte, 32*1024)
	for i := 0; i < len(zeroBytes); i++ {
		zeroBytes[i] = '0'
	}

	http.HandleFunc("/_down", func(w http.ResponseWriter, r *http.Request) {
		byteSize, err := strconv.Atoi(r.URL.Query().Get("bytes"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Disposition", "attachment; filename=largefile")
		w.Header().Add("Content-Type", "application/octet-stream")

		batchCount := byteSize / len(zeroBytes)
		for i := 0; i < batchCount; i++ {
			w.Write(zeroBytes)
		}
		w.Write(zeroBytes[:byteSize%len(zeroBytes)])
	})
	http.ListenAndServe(":8080", nil)
}
