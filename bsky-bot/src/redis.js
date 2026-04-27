import { createClient } from "redis";
const client = createClient({
    url: `redis://${process.env["REDIS_USER"]}:${process.env["REDIS_PASS"]}@${process.env["REDIS_HOST"]}:${process.env["REDIS_PORT"]}`
})

await client.connect();

export default client