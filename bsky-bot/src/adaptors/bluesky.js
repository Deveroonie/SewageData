import client from "../redis.js";
import Adapter from "./Adaptor.js";
import { AtpAgent, RichText } from "@atproto/api";
import config from "../config.json" with { type: "json" };

const blueskyAgent = new AtpAgent({
    service: 'https://bsky.social',
    persistSession: async (evt, session) => {
        if (!session) return;
        if (config.useRedisBsky) {
            await client.set("bluesky:access", session.accessJwt, { EX: 300 });
            await client.set("bluesky:refresh", session.refreshJwt, { EX: 86400 });
        }
    }
})

const blueskyAdapter = new Adapter({
    post: async(text, image) => {
        console.log('🔵 BSKY POST CALLED - image type:', typeof image, '- falsy?', !image)
        if (!blueskyAgent.session) {
            if(config.useRedisBsky) {
                const refreshJwt = await client.get("bluesky:refresh")
                if (refreshJwt) {
                    const accessJwt = await client.get("bluesky:access")
                    await blueskyAgent.resumeSession({ accessJwt, refreshJwt })
                } else {
                    await blueskyAdapter.login(process.env.BSKY_USER, process.env.BSKY_PASS)
                }
            } else {
                await blueskyAdapter.login(process.env.BSKY_USER, process.env.BSKY_PASS)
            }
        }
        const richText = new RichText({
            text
        })
        await richText.detectFacets(blueskyAgent)
        let postRecord; 
        if(image) {
    
                const { data } = await blueskyAgent.uploadBlob(image, {
                    encoding: 'image/png'
                })
                
                console.log('Blob response:', JSON.stringify(data.blob))
            postRecord = {
                $type: 'app.bsky.feed.post',
                text: richText.text,
                facets: richText.facets,
                createdAt: new Date().toISOString(),
                embed: {
                    $type: "app.bsky.embed.images",
                    images: [
                        {
                            image: data.blob,
                            alt: "",
                            aspectRatio: {
                                width: 1200,
                                height: 675
                            }
                        }
                    ]
                }
            }
        } else {
            postRecord = {
                $type: 'app.bsky.feed.post',
                text: richText.text,
                facets: richText.facets,
                createdAt: new Date().toISOString(),
            }
        }
        await blueskyAgent.post(postRecord)
    },
    login: async(username, password) => {
        const l = await blueskyAgent.login({ identifier: username, password })
        if (config.useRedisBsky) {
            await client.set("bluesky:access", l.data.accessJwt, { EX: 300 });
            await client.set("bluesky:refresh", l.data.refreshJwt, { EX: 86400 });
        }
    }
})

export { blueskyAgent, blueskyAdapter }