import express from "express"
import playwright from "playwright"
import * as path from "path"
import { fileURLToPath } from "url"


const __dirname = path.dirname(fileURLToPath(import.meta.url))

const app = express()
const browser = await playwright.chromium.launch()

app.use("/assets", express.static(path.join(__dirname, "../dist/assets")))
app.get("/card", (req,res) => {
    res.sendFile(path.join(__dirname, "../dist/index.html"))
})

app.get("/render", async(req,res) => {
    const page = await browser.newPage()
    await page.setViewportSize({ width: 1200, height: 675 })
    await page.goto(`http://localhost:3040/card?${new URLSearchParams(req.query).toString()}`)
    await page.waitForFunction(() => window.__mapReady)
    await page.evaluate(() => document.fonts.ready)
    const buffer = await page.screenshot({ type: 'png' })
    await page.close()
    res.type('png').send(buffer)
})

app.listen(3040)