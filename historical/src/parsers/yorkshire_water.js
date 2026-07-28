const path = require("path")
const xlsx = require("xlsx")

module.exports = function parseYorkshireWater(file) {
    const dataFile = path.join(__dirname, file)
    const workbook = xlsx.readFile(dataFile, {
        cellDates: true
    })
    const rawData = xlsx.utils.sheet_to_json(workbook.Sheets[workbook.SheetNames[0]])
    
    const overflows = rawData.map(overflow => ({
        asset_id: overflow["Unique ID"],
        start: new Date(overflow["Discharge Start (GMT)"]),
        end: new Date(overflow["Discharge Stop (GMT)"]),
    }))
    return overflows;
}