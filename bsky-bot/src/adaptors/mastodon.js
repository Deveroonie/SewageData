import { createRestAPIClient } from "masto";
import Adapter from "./Adaptor.js";

const mastodonClient = createRestAPIClient({
    url: "https://mastodon.social",
    accessToken: process.env["MASTODON_TOKEN"]
})

const mastodonAdapter = new Adapter({
    post: async(text) => {
        await mastodonClient.v1.statuses.create({
            status: text,
            visibility: "public"
        })
    },
    login: async(username, password) => {
        // This function is not needed, however it is mandatory in the Adapter class.
    }
})

export { mastodonClient, mastodonAdapter }