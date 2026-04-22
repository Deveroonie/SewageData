import post from "./post.js";
import generateLists from "./util/generateLists.js";
import validate from "./validate.js";
import update from "./update.js";
let running = false
export default async function run() {
    if (running) {
        console.log('[RUN] Skipping — previous run still in progress')
        return
    }
    running = true

    try {
        const lists = await generateLists()
        const validated = await validate(lists)
        for (const item of validated) await post(item)
        for (const item of validated) await update(item)
    } catch (err) {
        console.log("An error has occoured in run.js -\n" + err)
    } finally {
        running = false
    }
}
