package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	// _ "github.com/go-sql-driver/mysql"
	"io"
	"log"
	"net/http"
)

var db *sql.DB

func main() {
	var config Config
	dev := os.Getenv("FETCHER_IS_DEV")
	if dev != "" {
		config = fetchConfig("./config.json")
	} else {
		if len(os.Args) < 2 {
			log.Fatal("usage: cso-poller <config-path>")
		}
		config = fetchConfig(os.Args[1])
	}

	cfg := mysql.NewConfig()
	cfg.User = config.DBUser
	cfg.Passwd = config.DBPass
	cfg.Net = "tcp"
	cfg.Addr = config.DBHost
	cfg.ParseTime = true
	cfg.DBName = config.DBName

	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	fmt.Println("Connected!")

	go func() {
		updateBathingWater()
		bwTicker := time.NewTicker(24 * time.Hour)
		defer bwTicker.Stop()
		for range bwTicker.C {
			updateBathingWater()
		}
	}()

	poll()
	ticker := time.NewTicker(time.Duration(config.PollInterval) * time.Second) // Create a ticker that ticks every 2 seconds
	defer ticker.Stop()                                                        // Ensure the ticker stops when done

	for range ticker.C {
		poll()
	}
}

func poll() {
	start := time.Now()
	log.Println("[POLL] Starting poll cycle")
	var assets []Asset
	twassets, err := fetch("thames-water")
	if err != nil {
		log.Println("thames-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from thames-water\n", len(twassets))
		assets = append(assets, twassets...)
	}
	swassets, err := fetch("southern-water")
	if err != nil {
		log.Println("southern-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from southern-water\n", len(swassets))
		assets = append(assets, swassets...)
	}
	uuassets, err := fetch("united-utilities")
	if err != nil {
		log.Println("united-utilities fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from united-utilities\n", len(uuassets))
		assets = append(assets, uuassets...)
	}
	awassets, err := fetch("anglian-water")
	if err != nil {
		log.Println("anglian-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from anglian-water\n", len(awassets))
		assets = append(assets, awassets...)
	}
	nwassets, err := fetch("northumbrian-water")
	if err != nil {
		log.Println("northumbrian-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from northumbrian-water\n", len(nwassets))
		assets = append(assets, nwassets...)
	}
	stwassets, err := fetch("severn-trent-water")
	if err != nil {
		log.Println("severn-trent-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from severn-trent-water\n", len(stwassets))
		assets = append(assets, stwassets...)
	}
	wwassets, err := fetch("wessex-water")
	if err != nil {
		log.Println("wessex-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from wessex-water\n", len(wwassets))
		assets = append(assets, wwassets...)
	}
	ywassets, err := fetch("yorkshire-water")
	if err != nil {
		log.Println("yorkshire-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from yorkshire-water\n", len(ywassets))
		assets = append(assets, ywassets...)
	}
	swwassets, err := fetch("south-west-water")
	if err != nil {
		log.Println("south-west-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from south-west-water\n", len(swwassets))
		assets = append(assets, swwassets...)
	}
	dwrassets, err := fetch("dwr-cymru")
	if err != nil {
		log.Println("dwr-cymru fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from dwr-cymru\n", len(dwrassets))
		assets = append(assets, dwrassets...)
	}
	scowassets, err := fetchPageScottishWater()
	if err != nil {
		log.Println("scottish-water fetch failed:", err)
	} else {
		log.Printf("[POLL] Fetched %d assets from scottish-water\n", len(scowassets))
		assets = append(assets, scowassets...)
	}
	err = upsertAssets(assets)
	if err != nil {
		log.Println("upsertAssets failed:", err)
	}
	state, err := loadLatestState()
	if err != nil {
		log.Println("failed to load state:", err)
		return
	}
	for _, asset := range assets {
		err = updateState(asset, state)
		if err != nil {
			log.Println(err)
		}
	}
	log.Printf("[POLL] Completed in %s\n", time.Since(start))
}

func fetchConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	var result Config
	err = json.Unmarshal(data, &result)
	if err != nil {
		log.Fatal(err)
	}
	return result
}

func loadLatestState() (map[string]LatestState, error) {
	rows, err := db.Query("SELECT asset_id, latest_event_start, latest_event_end, polled_at FROM latest_state")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	latestState := make(map[string]LatestState)
	for rows.Next() {
		var assetID string
		var latestEventStart *time.Time
		var latestEventEnd *time.Time
		var polledAt time.Time
		err = rows.Scan(&assetID, &latestEventStart, &latestEventEnd, &polledAt)
		if err != nil {
			return nil, err
		}
		latestState[assetID] = LatestState{
			LatestEventStart: latestEventStart,
			LatestEventEnd:   latestEventEnd,
			PolledAt:         polledAt,
			AssetID:          assetID,
		}
	}
	return latestState, nil
}

func updateState(asset Asset, state map[string]LatestState) error {
	assetState := state[asset.AssetID]
	_, assetExists := state[asset.AssetID]
	if !assetExists {
		// First time we've seen this asset, just record state
		_, err := db.Exec(
			"INSERT INTO latest_state (asset_id, status, status_start, latest_event_start, latest_event_end, last_updated, polled_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			asset.AssetID, asset.Status, msToTime(asset.StatusStart), msPtrToTime(asset.LatestEventStart), msPtrToTime(asset.LatestEventEnd), msToTime(asset.LastUpdated), time.Now().UTC(),
		)
		if err != nil {
			return err
		}
	} else {
		apiEventStart := msPtrToTime(asset.LatestEventStart)
		apiEventEnd := msPtrToTime(asset.LatestEventEnd)
		if !timePtrEqual(apiEventStart, assetState.LatestEventStart) {
			// New event detected - update state, don't write to events yet
			_, err := db.Exec(
				"UPDATE latest_state SET status = ?, status_start = ?, latest_event_start = ?, latest_event_end = ?, last_updated = ?, polled_at = ? WHERE asset_id = ?",
				asset.Status, msToTime(asset.StatusStart), msPtrToTime(asset.LatestEventStart), msPtrToTime(asset.LatestEventEnd), msToTime(asset.LastUpdated), time.Now().UTC(), asset.AssetID,
			)
			if err != nil {
				return err
			}

		} else if !timePtrEqual(apiEventEnd, assetState.LatestEventEnd) {
			// Event has now completed - write to events using polled_at as detected_at
			var completedAt *time.Time
			if asset.LatestEventEnd != nil {
				t := time.Now().UTC()
				completedAt = &t
			}

			_, err := db.Exec(
				`INSERT INTO events (asset_id, event_start, event_end, detected_at, completed_at)
    VALUES (?, ?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE
        event_end = VALUES(event_end),
        completed_at = VALUES(completed_at)`,
				asset.AssetID, msPtrToTime(asset.LatestEventStart), msPtrToTime(asset.LatestEventEnd), assetState.PolledAt, completedAt,
			)
			if err != nil {
				return err
			}

			// Update latest_state
			_, err = db.Exec(
				"UPDATE latest_state SET status = ?, status_start = ?, latest_event_start = ?, latest_event_end = ?, last_updated = ?, polled_at = ? WHERE asset_id = ?",
				asset.Status, msToTime(asset.StatusStart), msPtrToTime(asset.LatestEventStart), msPtrToTime(asset.LatestEventEnd), msToTime(asset.LastUpdated), time.Now().UTC(), asset.AssetID,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func timePtrEqual(one *time.Time, two *time.Time) bool {
	if one == nil && two == nil {
		return true
	}
	if one == nil || two == nil {
		return false
	}
	return one.Equal(*two)
}
func msToTime(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.Unix(ms/1000, 0).UTC()
	return &t
}

func msPtrToTime(ms *int64) *time.Time {
	if ms == nil {
		return nil
	}
	t := time.Unix(*ms/1000, 0).UTC()
	return &t
}
func upsertAssets(assets []Asset) error {
	chunkSize := 1000
	for i := 0; i < len(assets); i += chunkSize {
		end := i + chunkSize
		if end > len(assets) {
			end = len(assets)
		}
		chunk := assets[i:end]
		err := upsertAssetChunk(chunk)
		if err != nil {
			return err
		}
	}
	return nil
}
func upsertAssetChunk(assets []Asset) error {
	placeholders := make([]string, len(assets))
	for i := range assets {
		placeholders[i] = "(?, ?, ?, ?, ?, ?)"
	}
	query := "INSERT IGNORE INTO assets (asset_id, company, receiving_watercourse, latitude, longitude, first_seen) VALUES " + strings.Join(placeholders, ",")

	var args []interface{}
	for _, asset := range assets {
		args = append(args, asset.AssetID, asset.Company, asset.ReceivingWaterCourse, asset.Latitude, asset.Longitude, time.Now().UTC())
	}
	_, err := db.Exec(query, args...)
	if err != nil {
		return err
	}
	return nil
}

func fetch(company string) ([]Asset, error) {
	url := ""
	switch company {
	case "thames-water":
		url = "https://services2.arcgis.com/g6o32ZDQ33GpCIu3/arcgis/rest/services/Thames_Water_Storm_Overflow_Activity_(Production)_view/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "southern-water":
		url = "https://services-eu1.arcgis.com/XxS6FebPX29TRGDJ/arcgis/rest/services/Southern_Water_Storm_Overflow_Activity/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "united-utilities":
		url = "https://services5.arcgis.com/5eoLvR0f8HKb7HWP/arcgis/rest/services/United_Utilities_Storm_Overflow_Activity/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "anglian-water":
		url = "https://services3.arcgis.com/VCOY1atHWVcDlvlJ/arcgis/rest/services/stream_service_outfall_locations_view/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "northumbrian-water":
		url = "https://services-eu1.arcgis.com/MSNNjkZ51iVh8yBj/arcgis/rest/services/Northumbrian_Water_Storm_Overflow_Activity_2_view/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "severn-trent-water":
		url = "https://services1.arcgis.com/NO7lTIlnxRMMG9Gw/arcgis/rest/services/Severn_Trent_Water_Storm_Overflow_Activity/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "wessex-water":
		url = "https://services.arcgis.com/3SZ6e0uCvPROr4mS/arcgis/rest/services/Wessex_Water_Storm_Overflow_Activity/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "yorkshire-water":
		url = "https://services-eu1.arcgis.com/1WqkK5cDKUbF0CkH/arcgis/rest/services/Yorkshire_Water_Storm_Overflow_Activity/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "south-west-water": // special cunts with their camel case
		url = "https://services-eu1.arcgis.com/OMdMOtfhATJPcHe3/arcgis/rest/services/NEH_outlets_PROD/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	case "dwr-cymru":
		url = "https://services3.arcgis.com/KLNF7YxtENPLYVey/arcgis/rest/services/Spill_Prod__view/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	}

	if url == "" {
		return nil, errors.New("unknown company: " + company)
	}

	var all []Asset
	offset := 0
	for {
		page, err, overlimit := fetchPage(url, offset, company)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if !overlimit {
			break
		}
		offset += 1000
	}
	return all, nil
}

func fetchPage(url string, offset int, company string) (assets []Asset, err error, overLimit bool) {
	resp, err := http.Get(url + "&resultOffset=" + strconv.Itoa(offset) + "&resultRecordCount=1000")
	if err != nil {
		return nil, err, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status), false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	if company == "south-west-water" {
		var result SWWArcGISResponse
		err = json.Unmarshal(body, &result)
		if err != nil {
			log.Fatalln(err)
		}
		var returnres []Asset

		for _, item := range result.Features {
			returnres = append(returnres, item.Attributes.ToAsset())
		}

		return returnres, nil, result.ExceededLimit
	}
	if company == "dwr-cymru" {
		var result DWRArcGISResponse
		err = json.Unmarshal(body, &result)
		if err != nil {
			log.Fatalln(err)
		}
		var returnres []Asset

		for _, item := range result.Features {
			returnres = append(returnres, item.Attributes.ToAsset(item.Coordinates))
		}

		return returnres, nil, result.ExceededLimit
	}
	var result ArcGISResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatalln(err)
	}
	var returnres []Asset

	for _, item := range result.Features {
		returnres = append(returnres, item.Attributes)
	}

	return returnres, nil, result.ExceededLimit
}

func fetchPageScottishWater() (assets []Asset, err error) {
	resp, err := http.Get("https://api.scottishwater.co.uk/overflow-event-monitoring/v1/near-real-time")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result ScottishWaterResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatalln(err)
	}
	var returnres []Asset

	for _, item := range result.Results {
		returnres = append(returnres, item.ToAsset())
	}

	return returnres, nil
}

func updateBathingWater() {
	start := time.Now()
	log.Println("[POLL] Starting bathing water cycle")
	bathingWaters, err := fetchBathingWaters()
	if err != nil {
		log.Println(err.Error())
	}

	err = upsertBathingWaters(bathingWaters)
	if err != nil {
		log.Println(err.Error())
	}

	err = updateNearestBathingWater()
	if err != nil {
		log.Println(err.Error())
	}
	log.Println("[POLL] Finished bathing water cycle in ", time.Since(start))
}

func fetchBathingWaters() ([]BathingWater, error) {
	resp, err := http.Get("https://environment.data.gov.uk/doc/bathing-water.json?_pageSize=500&_view=bathing-water")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result EABathingWaterResponse
	err = json.Unmarshal(body, &result)

	if err != nil {
		return nil, err
	}

	var returnres []BathingWater

	for _, item := range result.Result.Items {
		bw := item.ToBathingWater()
		if bw.Lat == 0 || bw.Long == 0 {
			continue
		}
		returnres = append(returnres, bw)
	}

	return returnres, nil
}

func upsertBathingWaters(bathingWaters []BathingWater) error {
	chunkSize := 1000
	for i := 0; i < len(bathingWaters); i += chunkSize {
		end := i + chunkSize
		if end > len(bathingWaters) {
			end = len(bathingWaters)
		}
		chunk := bathingWaters[i:end]
		err := upsertBathingWaterChunk(chunk)
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertBathingWaterChunk(bathingWaters []BathingWater) error {
	placeholders := make([]string, len(bathingWaters))
	for i := range bathingWaters {
		placeholders[i] = "(?, ?, ?, ?, ?, ?)"
	}
	query := "INSERT INTO bathing_waters (id, name, latitude, longitude, classification, last_fetched) VALUES " + strings.Join(placeholders, ",") + " ON DUPLICATE KEY UPDATE name = VALUES(name), latitude = VALUES(latitude), longitude = VALUES(longitude), classification = VALUES(classification), last_fetched = VALUES(last_fetched)"
	var args []interface{}

	for _, bathingWater := range bathingWaters {
		args = append(args, bathingWater.ID, bathingWater.Name, bathingWater.Lat, bathingWater.Long, bathingWater.Classification, time.Now().UTC())
	}
	_, err := db.Exec(query, args...)
	if err != nil {
		return err
	}
	return nil
}

func updateNearestBathingWater() error {
	rows, err := db.Query("SELECT asset_id, latitude, longitude FROM assets")
	if err != nil {
		return err
	}
	defer rows.Close()
	assets := make(map[string]MinimalAsset)
	for rows.Next() {
		var assetID string
		var Latitude float64
		var Longitude float64
		err = rows.Scan(&assetID, &Latitude, &Longitude)
		if err != nil {
			return err
		}
		assets[assetID] = MinimalAsset{
			AssetID:   assetID,
			Latitude:  Latitude,
			Longitude: Longitude,
		}
	}
	rows.Close()
	rows, err = db.Query("SELECT id, name, latitude, longitude, classification FROM bathing_waters")
	if err != nil {
		return err
	}
	defer rows.Close()
	bathingWaters := make([]BathingWater, 0)
	for rows.Next() {
		var BathingWaterID string
		var BathingWaterName string
		var Latitude float64
		var Longitude float64
		var Classification string
		err = rows.Scan(&BathingWaterID, &BathingWaterName, &Latitude, &Longitude, &Classification)
		if err != nil {
			return err
		}
		bathingWaters = append(bathingWaters, BathingWater{
			ID:             BathingWaterID,
			Name:           BathingWaterName,
			Lat:            Latitude,
			Long:           Longitude,
			Classification: Classification,
		})
	}

	var results []NearestBathingWaterResult

	for _, asset := range assets {
		var nearest BathingWater
		var nearestDist float64 = -1

		for _, bw := range bathingWaters {
			dist := haversine(asset.Latitude, asset.Longitude, bw.Lat, bw.Long)
			if nearestDist == -1 || dist < nearestDist {
				nearest = bw
				nearestDist = dist
			}
		}

		if nearestDist == -1 || nearestDist > 10000 {
			continue
		}

		results = append(results, NearestBathingWaterResult{
			AssetID:                    asset.AssetID,
			BathingWaterID:             nearest.ID,
			BathingWaterName:           nearest.Name,
			BathingWaterClassification: nearest.Classification,
			Distance:                   int(nearestDist),
		})
	}

	for _, bw := range results {
		_, err := db.Exec("UPDATE assets SET nearest_bw_id = ?, nearest_bw_name = ?, nearest_bw_classification = ?, nearest_bw_distance_m = ? WHERE asset_id = ?", bw.BathingWaterID, bw.BathingWaterName, bw.BathingWaterClassification, bw.Distance, bw.AssetID)

		if err != nil {
			return err
		}
	}

	return nil
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in metres
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func (s SWWAsset) ToAsset() Asset {
	return Asset{
		AssetID:              s.AssetID,
		Company:              s.Company,
		Status:               s.Status,
		StatusStart:          s.StatusStart,
		LatestEventStart:     s.LatestEventStart,
		LatestEventEnd:       s.LatestEventEnd,
		Longitude:            s.Longitude,
		Latitude:             s.Latitude,
		ReceivingWaterCourse: s.ReceivingWaterCourse,
		LastUpdated:          s.LastUpdated,
	}
}

// welsh water workarounds stupid shit

func (s DWRAsset) ToAsset(c DWRCoords) Asset {
	return Asset{
		AssetID:              s.AssetID,
		Company:              "Dwr Cymru Welsh Water",
		Status:               DWRStatusToStatus(s.Status),
		StatusStart:          0,
		LatestEventStart:     parseISOToMillis(s.LatestEventStart),
		LatestEventEnd:       parseISOToMillis(s.LatestEventEnd),
		Longitude:            c.Longitude,
		Latitude:             c.Latitude,
		ReceivingWaterCourse: s.ReceivingWaterCourse,
		LastUpdated:          s.LastUpdated,
	}
}
func DWRStatusToStatus(dwrstatus string) int {
	switch dwrstatus {
	case "Overflow Not Operating":
		return 0
	case "Overflow Operating":
		return 1
	case "Overflow Not Operating (Has in the last 24 hours)":
		return 0
	default:
		println("Unknown status: " + dwrstatus)
		return -1
	}
}

func ScottishWaterStatusToStatus(ScottishWaterStatus string) int {
	switch ScottishWaterStatus {
	case "13":
		return 1
	case "14":
		return 0
	case "15":
		return 0
	case "16":
		return -1
	}
	return -1
}
func parseISOToMillis(s *string) *int64 {
	if s == nil {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, format := range formats {
		t, err := time.Parse(format, *s)
		if err == nil {
			ms := t.UnixMilli()
			return &ms
		}
	}
	log.Println("parseISOToMillis failed:", *s)
	return nil
}
func parseISOToMillisSWPtr(s *string) *int64 {
	if s == nil {
		return nil
	}
	t, err := time.Parse("2006-01-02T15:04:05.000Z", *s)
	if err != nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}
func parseISOToMillisSW(s string) int64 {
	t, err := time.Parse("2006-01-02T15:04:05.000Z", s)
	if err != nil {
		return 0
	}
	ms := t.UnixMilli()
	return ms
}
func parseCoordsSW(s string) float64 {
	data, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Println("Error parsing coordinates: ", err)
		return 0
	}
	return data
}

func (s ScottishWaterAsset) ToAsset() Asset {
	return Asset{
		AssetID:              s.AssetID,
		Company:              "Scottish Water",
		Status:               ScottishWaterStatusToStatus(s.Status),
		StatusStart:          0,
		LatestEventStart:     parseISOToMillisSWPtr(s.LatestEventStart),
		LatestEventEnd:       parseISOToMillisSWPtr(s.LatestEventEnd),
		Longitude:            parseCoordsSW(s.Longitude),
		Latitude:             parseCoordsSW(s.Latitude),
		ReceivingWaterCourse: s.ReceivingWaterCourse,
		LastUpdated:          parseISOToMillisSW(s.LastUpdated),
	}
}

func (item EABathingWaterItem) ToBathingWater() BathingWater {
	return BathingWater{
		ID:             item.ID,
		Name:           item.Name.Value,
		Lat:            item.SamplingPoint.Lat,
		Long:           item.SamplingPoint.Long,
		Classification: item.LatestComplianceAssessment.ComplianceClassification.Name.Value,
	}
}

type ScottishWaterResponse struct {
	Results []ScottishWaterAsset `json:"results"`
}
type ScottishWaterAsset struct {
	AssetID              string  `json:"ASSET_ID"`
	Status               string  `json:"OVERFLOW_STATUS_ID"`
	LatestEventStart     *string `json:"OVERFLOW_START_DATETIME"`
	LatestEventEnd       *string `json:"OVERFLOW_END_DATETIME"`
	Longitude            string  `json:"DISCHARGE_OVERFLOW_LOCATION_LONGITUDE"`
	Latitude             string  `json:"DISCHARGE_OVERFLOW_LOCATION_LATITUDE"`
	ReceivingWaterCourse string  `json:"RECEIVING_WATER"`
	LastUpdated          string  `json:"LAST_TRANSMITTED_DATETIME"`
}

type ArcGISResponse struct {
	Features      []Features `json:"features"`
	ExceededLimit bool       `json:"exceededTransferLimit"`
}

type Features struct {
	Attributes Asset `json:"attributes"`
}
type Asset struct {
	AssetID              string  `json:"Id"`
	Company              string  `json:"Company"`
	Status               int     `json:"Status"`
	StatusStart          int64   `json:"StatusStart"`
	LatestEventStart     *int64  `json:"LatestEventStart"`
	LatestEventEnd       *int64  `json:"LatestEventEnd"`
	Longitude            float64 `json:"Longitude"`
	Latitude             float64 `json:"Latitude"`
	ReceivingWaterCourse string  `json:"ReceivingWaterCourse"`
	LastUpdated          int64   `json:"LastUpdated"`
}
type MinimalAsset struct {
	AssetID   string
	Longitude float64
	Latitude  float64
}

type SWWArcGISResponse struct {
	Features      []SWWFeatures `json:"features"`
	ExceededLimit bool          `json:"exceededTransferLimit"`
}

type SWWFeatures struct {
	Attributes SWWAsset `json:"attributes"`
}
type SWWAsset struct {
	AssetID              string  `json:"Id"`
	Company              string  `json:"company"`
	Status               int     `json:"status"`
	StatusStart          int64   `json:"statusStart"`
	LatestEventStart     *int64  `json:"latestEventStart"`
	LatestEventEnd       *int64  `json:"latestEventEnd"`
	Longitude            float64 `json:"longitude"`
	Latitude             float64 `json:"latitude"`
	ReceivingWaterCourse string  `json:"receivingWaterCourse"`
	LastUpdated          int64   `json:"lastUpdated"`
}
type DWRArcGISResponse struct {
	Features      []DWRFeatures `json:"features"`
	ExceededLimit bool          `json:"exceededTransferLimit"`
}

type DWRFeatures struct {
	Attributes  DWRAsset  `json:"attributes"`
	Coordinates DWRCoords `json:"geometry"`
}
type DWRAsset struct { // absolute wankers
	AssetID              string  `json:"GlobalID"`
	Status               string  `json:"status"`
	LatestEventStart     *string `json:"start_date_time_discharge"`
	LatestEventEnd       *string `json:"stop_date_time_discharge"`
	ReceivingWaterCourse string  `json:"Receiving_Water"`
	LastUpdated          int64   `json:"EditDate"`
}
type DWRCoords struct {
	Longitude float64 `json:"x"`
	Latitude  float64 `json:"y"`
}

type LatestState struct {
	AssetID          string
	Status           int
	LatestEventStart *time.Time
	LatestEventEnd   *time.Time
	PolledAt         time.Time
}
type EABathingWaterResponse struct {
	Result struct {
		Items []EABathingWaterItem `json:"items"`
	} `json:"result"`
}

type EABathingWaterItem struct {
	ID   string `json:"eubwidNotation"`
	Name struct {
		Value string `json:"_value"`
	} `json:"name"`
	SamplingPoint struct {
		Lat  float64 `json:"lat"`
		Long float64 `json:"long"`
	} `json:"samplingPoint"`
	LatestComplianceAssessment struct {
		ComplianceClassification struct {
			Name struct {
				Value string `json:"_value"`
			} `json:"name"`
		} `json:"complianceClassification"`
	} `json:"latestComplianceAssessment"`
}

type BathingWater struct {
	ID             string `json:"eubwidNotation"`
	Name           string `json:"name"`
	Lat            float64
	Long           float64
	Classification string
}

type NearestBathingWaterResult struct {
	AssetID                    string
	BathingWaterID             string
	BathingWaterName           string
	BathingWaterClassification string
	Distance                   int
}

type Config struct {
	DBHost                     string `json:"db_host"`
	DBUser                     string `json:"db_user"`
	DBPass                     string `json:"db_pass"`
	DBName                     string `json:"db_name"`
	PollInterval               int    `json:"poll_interval"`
	BathingWaterUpdateInterval int    `json:"bathing_water_update_interval"`
}
