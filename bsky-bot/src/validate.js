export default function validate(lists) {
    // destructure
    const { active, ended } = lists
    let list = []
    for (const discharge of active) {
        const twentyFourHours = 24 * 60 * 60 * 1000
        if (Date.now() - new Date(discharge.polled_at) > twentyFourHours) continue
        if(discharge.latest_event_end) continue
        if(discharge.notified_start && !discharge.notified_24h && Date.now() - new Date(discharge.latest_event_start) >= 24 * 60 * 60 * 1000) {
            list.push({type: "24h", discharge})
        } else if (!discharge.notified_start) {
            list.push({type: "start", discharge})
        }
    }
    for (const discharge of ended) {
        list.push({type: "ended", discharge})
    }

    return list;
}