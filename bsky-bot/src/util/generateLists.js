import { getActiveDischarges, getPendingNotifications } from "./queries.js";

export default async function generateLists() {
    const active = await getActiveDischarges()
    const pending = await getPendingNotifications()
    const ended = pending.filter(n => !active.find(d => d.asset_id === n.asset_id))

    return {
        active,
        ended
    }
}