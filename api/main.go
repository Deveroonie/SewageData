package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)
import "github.com/gin-contrib/gzip"

var db *sql.DB

func main() {
	dev := os.Getenv("API_IS_DEV")

	var config Config

	if dev != "" {
		config = fetchConfig("./config.json")
	} else {
		if len(os.Args) < 2 {
			log.Fatal("usage: cso-api <config-path>")
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

	if err := sentry.Init(sentry.ClientOptions{
		Dsn: config.DSN,
	}); err != nil {
		fmt.Printf("Sentry initialization failed: %v\n", err)
	}

	r := gin.Default()
	r.Use(sentrygin.New(sentrygin.Options{}))
	r.Use(cors.Default())
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	r.GET("/api/assets", getAssets)
	r.GET("/api/asset/:id", getAsset)
	r.GET("/api/asset/:id/events", getAssetEvents)
	r.GET("/api/stats", getStats)
	r.GET("/api/top-discharges", getTopDischarges)
	r.Run(":8080")
}

func getAssets(c *gin.Context) {
	rows, err := db.Query(`
    SELECT a.asset_id,
        a.latitude,
        a.longitude,
        l.status,
        l.latest_event_end
    FROM assets a
    INNER JOIN latest_state l ON l.asset_id = a.asset_id
`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var response AssetResponse
	for rows.Next() {
		var assetId string
		var latitude float64
		var longitude float64
		var status int
		var latestEventEnd *time.Time

		err := rows.Scan(&assetId, &latitude, &longitude, &status, &latestEventEnd)
		if err != nil {
			return
		}

		asset := MinimalAsset{
			assetId,
			latitude,
			longitude,
			status,
			latestEventEnd,
		}

		response.Assets = append(response.Assets, asset)
	}

	c.JSON(200, response)
}
func getAsset(c *gin.Context) {
	assetId := c.Param("id")

	rows, err := db.Query(`
    SELECT a.asset_id,
        a.company,
        a.receiving_watercourse,
        a.latitude,
        a.longitude,
        l.status,
        l.latest_event_start,
        l.latest_event_end,
        l.polled_at,
        a.nearest_bw_id,
        a.nearest_bw_name,
        a.nearest_bw_classification,
        a.nearest_bw_distance_m
    FROM assets a
    INNER JOIN latest_state l ON l.asset_id = a.asset_id
    WHERE a.asset_id = ?
`, assetId)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var response Asset
	for rows.Next() {
		var assetId string
		var company string
		var receivingWaterCourse string
		var latitude float64
		var longitude float64
		var status int
		var latestEventStart *time.Time
		var latestEventEnd *time.Time
		var polledAt time.Time
		var nearestBathingWaterID *string
		var nearestBathingWaterName *string
		var nearestBathingWaterClassification *string
		var nearestBathingDistance *int

		err := rows.Scan(&assetId, &company, &receivingWaterCourse, &latitude, &longitude, &status, &latestEventStart, &latestEventEnd, &polledAt, &nearestBathingWaterID, &nearestBathingWaterName, &nearestBathingWaterClassification, &nearestBathingDistance)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		response = Asset{
			assetId,
			company,
			receivingWaterCourse,
			latitude,
			longitude,
			status,
			latestEventStart,
			latestEventEnd,
			polledAt,
			nearestBathingWaterName,
			nearestBathingWaterID,
			nearestBathingWaterClassification,
			nearestBathingDistance,
		}

		c.JSON(200, response)
	}
}
func getAssetEvents(c *gin.Context) {
	assetId := c.Param("id")
	rows, err := db.Query(`SELECT 
    asset_id,
    event_start,
    event_end,
    TIMESTAMPDIFF(MINUTE, event_start, event_end) as duration_minutes
FROM events
WHERE asset_id = ?
ORDER BY event_start DESC`, assetId)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var response EventsResponse
	for rows.Next() {
		var assetId string
		var eventStart time.Time
		var eventEnd time.Time
		var durationMinutes int

		err := rows.Scan(&assetId, &eventStart, &eventEnd, &durationMinutes)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		event := Event{
			assetId,
			eventStart,
			eventEnd,
			durationMinutes,
		}
		response.Events = append(response.Events, event)
	}
	c.JSON(200, response)
}
func getStats(c *gin.Context) {
	var stats Stats

	// Total assets
	err := db.QueryRow("SELECT COUNT(*) FROM assets").Scan(&stats.TotalAssets)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Total discharging
	err = db.QueryRow("SELECT COUNT(*) FROM latest_state WHERE status = 1").Scan(&stats.TotalDischarging)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Total offline
	err = db.QueryRow("SELECT COUNT(*) FROM latest_state WHERE status = -1").Scan(&stats.TotalOffline)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rows, err := db.Query(`SELECT 
    a.company,
    COUNT(*) as total_assets,
    SUM(CASE WHEN l.status = 1 THEN 1 ELSE 0 END) as total_discharging,
    SUM(CASE WHEN l.status = -1 THEN 1 ELSE 0 END) as total_offline,
    ROUND(SUM(CASE WHEN l.status = 1 THEN 1 ELSE 0 END) / COUNT(*) * 100, 1) as percent_active
FROM assets a
LEFT JOIN latest_state l ON a.asset_id = l.asset_id
GROUP BY a.company
ORDER BY percent_active DESC`)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var company string
		var totalAssets int
		var totalDischarging int
		var totalOffline int
		var percentActive float64
		rows.Scan(&company, &totalAssets, &totalDischarging, &totalOffline, &percentActive)

		companysta := StatsCompanies{company, totalAssets, totalDischarging, totalOffline, percentActive}

		stats.Companies = append(stats.Companies, companysta)
	}

	c.JSON(200, stats)
}
func getTopDischarges(c *gin.Context) {
	rows, err := db.Query(`SELECT a.asset_id,
	a.company,
    a.receiving_watercourse,
    l.latest_event_start
FROM assets a
INNER JOIN latest_state l ON l.asset_id = a.asset_id
WHERE l.status = 1
ORDER BY l.latest_event_start
LIMIT 20
`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var response TopDischargingStats
	for rows.Next() {
		var assetId string
		var company string
		var receivingWatercourse string
		var dischargeStart time.Time

		err := rows.Scan(&assetId, &company, &receivingWatercourse, &dischargeStart)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
		}

		asset := TopDischarge{
			assetId,
			company,
			receivingWatercourse,
			dischargeStart,
		}

		response.Discharges = append(response.Discharges, asset)
	}

	c.JSON(200, response)
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

type EventsResponse struct {
	Events []Event `json:"events"`
}

type Event struct {
	AssetID    string    `json:"asset_id"`
	EventStart time.Time `json:"event_start"`
	EventEnd   time.Time `json:"event_end"`
	Duration   int       `json:"duration_minutes"`
}

type AssetResponse struct {
	Total  int            `json:"total_assets"`
	Assets []MinimalAsset `json:"assets"`
}

type MinimalAsset struct {
	AssetId        string     `json:"asset_id"`
	Latitude       float64    `json:"latitude"`
	Longitude      float64    `json:"longitude"`
	Status         int        `json:"status"`
	LatestEventEnd *time.Time `json:"latest_event_end"`
}
type Asset struct {
	ID                                string     `json:"asset_id"`
	Company                           string     `json:"company"`
	ReceivingWatercourse              string     `json:"receiving_watercourse"`
	Latitude                          float64    `json:"latitude"`
	Longitude                         float64    `json:"longitude"`
	Status                            int        `json:"status"`
	LatestEventStart                  *time.Time `json:"latest_event_start"`
	LatestEventEnd                    *time.Time `json:"latest_event_end"`
	PolledAt                          time.Time  `json:"polled_at"`
	NearestBathingWaterName           *string    `json:"nearest_bathing_water_name"`
	NearestBathingWaterID             *string    `json:"nearest_bathing_water_id"`
	NearestBathingWaterClassification *string    `json:"nearest_bathing_water_classification"`
	NearestBathingWaterDistance       *int       `json:"nearest_bathing_water_distance"`
}

type Stats struct {
	TotalAssets      int              `json:"total_assets"`
	TotalDischarging int              `json:"total_discharging"`
	TotalOffline     int              `json:"total_offline"`
	Companies        []StatsCompanies `json:"companies"`
}

type StatsCompanies struct {
	Company          string  `json:"company"`
	TotalAssets      int     `json:"total_assets"`
	TotalDischarging int     `json:"total_discharging"`
	TotalOffline     int     `json:"total_offline"`
	PercentActive    float64 `json:"percent_active"`
}

type TopDischargingStats struct {
	Discharges []TopDischarge `json:"discharges"`
}

type TopDischarge struct {
	AssetID              string    `json:"asset_id"`
	Company              string    `json:"company"`
	ReceivingWaterCourse string    `json:"receiving_watercourse"`
	DischargeStart       time.Time `json:"discharge_start"`
}

type Config struct {
	DBHost string `json:"db_host"`
	DBUser string `json:"db_user"`
	DBPass string `json:"db_pass"`
	DBName string `json:"db_name"`
	DSN    string `json:"dsn"`
}
