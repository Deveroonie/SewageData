import { blueskyAdapter } from "./adaptors/bluesky.js";
import { mastodonAdapter } from "./adaptors/mastodon.js";

export default async function post(postInfo) {
    const discharge = postInfo.discharge
    try {
        switch (postInfo.type) {
            case "start": {
                const text = buildPost(
                        `🚨 NEW DISCHARGE NEAR ${discharge.nearest_bw_name}`,
                        `💩 ${discharge.company} has started discharging raw sewage ${mtomi(discharge.nearest_bw_distance_m)}mi from this bathing water`,
                        `📋 This bathing water was rated ${discharge.nearest_bw_classification} by the EA`,
                        `⏰ This discharge started at ${datetoHHMM(discharge.latest_event_start)}`,
                        `🔗 https://sewagedata.co.uk/?asset=${discharge.asset_id}`
                    )
                
                await blueskyAdapter.post(text)
                await mastodonAdapter.post(text)
                break;
            }
            case "24h": {
                const text = buildPost(
                    `🚨 ONGOING DISCHARGE NEAR ${discharge.nearest_bw_name}`,
                    `💩 ${discharge.company} has been discharging raw sewage ${mtomi(discharge.nearest_bw_distance_m)}mi from this bathing water for ${hoursAgo(discharge.latest_event_start)} hours`,
                    `📋 This bathing water was rated ${discharge.nearest_bw_classification} by the EA`,
                    `⏰ This discharge started at ${datetoHHMM(discharge.latest_event_start)} ${datetoDDMMYYYY(discharge.latest_event_start)}`,
                    `🔗 https://sewagedata.co.uk/?asset=${discharge.asset_id}`
                )
                await blueskyAdapter.post(text)
                await mastodonAdapter.post(text)
                break;
            }

            case "ended": {
                const text = buildPost(
                    `✅ DISCHARGE ENDED NEAR ${discharge.nearest_bw_name}`,
                    `💩 ${discharge.company} has stopped discharging raw sewage ${mtomi(discharge.nearest_bw_distance_m)}mi from this bathing water. This discharge lasted for ${hoursAgo(discharge.latest_event_start, discharge.latest_event_end)} hours`,
                    `📋 This bathing water was rated ${discharge.nearest_bw_classification} by the EA`,
                    `⏰ This discharge started at ${datetoHHMM(discharge.latest_event_start)} ${datetoDDMMYYYY(discharge.latest_event_start)}, and ended at ${datetoHHMM(discharge.latest_event_end)} ${datetoDDMMYYYY(discharge.latest_event_end)}`,
                    `🔗 https://sewagedata.co.uk/?asset=${discharge.asset_id}`
                )
                await blueskyAdapter.post(text)
                await mastodonAdapter.post(text)
                break;
            }
        }
    } catch(err) {
        console.log("An error has occoured in post.js -\n" + err)
    }
}

function mtomi(m) {
    const mi = m/1609
    return mi.toFixed(2)
}

function datetoHHMM(date) {
    const d = new Date(date)
    return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`
}

function datetoDDMMYYYY(date) {
    const d = new Date(date)
    return `${d.getDate().toString().padStart(2, "0")}/${(d.getMonth()+1).toString().padStart(2, "0")}/${d.getFullYear()}`
}

function hoursAgo(date, date2) {
  const d1 = new Date(date);
  let d2 = null;
  if(date2) d2 = new Date(date2)
    else d2 = new Date()
  
  const diffInMs = Math.abs(d2 - d1);
  
  const hours = diffInMs / (1000 * 60 * 60);
  
  return hours.toFixed(1);
}

function buildPost(header, body, classification, time, url) {
    const limit = 300
    const fixed = `${header}\n${classification}\n${time}\n\n${url}`
    const fixedLen = [...fixed].length
    const bodyBudget = limit - fixedLen - 1 // -1 for the \n after body
    
    const trimmedBody = [...body].length > bodyBudget 
        ? [...body].slice(0, bodyBudget - 1).join('') + '…'
        : body
    
    return `${header}\n${trimmedBody}\n${classification}\n${time}\n\n${url}`
}