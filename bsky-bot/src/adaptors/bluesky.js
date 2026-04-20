import client from "../redis.js";
import Adapter from "./Adaptor.js";
import { AtpAgent, RichText } from "@atproto/api";

const blueskyAgent = new AtpAgent({
    service: 'https://bsky.social',
    persistSession: async (evt, session) => {
        if (!session) return;
        await client.set("bluesky:access", session.accessJwt, { EX: 300 });
        await client.set("bluesky:refresh", session.refreshJwt, { EX: 86400 });
    }
})

const blueskyAdapter = new Adapter({
    post: async(text) => {
        if (!blueskyAgent.session) {
            const refreshJwt = await client.get("bluesky:refresh")
            if (refreshJwt) {
                const accessJwt = await client.get("bluesky:access")
                await blueskyAgent.resumeSession({ accessJwt, refreshJwt })
            } else {
                await blueskyAdapter.login(process.env.BSKY_USER, process.env.BSKY_PASS)
            }
        }
        const richText = new RichText({
            text
        })
        await richText.detectFacets(blueskyAgent)
        const postRecord = {
            $type: 'app.bsky.feed.post',
            text: richText.text,
            facets: richText.facets,
            createdAt: new Date().toISOString(),
        }
        await blueskyAgent.post(postRecord)
    },
    login: async(username, password) => {
        const l = await blueskyAgent.login({ identifier: username, password })
        await client.set("bluesky:access", l.data.accessJwt, { EX: 300 });
        await client.set("bluesky:refresh", l.data.refreshJwt, { EX: 86400 });
    }
})

export { blueskyAgent, blueskyAdapter }