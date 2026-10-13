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
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	// _ "github.com/go-sql-driver/mysql"
	"io"
	"log"
	"net/http"
)

var db *sql.DB

// httpClient is shared by every fetch. The timeout stops one stalled upstream
// from hanging the poll loop forever.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// loggedOnce stops data problems that repeat on every poll from flooding the log.
var loggedOnce sync.Map

func logOnce(key string, format string, args ...any) {
	if _, seen := loggedOnce.LoadOrStore(key, true); !seen {
		log.Printf(format, args...)
	}
}

func main() {
	var config Config
	dev := os.Getenv("FETCHER_IS_DEV")
	if dev != "" && dev != "0" && !strings.EqualFold(dev, "false") {
		config = fetchConfig("./config.json")
	} else {
		if len(os.Args) < 2 {
			log.Fatal("usage: sewagedata-poller <config-path>")
		}
		config = fetchConfig(os.Args[1])
	}
	if config.PollInterval <= 0 {
		log.Fatal("config: poll_interval must be a positive number of seconds")
	}
	bathingWaterInterval := 24 * time.Hour
	if config.BathingWaterUpdateInterval > 0 {
		bathingWaterInterval = time.Duration(config.BathingWaterUpdateInterval) * time.Minute
	}

	cfg := mysql.NewConfig()
	cfg.User = config.DBUser
	cfg.Passwd = config.DBPass
	cfg.Net = "tcp"
	cfg.Addr = config.DBHost
	cfg.ParseTime = true
	cfg.DBName = config.DBName
	// Optional: "true", "skip-verify" or "preferred". Unset means no TLS.
	cfg.TLSConfig = config.DBTLS

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
		bwTicker := time.NewTicker(bathingWaterInterval)
		defer bwTicker.Stop()
		for range bwTicker.C {
			updateBathingWater()
		}
	}()

	poll()
	ticker := time.NewTicker(time.Duration(config.PollInterval) * time.Second) // Create a ticker that ticks every poll_interval seconds
	defer ticker.Stop()                                                        // Ensure the ticker stops when done

	for range ticker.C {
		poll()
	}
}

// arcgisCompanies are the feeds fetch() knows; Scottish Water is fetched separately.
var arcgisCompanies = []string{
	"thames-water", "southern-water", "united-utilities", "anglian-water", "northumbrian-water",
	"severn-trent-water", "wessex-water", "yorkshire-water", "south-west-water", "dwr-cymru",
}

func poll() {
	start := time.Now()
	log.Println("[POLL] Starting poll cycle")
	var assets []Asset
	// Whether every feed came back with assets this poll.
	allFetched := true
	for _, company := range arcgisCompanies {
		companyAssets, err := fetch(company)
		if err != nil {
			log.Println(company+" fetch failed:", err)
			allFetched = false
			continue
		}
		log.Printf("[POLL] Fetched %d assets from %s\n", len(companyAssets), company)
		allFetched = allFetched && len(companyAssets) > 0
		assets = append(assets, companyAssets...)
	}
	scowassets, err := fetchPageScottishWater()
	if err != nil {
		log.Println("scottish-water fetch failed:", err)
		allFetched = false
	} else {
		log.Printf("[POLL] Fetched %d assets from scottish-water\n", len(scowassets))
		allFetched = allFetched && len(scowassets) > 0
		assets = append(assets, scowassets...)
	}
	assets = dedupeAssets(assets)
	err = upsertAssets(assets)
	if err != nil {
		log.Println("upsertAssets failed:", err)
	}
	state, err := loadLatestState()
	if err != nil {
		log.Println("failed to load state:", err)
		lastPoll = map[string]observation{}
		return
	}
	seen := make(map[string]observation, len(assets))
	for _, asset := range assets {
		err = updateState(asset, state, lastPoll)
		if err != nil {
			log.Println(err)
		}
		seen[asset.AssetID] = observation{Status: asset.Status, EventStart: msPtrToTime(asset.LatestEventStart)}
	}
	lastPoll = seen
	err = flagRemovedAssets(assets, allFetched, time.Now().UTC())
	if err != nil {
		log.Println("flagRemovedAssets failed:", err)
	}
	log.Printf("[POLL] Completed in %s\n", time.Since(start))
}

// removedAfter is how long an asset has to be missing from its company's feed
// before it's flagged as removed.
const removedAfter = 24 * time.Hour

// missingSince is when each asset was first found missing from its company's
// feed. It's in memory, so a restart just restarts the clock.
var missingSince = map[string]time.Time{}

// flagRemovedAssets sets assets.asset_removed for assets that have dropped out
// of their company's feed, so the site can stop showing them. upsertAssets
// clears the flag if one comes back, so a feed that's wrong for a day only
// hides assets until it recovers.
func flagRemovedAssets(assets []Asset, allFetched bool, now time.Time) error {
	rows, err := db.Query("SELECT asset_id, COALESCE(company, '') FROM assets WHERE asset_removed IS NOT TRUE")
	if err != nil {
		return err
	}
	defer rows.Close()
	known := make(map[string][]string)
	for rows.Next() {
		var assetID string
		var company string
		err = rows.Scan(&assetID, &company)
		if err != nil {
			return err
		}
		known[company] = append(known[company], assetID)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	removed := findRemovedAssets(assets, known, allFetched, now)
	if len(removed) == 0 {
		return nil
	}
	log.Printf("[REMOVED] Flagging %d assets that have left their feeds, e.g. %v\n", len(removed), removed[:min(len(removed), 20)])
	err = execForAssets("UPDATE assets SET asset_removed = TRUE WHERE asset_id IN ", removed)
	if err != nil {
		return err
	}
	for _, assetID := range removed {
		delete(missingSince, assetID)
	}
	return nil
}

// findRemovedAssets compares this poll's assets with the ones we know of
// (company -> asset IDs not yet flagged) and returns those that have been
// missing from their company's feed for at least removedAfter. A company is
// only judged when its fetch came back with assets: a failed fetch never
// removes anything. Rows under a company name no feed returns any more (the
// feed renamed itself) are judged only when every feed came back.
func findRemovedAssets(assets []Asset, known map[string][]string, allFetched bool, now time.Time) []string {
	fetched := make(map[string]bool)
	present := make(map[string]bool, len(assets))
	for _, asset := range assets {
		fetched[asset.Company] = true
		present[asset.AssetID] = true
	}

	var removed []string
	for company, assetIDs := range known {
		if !fetched[company] && !allFetched {
			continue
		}
		for _, assetID := range assetIDs {
			if present[assetID] {
				delete(missingSince, assetID)
				continue
			}
			since, ok := missingSince[assetID]
			if !ok {
				missingSince[assetID] = now
				continue
			}
			if now.Sub(since) >= removedAfter {
				removed = append(removed, assetID)
			}
		}
	}
	return removed
}

// lastPoll is what the previous poll saw in each feed. It's empty after a
// restart and leaves out assets whose company failed to fetch, so it tells a
// status change we watched happen apart from a stored status that's just stale.
var lastPoll = map[string]observation{}

type observation struct {
	Status     int
	EventStart *time.Time
}

// dedupeAssets drops repeated asset IDs, keeping the first. Some feeds list the
// same asset twice, and both copies would otherwise be compared against the
// same stored state and overwrite each other.
func dedupeAssets(assets []Asset) []Asset {
	seen := make(map[string]bool, len(assets))
	deduped := assets[:0]
	dupes := 0
	for _, asset := range assets {
		if seen[asset.AssetID] {
			dupes++
			continue
		}
		seen[asset.AssetID] = true
		deduped = append(deduped, asset)
	}
	if dupes > 0 {
		log.Printf("[POLL] Skipped %d duplicate asset IDs\n", dupes)
	}
	return deduped
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
	rows, err := db.Query("SELECT asset_id, status, status_start, latest_event_start, latest_event_end, polled_at FROM latest_state")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	latestState := make(map[string]LatestState)
	for rows.Next() {
		var assetID string
		var status *int
		var statusStart *time.Time
		var latestEventStart *time.Time
		var latestEventEnd *time.Time
		var polledAt time.Time
		err = rows.Scan(&assetID, &status, &statusStart, &latestEventStart, &latestEventEnd, &polledAt)
		if err != nil {
			return nil, err
		}
		latestState[assetID] = LatestState{
			Status:           status,
			StatusStart:      statusStart,
			LatestEventStart: latestEventStart,
			LatestEventEnd:   latestEventEnd,
			PolledAt:         polledAt,
			AssetID:          assetID,
		}
	}
	// A connection dropped mid-read ends the loop early; a partial map would
	// make every missing asset look brand new.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return latestState, nil
}

// eventRow is a finished event to upsert into events. Rows are only written
// once they have both a start and an end: the API can't read events with a
// NULL end.
type eventRow struct {
	Start       time.Time
	End         time.Time
	DetectedAt  time.Time
	CompletedAt time.Time
}

// stateChange is what one poll needs to write for one asset.
type stateChange struct {
	InsertState  bool       // first time we've seen the asset
	UpdateState  bool       // the event changed: rewrite latest_state, including polled_at
	UpdateStatus bool       // only the status changed: polled_at keeps its meaning
	Events       []eventRow // finished events to record
	Lost         *time.Time // start of an event replaced before the feed reported its end
	ResumedAt    *time.Time // a reopened event: write this as status_start instead of the feed's
}

// planStateChange compares what a feed says about an asset now with what we
// stored last time and works out what to write. It doesn't touch the database.
func planStateChange(asset Asset, prev LatestState, exists bool, now time.Time) stateChange {
	if !exists {
		// First time we've seen this asset, just record state
		return stateChange{InsertState: true}
	}

	var change stateChange
	start := msPtrToTime(asset.LatestEventStart)
	end := msPtrToTime(asset.LatestEventEnd)
	statusStart := msToTime(asset.StatusStart)

	switch {
	case !timePtrEqual(start, prev.LatestEventStart):
		change.UpdateState = true
		if start != nil && ((prev.LatestEventStart != nil && start.Before(*prev.LatestEventStart)) ||
			(prev.LatestEventEnd != nil && start.Before(*prev.LatestEventEnd))) {
			// Events don't overlap, so a start before the event we're tracking
			// (or before its end) is the feed correcting that event, not a new one.
			// If it was still open it has no events row yet, so a corrected
			// version that has already finished has to be recorded now.
			if prev.LatestEventEnd == nil && isFinished(start, end) {
				change.Events = append(change.Events, eventRow{*start, *end, prev.PolledAt, now})
			}
			break
		}

		// New event. Before moving on, make sure the one it replaced is recorded.
		prevOpen := prev.LatestEventStart != nil && prev.LatestEventEnd == nil
		recovered := false

		// Scottish Water also publishes the event before the current one.
		// That gives the end of an event we saw start, or a whole event that
		// happened between polls.
		previousStart := msPtrToTime(asset.PreviousEventStart)
		previousEnd := msPtrToTime(asset.PreviousEventEnd)
		if isFinished(previousStart, previousEnd) && !timePtrEqual(previousStart, start) {
			sameAsTracked := timePtrEqual(previousStart, prev.LatestEventStart)
			switch {
			case sameAsTracked && prevOpen:
				change.Events = append(change.Events, eventRow{*previousStart, *previousEnd, prev.PolledAt, now})
				recovered = true
			case !sameAsTracked:
				change.Events = append(change.Events, eventRow{*previousStart, *previousEnd, now, now})
			}
		}
		if prevOpen && !recovered {
			change.Lost = prev.LatestEventStart
		}

		// A new event that has already finished won't change again, so this is
		// the only chance to record it.
		if isFinished(start, end) {
			change.Events = append(change.Events, eventRow{*start, *end, now, now})
		}

	case !timePtrEqual(end, prev.LatestEventEnd):
		// The event has finished, or its end was revised: record it using
		// polled_at (when we saw it start) as detected_at. If the end has gone
		// back to NULL the event has reopened; leave its events row alone
		// rather than wiping the end we recorded.
		change.UpdateState = true
		if isFinished(start, end) {
			change.Events = append(change.Events, eventRow{*start, *end, prev.PolledAt, now})
		}
		if end == nil && prev.LatestEventEnd != nil && asset.Status == 1 {
			// Discharging again. The part up to the recorded end is already in
			// events, so count the ongoing spill from there; starting it at the
			// event's start would count that part twice.
			resumed := *prev.LatestEventEnd
			if statusStart != nil && statusStart.After(resumed) {
				resumed = *statusStart
			}
			change.ResumedAt = &resumed
		}

	case prev.Status == nil || *prev.Status != asset.Status ||
		(prev.StatusStart == nil && statusStart != nil):
		// Status changed without a new event, e.g. a monitor going offline.
		change.UpdateStatus = true

		// United Utilities often stops a discharge without ever filling in
		// LatestEventEnd. When the previous poll saw it discharging and this one
		// doesn't, the time the status changed is when the discharge stopped. A
		// real end arriving later overwrites this through the branch above.
		// Without that observation (after a restart, a failed fetch, or a stored
		// status the old fetcher never updated) or after an offline spell
		// (-1 -> 0), the status change may only say when the monitor came back,
		// so nothing is recorded.
		if prev.Status != nil && *prev.Status == 1 && asset.Status == 0 &&
			prev.SeenLastPoll != nil && prev.SeenLastPoll.Status == 1 &&
			timePtrEqual(prev.SeenLastPoll.EventStart, start) &&
			end == nil && isFinished(start, statusStart) {
			change.Events = append(change.Events, eventRow{*start, *statusStart, prev.PolledAt, now})
		}
	}
	return change
}

func isFinished(start *time.Time, end *time.Time) bool {
	return start != nil && end != nil && !end.Before(*start)
}

func updateState(asset Asset, state map[string]LatestState, lastPoll map[string]observation) error {
	prev, exists := state[asset.AssetID]
	if seen, ok := lastPoll[asset.AssetID]; ok {
		prev.SeenLastPoll = &seen
	}
	now := time.Now().UTC()
	change := planStateChange(asset, prev, exists, now)

	if change.Lost != nil {
		log.Printf("[STATE] %s: event starting %s was replaced before the feed reported its end\n",
			asset.AssetID, change.Lost.Format(time.RFC3339))
	}

	// Write events before latest_state: if anything fails, the next poll sees
	// the same change again and the upsert makes the retry harmless.
	for _, event := range change.Events {
		_, err := db.Exec(
			`INSERT INTO events (asset_id, event_start, event_end, detected_at, completed_at)
    VALUES (?, ?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE
        event_end = VALUES(event_end),
        completed_at = VALUES(completed_at)`,
			asset.AssetID, event.Start, event.End, event.DetectedAt, event.CompletedAt,
		)
		if err != nil {
			return err
		}
	}

	switch {
	case change.InsertState:
		_, err := db.Exec(
			"INSERT INTO latest_state (asset_id, status, status_start, latest_event_start, latest_event_end, last_updated, polled_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			asset.AssetID, asset.Status, msToTime(asset.StatusStart), msPtrToTime(asset.LatestEventStart), msPtrToTime(asset.LatestEventEnd), msToTime(asset.LastUpdated), now,
		)
		return err
	case change.UpdateState:
		statusStart := msToTime(asset.StatusStart)
		if change.ResumedAt != nil {
			statusStart = change.ResumedAt
		}
		_, err := db.Exec(
			"UPDATE latest_state SET status = ?, status_start = ?, latest_event_start = ?, latest_event_end = ?, last_updated = ?, polled_at = ? WHERE asset_id = ?",
			asset.Status, statusStart, msPtrToTime(asset.LatestEventStart), msPtrToTime(asset.LatestEventEnd), msToTime(asset.LastUpdated), now, asset.AssetID,
		)
		return err
	case change.UpdateStatus:
		_, err := db.Exec(
			"UPDATE latest_state SET status = ?, status_start = ?, last_updated = ? WHERE asset_id = ?",
			asset.Status, msToTime(asset.StatusStart), msToTime(asset.LastUpdated), asset.AssetID,
		)
		return err
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
	// Keep details current when a company moves or renames an asset, but don't
	// let a blank value in one poll wipe out a good one. Anything in a feed
	// isn't removed, so clear the flag if it had been set.
	query := "INSERT INTO assets (asset_id, company, receiving_watercourse, latitude, longitude, first_seen) VALUES " + strings.Join(placeholders, ",") +
		" ON DUPLICATE KEY UPDATE" +
		" company = COALESCE(NULLIF(VALUES(company), ''), company)," +
		" receiving_watercourse = COALESCE(NULLIF(VALUES(receiving_watercourse), ''), receiving_watercourse)," +
		" latitude = IF(VALUES(latitude) = 0 OR VALUES(longitude) = 0, latitude, VALUES(latitude))," +
		" longitude = IF(VALUES(latitude) = 0 OR VALUES(longitude) = 0, longitude, VALUES(longitude))," +
		" asset_removed = FALSE"

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
		url = "https://services-eu1.arcgis.com/6qJmARkS2dt2IjVA/arcgis/rest/services/SouthernWater_StormOverflowActivity_PROD_view/FeatureServer/0/query?outFields=*&where=1%3D1&f=json"
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
		url = "https://services3.arcgis.com/KLNF7YxtENPLYVey/arcgis/rest/services/Spill_Prod/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"
	}

	if url == "" {
		return nil, errors.New("unknown company: " + company)
	}

	return fetchAllPages(url, company)
}

func fetchAllPages(url string, company string) ([]Asset, error) {
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
		if len(page) == 0 {
			return nil, errors.New("exceededTransferLimit set on an empty page")
		}
		// Advance by what the server actually returned: it can cap pages below
		// the 1000 we ask for.
		offset += len(page)
	}
	return all, nil
}

func fetchPage(url string, offset int, company string) (assets []Asset, err error, overLimit bool) {
	resp, err := httpClient.Get(url + "&resultOffset=" + strconv.Itoa(offset) + "&resultRecordCount=1000")
	if err != nil {
		return nil, err, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status), false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return nil, err, false
	}
	// ArcGIS reports most failures as HTTP 200 with an "error" object and no
	// features, which would otherwise look like an empty (or final) page.
	var errResp struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && len(errResp.Error) > 0 && string(errResp.Error) != "null" {
		return nil, fmt.Errorf("arcgis error: %s", errResp.Error), false
	}
	if company == "south-west-water" {
		var result SWWArcGISResponse
		err = json.Unmarshal(body, &result)
		if err != nil {
			log.Println(err)
			return nil, err, false
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
			log.Println(err)
			return nil, err, false
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
		log.Println(err)
		return nil, err, false
	}
	var returnres []Asset

	for _, item := range result.Features {
		returnres = append(returnres, item.Attributes)
	}

	return returnres, nil, result.ExceededLimit
}

func fetchPageScottishWater() (assets []Asset, err error) {
	resp, err := httpClient.Get("https://api.scottishwater.co.uk/overflow-event-monitoring/v1/near-real-time")
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
		log.Println(err)
		return nil, err
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

var bathingWaterURL = "https://environment.data.gov.uk/doc/bathing-water.json?_pageSize=500&_view=bathing-water"

// maxBathingWaterPages guards against a misbehaving API linking pages forever.
const maxBathingWaterPages = 50

func fetchBathingWaters() ([]BathingWater, error) {
	var returnres []BathingWater

	// The EA API pages its results; keep going while it links a next page.
	for page := 0; ; page++ {
		if page == maxBathingWaterPages {
			return nil, fmt.Errorf("bathing waters: gave up after %d pages", page)
		}
		result, err := fetchBathingWaterPage(page)
		if err != nil {
			return nil, err
		}

		for _, item := range result.Result.Items {
			bw := item.ToBathingWater()
			if bw.Lat == 0 || bw.Long == 0 {
				continue
			}
			returnres = append(returnres, bw)
		}

		if result.Result.Next == "" || len(result.Result.Items) == 0 {
			break
		}
	}

	return returnres, nil
}

func fetchBathingWaterPage(page int) (*EABathingWaterResponse, error) {
	resp, err := httpClient.Get(bathingWaterURL + "&_page=" + strconv.Itoa(page))
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
	return &result, nil
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
	if err = rows.Err(); err != nil {
		return err
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
		if Classification == "Closed" {
			// Not a bathing water any more; match the next nearest open one.
			continue
		}
		bathingWaters = append(bathingWaters, BathingWater{
			ID:             BathingWaterID,
			Name:           BathingWaterName,
			Lat:            Latitude,
			Long:           Longitude,
			Classification: Classification,
		})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(bathingWaters) == 0 {
		// Nothing to compare against; don't clear every asset's nearest bathing water.
		return nil
	}

	var results []NearestBathingWaterResult
	var outOfRange []string

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
			outOfRange = append(outOfRange, asset.AssetID)
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

	return clearNearestBathingWater(outOfRange)
}

// clearNearestBathingWater removes stale matches from assets that no longer
// have a bathing water within range (it moved, or the asset did).
func clearNearestBathingWater(assetIDs []string) error {
	return execForAssets("UPDATE assets SET nearest_bw_id = NULL, nearest_bw_name = NULL, nearest_bw_classification = NULL, nearest_bw_distance_m = NULL WHERE nearest_bw_id IS NOT NULL AND asset_id IN ", assetIDs)
}

// execForAssets runs query, which must end in "asset_id IN ", for assetIDs in
// chunks of 1000.
func execForAssets(query string, assetIDs []string) error {
	chunkSize := 1000
	for i := 0; i < len(assetIDs); i += chunkSize {
		chunk := assetIDs[i:min(i+chunkSize, len(assetIDs))]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		args := make([]interface{}, len(chunk))
		for j, id := range chunk {
			args[j] = id
		}
		_, err := db.Exec(query+"("+placeholders+")", args...)
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

// Workarounds because DWR Cymru uses a different format (annoying!)

func (s DWRAsset) ToAsset(c DWRCoords) Asset {
	status := DWRStatusToStatus(s.Status)
	start := parseISOToMillis(s.LatestEventStart)
	end := parseISOToMillis(s.LatestEventEnd)
	return Asset{
		AssetID:              s.AssetID,
		Company:              "Dwr Cymru Welsh Water",
		Status:               status,
		StatusStart:          statusStartFromEvent(status, start, end),
		LatestEventStart:     start,
		LatestEventEnd:       end,
		Longitude:            c.Longitude,
		Latitude:             c.Latitude,
		ReceivingWaterCourse: s.ReceivingWaterCourse,
		LastUpdated:          s.LastUpdated,
	}
}

// statusStartFromEvent works out when the current status began for feeds
// that don't publish it: a discharge began when the event started, and
// "not discharging" began when it ended. The API needs this to count spills
// that are still going.
func statusStartFromEvent(status int, start *int64, end *int64) int64 {
	switch {
	case status == 1 && start != nil && end == nil:
		return *start
	case status == 0 && end != nil:
		return *end
	}
	return 0
}
func DWRStatusToStatus(dwrstatus string) int {
	switch dwrstatus {
	case "Overflow Not Operating":
		return 0
	case "Overflow Operating":
		return 1
	case "Overflow Not Operating (Has in the last 24 hours)":
		return 0
	case "Under Investigation":
		return -1
	default:
		logOnce("dwr-status:"+dwrstatus, "Unknown DWR status: %q", dwrstatus)
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
	logOnce("dwr-time:"+*s, "parseISOToMillis failed: %s", *s)
	return nil
}

// parseSWTime parses a Scottish Water timestamp such as
// "2026-10-13T16:45:00.000Z". They send "" when there's no value.
func parseSWTime(s string) *int64 {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		logOnce("sw-time:"+s, "Unparseable Scottish Water timestamp: %q", s)
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}
func parseISOToMillisSWPtr(s *string) *int64 {
	if s == nil {
		return nil
	}
	return parseSWTime(*s)
}
func parseISOToMillisSW(s string) int64 {
	if ms := parseSWTime(s); ms != nil {
		return *ms
	}
	return 0
}
func parseCoordsSW(s string) float64 {
	data, err := strconv.ParseFloat(s, 64)
	if err != nil {
		logOnce("sw-coords:"+s, "Error parsing coordinates: %v", err)
		return 0
	}
	return data
}

func (s ScottishWaterAsset) ToAsset() Asset {
	status := ScottishWaterStatusToStatus(s.Status)
	start := parseISOToMillisSWPtr(s.LatestEventStart)
	end := parseISOToMillisSWPtr(s.LatestEventEnd)
	return Asset{
		AssetID:              s.AssetID,
		Company:              "Scottish Water",
		Status:               status,
		StatusStart:          statusStartFromEvent(status, start, end),
		LatestEventStart:     start,
		LatestEventEnd:       end,
		Longitude:            parseCoordsSW(s.Longitude),
		Latitude:             parseCoordsSW(s.Latitude),
		ReceivingWaterCourse: s.ReceivingWaterCourse,
		LastUpdated:          parseISOToMillisSW(s.LastUpdated),
		PreviousEventStart:   parseISOToMillisSWPtr(s.PreviousEventStart),
		PreviousEventEnd:     parseISOToMillisSWPtr(s.PreviousEventEnd),
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
	PreviousEventStart   *string `json:"OVERFLOW_START_DATETIME_PREVIOUS"`
	PreviousEventEnd     *string `json:"OVERFLOW_END_DATETIME_PREVIOUS"`
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
	// The event before the latest one. Only Scottish Water publishes this.
	PreviousEventStart *int64 `json:"-"`
	PreviousEventEnd   *int64 `json:"-"`
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
type DWRAsset struct {
	AssetID              string  `json:"DCWW_ID"`
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
	Status           *int
	StatusStart      *time.Time
	LatestEventStart *time.Time
	LatestEventEnd   *time.Time
	PolledAt         time.Time
	// What the previous poll in this process saw; nil after a restart or a failed fetch.
	SeenLastPoll *observation
}
type EABathingWaterResponse struct {
	Result struct {
		Items []EABathingWaterItem `json:"items"`
		Next  string               `json:"next"`
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
	PollInterval               int    `json:"poll_interval"`                 // seconds
	BathingWaterUpdateInterval int    `json:"bathing_water_update_interval"` // minutes; defaults to 1440
	DBTLS                      string `json:"db_tls"`                        // optional mysql tls mode
}
