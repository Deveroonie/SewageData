import connection from "../mysql.js";

 const table = (process.env["ENV"] == "PRODUCTION"? "discharge_notifications" : "discharge_notifications_test")
    console.log(table)
async function getActiveDischarges() {
    const [rows] = await connection.query(`
    SELECT 
        ls.asset_id,
        ls.latest_event_start,
        ls.polled_at,
        a.company,
        a.receiving_watercourse,
        a.latitude,
        a.longitude,
        a.nearest_bw_name,
        a.nearest_bw_classification,
        a.nearest_bw_distance_m,
        dn.id AS notification_id,
        dn.notified_start,
        dn.notified_24h,
        dn.notified_end
    FROM latest_state ls
    JOIN assets a ON ls.asset_id = a.asset_id
    LEFT JOIN ${table} dn 
    ON dn.asset_id = ls.asset_id 
    AND dn.event_start = ls.latest_event_start
    WHERE ls.status = 1
    AND a.nearest_bw_distance_m IS NOT NULL
    AND a.nearest_bw_distance_m <= 3218
    AND a.nearest_bw_classification != 'Closed'
    `)
    return rows
}

async function getPendingNotifications() {
    const [rows] = await connection.query(`
    SELECT dn.*, 
           dn.event_start AS latest_event_start,
           ls.latest_event_end,
           a.company,
           a.nearest_bw_name,
           a.nearest_bw_classification,
           a.nearest_bw_distance_m
    FROM ${table} dn
    LEFT JOIN latest_state ls ON dn.asset_id = ls.asset_id
    LEFT JOIN assets a ON dn.asset_id = a.asset_id
    WHERE dn.notified_end = 0
    AND dn.notified_start = 1
    `)
    return rows
}

async function getEventCount(cso) {
    const [rows] = await connection.query(`
        SELECT COUNT(*) AS count
        FROM events
        WHERE asset_id = ?
        AND event_start >= NOW() - INTERVAL 30 DAY`, [cso]
    )
    return rows
}
export { getActiveDischarges, getPendingNotifications, getEventCount }