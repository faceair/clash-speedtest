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

		zeroBytes := []byte("0")
		for i := 0; i < byteSize; i++ {
			w.Write(zeroBytes)
		}
	})
	http.ListenAndServe(":8080", nil)
}
