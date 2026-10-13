import { getActiveDischarges, getPendingNotifications } from "./queries.js";

export default async function generateLists() {
    const active = await getActiveDischarges()
    const pending = await getPendingNotifications()

    return {
        active,
        pending
    }
}