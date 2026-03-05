package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type GeoIPResult struct {
	Country     string // "DE", "RU", "CN"
	CountryName string // "Germany"
	City        string
}

var (
	geoCache   sync.Map
	geoClient  = &http.Client{Timeout: 2 * time.Second}
	geoLimiter = time.NewTicker(1500 * time.Millisecond) // ~40 req/min safe
)

type ipAPIResponse struct {
	Status      string `json:"status"`
	CountryCode string `json:"countryCode"`
	Country     string `json:"country"`
	City        string `json:"city"`
}

// LookupGeoIP returns cached result immediately or empty result if not cached.
// Call ResolveGeoIP in a goroutine to populate the cache.
func LookupGeoIP(ip string) (GeoIPResult, bool) {
	if v, ok := geoCache.Load(ip); ok {
		return v.(GeoIPResult), true
	}
	return GeoIPResult{}, false
}

// ResolveGeoIP fetches geo data for an IP and caches it.
// Safe to call from a goroutine. Respects rate limiting.
func ResolveGeoIP(ip string) GeoIPResult {
	if v, ok := geoCache.Load(ip); ok {
		return v.(GeoIPResult)
	}

	// Rate limit
	<-geoLimiter.C

	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode,country,city", ip)
	resp, err := geoClient.Get(url)
	if err != nil {
		return GeoIPResult{}
	}
	defer resp.Body.Close()

	var data ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || data.Status != "success" {
		return GeoIPResult{}
	}

	result := GeoIPResult{
		Country:     data.CountryCode,
		CountryName: data.Country,
		City:        data.City,
	}
	geoCache.Store(ip, result)
	return result
}

// ClearGeoCache clears the cache (for testing).
func ClearGeoCache() {
	geoCache.Range(func(key, _ any) bool {
		geoCache.Delete(key)
		return true
	})
}
