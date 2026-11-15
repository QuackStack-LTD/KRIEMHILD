package httpapi

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Large Worlds contain repeated Age identities and habitat profiles. Stream a
// compressed response so transferring them does not hold the map request gate
// for seconds. Browsers decode gzip transparently; other clients keep JSON.
func respondHistory(w http.ResponseWriter, r *http.Request, status int, value any) {
	w.Header().Add("Vary", "Accept-Encoding")
	if !acceptsHistoryGzip(r.Header.Get("Accept-Encoding")) {
		respond(w, status, value)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")
	writer, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
	w.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
	_ = writer.Close()
}

func acceptsHistoryGzip(header string) bool {
	wildcard := false
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(part, ";")
		encoding := strings.TrimSpace(fields[0])
		quality := 1.0
		for _, parameter := range fields[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(key, "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		if strings.EqualFold(encoding, "gzip") {
			return quality > 0
		}
		if encoding == "*" {
			wildcard = quality > 0
		}
	}
	return wildcard
}
