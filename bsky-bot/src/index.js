import { config as configDotEnv } from 'dotenv'
configDotEnv({ quiet: true })
import "./instrument.js";
import { CronJob } from "cron";
import run from "./run.js";
import client from "./redis.js";
import config from "./config.json" with { type: "json" };

// Nuke stale sessions
if (config.useRedisBsky) {
    await client.del("bluesky:access")
    await client.del("bluesky:refresh")
}

await run()

const job = new CronJob("*/5 * * * *", async () => {
    await run()
}, null, true, "Europe/London")
