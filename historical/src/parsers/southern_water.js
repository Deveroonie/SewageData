const fs = require("fs")
const path = require("path")
const xlsx = require("xlsx")

module.exports = function parseSouthernWater(year, file, mapFile) {
    switch(year) {
        case 2025: {
            const dataFile = path.join(__dirname, file)
            const workbook = xlsx.readFile(dataFile, {
                cellDates: true
            })

            const SOs = xlsx.utils.sheet_to_json(workbook.Sheets["2025 Storm Overflows"])
            const EOs = xlsx.utils.sheet_to_json(workbook.Sheets["2025 Emergency Overflows"])
            // We don't care about the distinction - we just want a clean array.
            const rawData = [...SOs, ...EOs]

            const overflows = rawData.map(overflow => ({
                asset_id: overflow.UniqueID,
                start: new Date(overflow["Start Time"]),
                end: new Date(overflow["End Time"])
            }))

            return overflows;
        }
        case 2024: {
            const dataFile = path.join(__dirname, file)
            const mapDataFile = path.join(__dirname, mapFile) // assume the map file is 2025's data

            const workbook = xlsx.readFile(dataFile, {
                cellDates: true
            })

            const mapWorkbook = xlsx.readFile(mapDataFile)

            const MapSOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Storm Overflows"])
            const MapEOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Emergency Overflows"])
            
            const map = Object.fromEntries(
                [...MapSOs, ...MapEOs].map(i => [i["Overflow ID"], i["UniqueID"]])
            )

            const SOs = xlsx.utils.sheet_to_json(workbook.Sheets["2024 Storm Overflows"])
            const EOs = xlsx.utils.sheet_to_json(workbook.Sheets["2024 Emergency Overflows"])

            const rawData = [...SOs, ...EOs]

            const overflows = rawData.map(overflow => ({
                asset_id: map[overflow["Overflow ID"]],
                start: new Date(overflow["Start Time"]),
                end: new Date(overflow["End Time"])
            })).filter(i => i.asset_id != undefined)

            return overflows;
        }
        case 2023: {
            const dataFile = path.join(__dirname, file)
            const mapDataFile = path.join(__dirname, mapFile) // assume the map file is 2025's data

            const workbook = xlsx.readFile(dataFile, {
                cellDates: true
            })

            const mapWorkbook = xlsx.readFile(mapDataFile)

            const MapSOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Storm Overflows"])
            const MapEOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Emergency Overflows"])
            
            const map = Object.fromEntries(
                [...MapSOs, ...MapEOs].map(i => [i["Overflow ID"], i["UniqueID"]])
            )
            const SOs = xlsx.utils.sheet_to_json(workbook.Sheets["2023 Storm Overflows"])
            const EOs = xlsx.utils.sheet_to_json(workbook.Sheets["2023 Emergency Overflows"])

            const rawData = [...SOs, ...EOs]

            const overflows = rawData.map(overflow => ({
                asset_id: map[overflow["OTE"]],
                start: new Date(overflow["Start Time"]),
                end: new Date(overflow["End Time"])
            })).filter(i => i.asset_id != undefined)

            return overflows;
        }
        case 2022: {
            const dataFile = path.join(__dirname, file)
            const mapDataFile = path.join(__dirname, mapFile) // assume the map file is 2025's data

            const workbook = xlsx.readFile(dataFile)

            const mapWorkbook = xlsx.readFile(mapDataFile)

            const MapSOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Storm Overflows"])
            const MapEOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Emergency Overflows"])
            
            const map = Object.fromEntries(
                [...MapSOs, ...MapEOs].map(i => [i["Overflow ID"], i["UniqueID"]])
            )
            const SOs = xlsx.utils.sheet_to_json(workbook.Sheets["2022 Storm 1224 Count"])
            const EOs = xlsx.utils.sheet_to_json(workbook.Sheets["2022 Emergency 1224 Count"])

            const rawData = [...SOs, ...EOs]

            const overflows = rawData.map(overflow => ({
                asset_id: map[overflow["Overflow"]],
                start: parse2022Date(overflow.CR_StartDate + overflow.CR_StartTime),
                end: parse2022Date(overflow.CR_EndDate + overflow.CR_EndTime)
            })).filter(i => i.asset_id != undefined)

            return overflows
        }
        case 2021: {
            const dataFile = path.join(__dirname, file)
            const mapDataFile = path.join(__dirname, mapFile) // assume the map file is 2025's data

            const workbook = xlsx.readFile(dataFile)

            const mapWorkbook = xlsx.readFile(mapDataFile)

            const MapSOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Storm Overflows"])
            const MapEOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Emergency Overflows"])
            
            const map = Object.fromEntries(
                [...MapSOs, ...MapEOs].map(i => [i["Overflow ID"], i["UniqueID"]])
            )
            const rawData = xlsx.utils.sheet_to_json(workbook.Sheets["2021 Calculated"])

            const overflows = rawData.map(overflow => ({
                asset_id: map[overflow["Overflow"]],
                start: parse2022Date(overflow.CR_StartDate + overflow.CR_StartTime),
                end: parse2022Date(overflow.CR_EndDate + overflow.CR_EndTime)
            })).filter(i => i.asset_id != undefined)

            return overflows
        }
        case 2020: {
            const dataFile = path.join(__dirname, file)
            const mapDataFile = path.join(__dirname, mapFile) // assume the map file is 2025's data

            const workbook = xlsx.readFile(dataFile)

            const mapWorkbook = xlsx.readFile(mapDataFile)

            const MapSOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Storm Overflows"])
            const MapEOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Emergency Overflows"])
            
            const map = Object.fromEntries(
                [...MapSOs, ...MapEOs].map(i => [i["Overflow ID"], i["UniqueID"]])
            )
            const rawData = xlsx.utils.sheet_to_json(workbook.Sheets["EACalculatedData2020"])

            const overflows = rawData.map(overflow => ({
                asset_id: map[overflow["CR_NSUN"]],
                start: parse2022Date(overflow.CR_StartDate + overflow.CR_StartTime),
                end: parse2022Date(overflow.CR_EndDate + overflow.CR_EndTime)
            })).filter(i => i.asset_id != undefined)

            return overflows;
        }
        case 2019: {
            const dataFile = path.join(__dirname, file)
            const mapDataFile = path.join(__dirname, mapFile) // assume the map file is 2025's data

            const workbook = xlsx.readFile(dataFile)

            const mapWorkbook = xlsx.readFile(mapDataFile)

            const MapSOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Storm Overflows"])
            const MapEOs = xlsx.utils.sheet_to_json(mapWorkbook.Sheets["2025 Emergency Overflows"])
            
            const map = Object.fromEntries(
                [...MapSOs, ...MapEOs].map(i => [i["Overflow ID"], i["UniqueID"]])
            )
            const rawData = xlsx.utils.sheet_to_json(workbook.Sheets["Processed Data"])

            const overflows = rawData.map(overflow => ({
                asset_id: map[overflow["Site Number"]],
                start: parse2022Date(overflow["Start Date"] + overflow["Start Time"]),
                end: parse2022Date(overflow["End Date"] + overflow["End Time"]),
            })).filter(i => i.asset_id != undefined)

            return overflows;
        }
    }
}
function parse2022Date(serial) {
    const utcMs = Math.round((serial - 25569) * 86400 * 1000);
    return new Date(utcMs);
}