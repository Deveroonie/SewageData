import connection from "./mysql.js"

export default async function update(dischargeInfo) {
    const discharge = dischargeInfo.discharge
    try {
        switch(dischargeInfo.type) {
            case "start": {
                await connection.execute(
                    `INSERT INTO discharge_notifications (asset_id, event_start, first_seen_at, notified_start) VALUES (?, ?, ?, 1)`,
                    [discharge.asset_id, discharge.latest_event_start, discharge.polled_at]
                )
                break;
            }
            case "24h": {
                await connection.execute(
                    `UPDATE discharge_notifications SET notified_24h = 1 WHERE asset_id = ? AND event_start = ?`,
                    [discharge.asset_id, discharge.latest_event_start]
                )
                break;
            }
            case "ended": {
                await connection.execute(
                    `UPDATE discharge_notifications SET notified_end = 1 WHERE asset_id = ? AND event_start = ?`,
                    [discharge.asset_id, discharge.latest_event_start]
                )
                break;
            }
        }
    } catch(err) {
        console.log("An error has occoured in update.js -\n" + err)
    }
}