import { CronJob } from "cron";
import run from "./run.js";
import client from "./redis.js";

// Nuke stale ass fucking stupid cunt sessions
await client.del("bluesky:access")
await client.del("bluesky:refresh")

await run()

const job = new CronJob("*/5 * * * *", async () => {
    await run()
}, null, true, "Europe/London")