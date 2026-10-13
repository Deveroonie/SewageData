const hour = 60 * 60 * 1000
const twentyFourHours = 24 * hour
// The bot runs every 5 minutes, so a real 24h crossing always lands in this
// window. Anything older (bot down, or a stale row) isn't news any more.
const ongoingAlertWindow = 36 * hour

export default function validate(lists) {
    // destructure
    const { active, pending } = lists
    let list = []
    for (const discharge of active) {
        if(discharge.latest_event_end) continue
        const age = Date.now() - new Date(discharge.latest_event_start)
        if (!discharge.notified_start) {
            // polled_at is when the fetcher first saw this event, not when it
            // last polled. Don't announce events it found over a day ago.
            if (Date.now() - new Date(discharge.polled_at) > twentyFourHours) continue
            list.push({type: "start", discharge})
        } else if (!discharge.notified_24h && age >= twentyFourHours && age < ongoingAlertWindow) {
            list.push({type: "24h", discharge})
        }
    }
    for (const notification of pending) {
        const type = endedType(notification)
        if (type) list.push({type, discharge: notification})
    }

    return list;
}

// endedType decides what to do with a notification whose end hasn't been posted:
// "ended" posts it, "closed" marks it done without posting, null waits.
function endedType(notification) {
    // Still the asset's current event and still discharging.
    if (notification.is_current_event && notification.status == 1) return null
    if (notification.latest_event_end) {
        // Finished, and we know when. Only post it while it's news, and while
        // the asset still has a bathing water to name.
        const sinceEnd = Date.now() - new Date(notification.latest_event_end)
        if (sinceEnd > twentyFourHours || !notification.nearest_bw_name) return "closed"
        return "ended"
    }
    // Over, but its end was never reported: a newer event replaced it, the asset
    // left latest_state, or it stopped after an offline spell. Don't post a
    // made-up end time.
    if (!notification.is_current_event || notification.status == 0) return "closed"
    // Offline mid-discharge: we don't know yet.
    return null
}
