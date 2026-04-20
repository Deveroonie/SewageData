import { createPool } from "mysql2/promise";
import { config } from 'dotenv'
config({ quiet: true })

const connection = createPool({
    host: process.env["MYSQL_HOST"],
    port: process.env["MYSQL_PORT"],
    user: process.env["MYSQL_USER"],
    password: process.env["MYSQL_PASS"],
    database: process.env["MYSQL_DB"]
})

/**
 * Exports a Mysql2 connection pool.
 * 
 * @module
 * @returns {import("mysql2").Pool}
 */
export default connection