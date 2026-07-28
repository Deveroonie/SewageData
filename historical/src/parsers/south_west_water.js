const fs = require("fs")
const path = require("path")
const xlsx = require("xlsx")

module.exports = function parseSouthWestWater(year, file, mapFile) {
    switch(year) {
        case 2025: {
            const dataFile = path.join(__dirname, file)

            const workbook = xlsx.readFile(dataFile)

            const rawData = xlsx.utils.sheet_to_json(workbook.Sheets["Data"])
            
            const overflows = rawData.map(overflow => ({
                asset_id: overflow["Unique ID"],
                start: parseDate(overflow["Discharge Start Date"], overflow["Discharge Start Time"]),
                end: parseDate(overflow["Discharge Stop Date"], overflow["Discharge Stop Time"]),
            }))

            return overflows;
        }
        case 2022: {
            const dataFile = path.join(__dirname, file)
            const mapDataFile = path.join(__dirname, mapFile) // we use the NP

            const mapRaw = fs.readFileSync(mapDataFile, "utf-8").split("\n").slice(1)

            const map = Object.fromEntries(mapRaw.map(row =>[row.split(",")[1], row.split(",")[0]]))
            //console.log(map)
            const workbook = xlsx.readFile(dataFile)

            const rawData = xlsx.utils.sheet_to_json(workbook.Sheets["Data"])
            
            const overflows = rawData.map(overflow => ({
                asset_id: map[overflow["Unique ID"]],
                start: parseDate(overflow["Discharge Start Date"], overflow["Discharge Start Time"]),
                end: parseDate(overflow["Discharge Stop Date"], overflow["Discharge Stop Time"]),
            }))

            return overflows;
        }
    }
}


function parseDate(dateSerial, timeStr) {
    // Convert the date serial to midnight UTC
    const utcDays = Math.floor(dateSerial - 25569);
    const dayMs = utcDays * 86400 * 1000;
    // Parse "hh:mm:ss" into ms
    const [hours, minutes, seconds] = timeStr.split(':').map(Number);
    const timeMs = (hours * 3600 + minutes * 60 + (seconds || 0)) * 1000;
    return new Date(dayMs + timeMs);
}