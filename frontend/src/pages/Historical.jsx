/* global umami */
import { Layer, Map, Source } from "react-map-gl/maplibre";
import 'maplibre-gl/dist/maplibre-gl.css';
import { useEffect, useMemo, useState } from "react";
import axios from "axios";
import base from "../api_base"
import { createColumnHelper, flexRender, getCoreRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table";
import { NavLink } from "react-router";

export default function Historical() {
    const [stats,setStats] = useState(null)
    const [assets,setAssets] = useState(null)
    const [selectedInfo, setSelectedInfo] = useState(null)
    const [selectedId, setSelectedId] = useState(null)
    const [selectedEvents, setSelectedEvents] = useState([])
    const [selectedFirst, setSelectedFirst] = useState(null)

    /// Data Fetching

    useEffect(() => {
        async function fetchStats() {
            const res = ((await axios.get(`${base}/api/assets/historic-events/assets-with-historic-events`)).data)
            
            setStats({
                total: res.total_assets,
                matched: (res.total_assets - res.unmatched_total),
                unmatched: res.unmatched_total
            })

            setAssets(res.assets)
        }
        fetchStats()
    }, [])

    async function changeSelected(id) {
        const data = (await axios.get(`${base}/api/asset/${id}/historic-events`)).data
        const info = (await axios.get(`${base}/api/asset/${id}`)).data
        const first = (await axios.get(`${base}/api/asset/${id}/historic-events/has-historic-events`)).data.first_historic_event


        setSelectedId(id)
        setSelectedInfo(info)
        setSelectedFirst(new Date(first))
        setSelectedEvents(data.events)

        umami.track('historic_cso_viewed', {
          cso: id,
        });
    }

    /// :3

    const accuracyNotes = {
        "Southern Water": `The data received from Southern Water is processed using the <b>12/24</b> counting method. This means that discharges that occurred closer together may be counted as one discharge, instead of the true count.<br />Some data from prior to 2020 may be missing. This is because we were only able to map roughly <b>a quarter</b> of discharges prior to 2020 to an asset in our database.`
    }

    /// Map tomfoolery

    const geojson = {
        type: 'FeatureCollection',
        features: (assets ?? []).map(asset => {
            return {
                type: 'Feature',
                geometry: { type: 'Point', coordinates: [asset.longitude, asset.latitude] },
                properties: { asset_id: asset.asset_id, company: asset.company }
            }
        })
    }

    const baseCircleStyle = {
      type: 'circle',
      paint: {
          'circle-radius': 5,
          'circle-opacity': 0.85
      }
    }

    const mapLayer  = { ...baseCircleStyle, id: 'assets', paint: { ...baseCircleStyle.paint, 'circle-color': 'blue'   } }

    /// Table Tomfoolery
    const columnHelper = createColumnHelper()
    const columns = useMemo(() => [
      columnHelper.accessor("asset_id", { header: "Asset ID", enableSorting: false }),
      columnHelper.accessor("event_start", { header: "Discharge Start", cell: info => dates(info.getValue()) }),
      columnHelper.accessor("event_end", { header: "Discharge End", cell: info => dates(info.getValue()) }),
      columnHelper.accessor("duration_minutes", { header: "Discharge Duration (hours)", cell: info => (info.getValue()/60).toLocaleString() }),
    ], [])

    function EventsTable({ data, columns }) {
        const [sorting, setSorting] = useState([])

        const table = useReactTable({
          data,
          columns,
          state: { sorting },
          onSortingChange: setSorting,
          getCoreRowModel: getCoreRowModel(),
          getSortedRowModel: getSortedRowModel(),
        })
    
        return (
          <table>
            <thead>
              {table.getHeaderGroups().map(headerGroup => (
                <tr key={headerGroup.id}>
                  {headerGroup.headers.map(header => (
                    <th
                      key={header.id}
                      onClick={header.column.getToggleSortingHandler()}
                      className="cursor-pointer select-none border p-4"
                    >
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {{
                        asc: ' 🔼',
                        desc: ' 🔽',
                      }[header.column.getIsSorted()] ?? null}
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map(row => (
                <tr key={row.id}>
                  {row.getVisibleCells().map(cell => (
                    <td key={cell.id} className="border p-4">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        )
    }

    if(!stats || !assets) return <div>Loading..?</div>

    return (
        <div className="flex flex-col h-screen">
            <div className='shrink-0 bg-brown text-white px-8 py-4 text-center'>
                <span className='lg:text-3xl text-xl font-bold'>Sewage Data - Historical</span>
                <p className='lg:text-lg text-sm'>We have managed to obtain historical discharge data - similar to the 'historical' data shown on our Live Map - for most Sewage companies in Great Britain. This page shows what we have managed to interpret.<br />
                We have inserted data from <b>{stats.total.toLocaleString()}</b> distinct CSOs. Of that, we have managed to map this data to <b>{stats.matched.toLocaleString()}</b> CSOs that are already in our database. Each pin represents a CSO with historical data - click on the pin to see it's discharge history!</p>
                Click <NavLink className={"underline"} to="/">here</NavLink> to return to our live map.
            </div>
            <div className="flex flex-row h-[70vh] shrink-0">
                <div className="grow">
                    <Map
                        initialViewState={{ longitude: -2.5, latitude: 54.5, zoom: 5 }}
                        style={{ width: '100%', height: '100%' }}
                        minZoom={4}
                        mapStyle="https://tiles.openfreemap.org/styles/liberty"
                        interactiveLayerIds={["assets"]}
                        onClick={async e => {
                            const feature = e.features?.[0]
                            if (feature) {
                              await changeSelected(feature.properties.asset_id)
                            }
                        }}
                    >
                        <Source id="assets" type="geojson" data={geojson}>
                            <Layer {...mapLayer} />
                        </Source>
                    </Map>
                </div>
            </div>
            <div>
                {selectedId && (
                    <div className="shrink-0 border-t flex flex-col min-h-screen justify-center items-center">
                        {accuracyNotes[selectedInfo.company] && (
                            <div className="p-6 rounded-md bg-blue-400 text-white text-center w-fit">
                                <b>Accuracy Info:</b>&nbsp;<span dangerouslySetInnerHTML={{__html: accuracyNotes[selectedInfo.company]}}></span>
                            </div>
                        )}
                        <div className="p-6 rounded-md bg-brown text-white text-center w-fit">
                            <b>CSO Info</b>&nbsp;Asset ID: {selectedInfo.asset_id}. Discharges Into: {selectedInfo.receiving_watercourse}, Operated By: {selectedInfo.company}. Historic Discharges: {selectedEvents.length}, oldest: {selectedFirst.getFullYear().toString()}
                        </div>
                        <EventsTable data={selectedEvents} columns={columns} />
                    </div>
                )}
            </div>
        </div>
    )
}

function dates(date) {
    const d = new Date(date)
    return `${d.getDate().toString().padStart(2, "0")}/${(d.getMonth()+1).toString().padStart(2, "0")}/${d.getFullYear()} ${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}:${d.getSeconds().toString().padStart(2, "0")}`
}