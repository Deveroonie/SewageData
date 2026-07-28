const config = require("./config.json")
const mysql = require("mysql2/promise");
const parseSouthernWater = require("./parsers/southern_water")
const parseSouthWestWater = require("./parsers/south_west_water")
const parseYorkshireWater = require("./parsers/yorkshire_water")

const args = process.argv.slice(2);

// 0 - WASC
// 1 - year
// 2 - file
// 3 - mapfile

async function insert() {
    const connection = await mysql.createConnection({
        host: config.sql.host,
        user: config.sql.user,
        password: config.sql.password,
        database: config.sql.database
    });
    switch(args[0]) {
        case "southernwater": {
            console.time('build');
            const data = parseSouthernWater(parseInt(args[1]), args[2], args[3])
            const values = data.map(row => [row.asset_id, row.start, row.end]);
            console.timeEnd('build');
            console.time('insert');
            const [result] = await connection.query(
                `INSERT IGNORE INTO ${config.sql.table} (asset_id, event_start, event_end) VALUES ?`,
                [values]
            );
            console.timeEnd('insert');

            console.log(`Inserted ${result.affectedRows} rows`);
            await connection.end();
            break;
        }
        case "southwestwater": {
            console.time('build');
            const data = parseSouthWestWater(parseInt(args[1]), args[2], args[3])
            const values = data.map(row => [row.asset_id, row.start, row.end]);
            console.timeEnd('build');
            console.time('insert');
            const [result] = await connection.query(
                `INSERT IGNORE INTO ${config.sql.table} (asset_id, event_start, event_end) VALUES ?`,
                [values]
            );
            console.timeEnd('insert');

            console.log(`Inserted ${result.affectedRows} rows`);
            await connection.end();
            break;
        }
        case "yorkshirewater": {
            console.time('build');
            const data = parseYorkshireWater(args[1])
            const values = data.map(row => [row.asset_id, row.start, row.end]);
            console.timeEnd('build');
            console.time('insert');
            const [result] = await connection.query(
                `INSERT IGNORE INTO ${config.sql.table} (asset_id, event_start, event_end) VALUES ?`,
                [values]
            );
            console.timeEnd('insert');

            console.log(`Inserted ${result.affectedRows} rows`);
            await connection.end();
            break;
        }
    }
}
(async() => {
    await insert()
})()

/* INSERTED:
* southernwater 2025/2024/2023/2022/2021/2020/2017-19
* southwestwater 2025/2024/2023/2022/2021/2020
* yorkshirewater 2025/2024/2023/2022/2021
*/