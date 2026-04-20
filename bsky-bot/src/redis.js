import { config } from 'dotenv'
import { createClient } from "redis";

config({ quiet: true })

const client = createClient({
    url: `redis://${process.env["REDIS_USER"]}:${process.env["REDIS_PASS"]}@${process.env["REDIS_HOST"]}:${process.env["REDIS_PORT"]}`
})

await client.connect();

export default client