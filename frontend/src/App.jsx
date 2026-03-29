/* global umami */
import Map, { Source, Layer, Popup } from 'react-map-gl/maplibre';
import 'maplibre-gl/dist/maplibre-gl.css';
import { useEffect, useRef, useState } from 'react';
import axios from 'axios';
import { faXmark } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import MobileCompanies from './components/homepage-stats/MobileCompanies';
import DesktopCompanies from './components/homepage-stats/DesktopCompanies';
import MobileCSOs from './components/homepage-stats/MobileCSOs';
import DesktopCSOs from './components/homepage-stats/DesktopCSOs';
import minutesAgo from './util/minutesAgo';
const base = "https://api.sewagedata.co.uk"


function App() {

    const [stats,setStats] = useState(null)
    const [assets,setAssets] = useState(null)
    const [topDischarges,setTopDischarges] = useState(null)
    const [selectedAsset, setSelectedAsset] = useState(null)
    const [assetData, setAssetData] = useState(null)
    const [isDefault, setIsDefault] = useState(null)
    const mapRef = useRef(null)

    async function fetchAsset(assetId, status) {
        if (!assetId || assetId == undefined || assetId == null) {
          setSelectedAsset(null)
          setAssetData(null)
          return;
        }
        setIsDefault(false)
        setSelectedAsset(assetId)
        const assetDataRes = (await axios.get(`${base}/api/asset/${assetId}`)).data
        const assetDataEvents = (await axios.get(`${base}/api/asset/${assetId}/events`)).data?.events
        
        const data = assetDataRes
        data.events = assetDataEvents
        data.hasRecentlyDischarged = (status == 2)
        setAssetData(data)
        umami.track('cso_viewed', {
          cso: assetId,
          direct: !!new URLSearchParams(window.location.search).get('asset'),
        });
    }

    function hoursActive(date) {
      // eslint-disable-next-line react-hooks/purity
      return date ? ((Date.now() - new Date(date).getTime()) / 3600000).toLocaleString("en-GB", {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
      }) : null
    }

    function isWithinLastMonth(date) {
  const d = new Date(date)
  const monthAgo = new Date()
  monthAgo.setMonth(monthAgo.getMonth() - 1)
  return d > monthAgo
}

    function duration(start, end) {
      const st = new Date(start)
      const en = new Date(end)
      // eslint-disable-next-line react-hooks/purity
      return en ? ((en - st) / 3600000).toFixed(1): ((Date.now() - st.getTime()) / 3600000).toFixed(1)
    }

    useEffect(() => {
        async function fetchData() {
            const data = (await axios.get(base+"/api/stats")).data
            const assetData = (await axios.get(base+"/api/assets")).data.assets
            const dischargesData = (await axios.get(base+"/api/top-discharges")).data.discharges
            setStats(data)
            console.log("stats set", data)
            setAssets(assetData)
            console.log("assets set", assetData)
            setTopDischarges(dischargesData)
            console.log("topDischarges set", dischargesData)
        }
        fetchData()

        async function checkURLEntry() {
          const params = new URLSearchParams(document.location.search)
          if(params.has("asset")) {
            const assetEntryID = params.get("asset")
            
            try {
                const asset = (await axios.get(`${base}/api/asset/${assetEntryID}`)).data
                console.log(asset)
                const status = (asset.status != 0 ? asset.status : minutesAgo(new Date(asset.latest_event_end)) < 2880 ? 2 : 0)
                await fetchAsset(assetEntryID, status)
                setIsDefault(true)
                if (mapRef.current) {
                  mapRef.current.flyTo({ 
                    center: [asset.longitude, asset.latitude], 
                    zoom: 13 
                  })
                }
            } catch(err) {
              console.log(err)
            }
          }
        }

        checkURLEntry()
    }, [])

const geojson = {
    type: 'FeatureCollection',
    features: (assets ?? []).map(asset => {
        const effectiveStatus = asset.status !== 0 
            ? asset.status 
            : minutesAgo(new Date(asset.latest_event_end)) < 2880 
                ? 2 
                : 0
        return {
            type: 'Feature',
            geometry: { type: 'Point', coordinates: [asset.longitude, asset.latitude] },
            properties: { status: effectiveStatus, name: asset.name, asset_id: asset.asset_id }
        }
    })
}

    const baseCircleStyle = {
      type: 'circle',
      paint: {
          'circle-radius': 6,
          'circle-opacity': 0.85
      }
    }

    const offlineLayer  = { ...baseCircleStyle, id: 'assets-offline',   filter: ['==', ['get', 'status'], -1], paint: { ...baseCircleStyle.paint, 'circle-color': 'gray'   } }
    const notDischargingLayer = { ...baseCircleStyle, id: 'assets-notdischarging',  filter: ['==', ['get', 'status'], 0],  paint: { ...baseCircleStyle.paint, 'circle-color': 'green'  } }
    const recentDischargeLayer = { ...baseCircleStyle, id: 'assets-recentdischarge', filter: ['==', ['get', 'status'], 2],  paint: { ...baseCircleStyle.paint, 'circle-color': 'yellow' } }
    const dischargingLayer   = { ...baseCircleStyle, id: 'assets-discharging',    filter: ['==', ['get', 'status'], 1],  paint: { ...baseCircleStyle.paint, 'circle-color': 'red'    } }
    console.log("rendering", stats, topDischarges)
    return (
      <>
      <div className='flex flex-col h-screen'>
          <div className='shrink-0 bg-brown text-white px-8 py-4 text-center'>
            <span className='lg:text-3xl text-xl font-bold'>Sewage Data - Map</span>
            <p className='lg:text-lg text-sm'>A live map showing sewage discharges from all water companies across England, Wales* and Scotland.</p>
            <span className='text-gray-300 italic lg:text-sm text-xs'>*excludes Hafren Dyfrdwy<br />Tracking for events began on 01/03/2026. Events before this date will not be displayed.</span><br />
            <p className='lg:text-lg font-semibold'>
              <span className='p-2 rounded-lg font-bold bg-brown-800'>{stats?.total_discharging || "..."}</span> CSOs discharging right now.
            </p>
          </div>
            <div className="flex flex-row flex-1 min-h-0">
              {selectedAsset && assetData && (
                <div className="bg-brown-700 border-2 border-white p-4 text-white flex flex-col overflow-hidden self-stretch  max-md:w-full">
                  <div className='flex flex-row justify-between'>
                    <h2 className="monserrat text-xl font-bold">{assetData.company}</h2>
                    <span className="text-xl cursor-pointer" onClick={() => {fetchAsset(null)}}><FontAwesomeIcon icon={faXmark} /></span> 
                  </div>
                  <span className="text-sm font-semibold monserrat">{assetData.asset_id}</span>
                  <p className=""><span className="font-semibold">Discharges Into:</span>&nbsp;{assetData.receiving_watercourse}</p>
                  <div className={`
                    p-2 border-2 rounded-lg mt-2 font-semibold
                      ${assetData.status == 1 ? "bg-red-500 border-red-700" :
                      assetData.hasRecentlyDischarged ? "bg-orange-500 border-orange-700" :
                      assetData.status == 0 ? "bg-green-500 border-green-700" :
                      "bg-gray-500 border-gray-700"
                    }`}>
                      {assetData.hasRecentlyDischarged ? "Not Discharging (Has Recently)" : assetData.status == 0 ? "Not Discharging": assetData.status == 1 ? "Discharging" : "Monitor Offline"}
                    </div>
                    <p>
                      {assetData.latest_event_end && (
                        <span><span className="font-semibold">Last Discharge Ended:</span>&nbsp;{new Date(assetData.latest_event_end).toLocaleString()}</span>
                      )}
                      {assetData.status == 1 && (
                        <span>
                          <span className="font-semibold">Discharging Since:</span>&nbsp;{new Date(assetData.latest_event_start).toLocaleString()}<br />
                          <span className="font-semibold">Hours Active:</span>&nbsp;{hoursActive(assetData.latest_event_start)}
                        </span>
                      )}<br />
                      {assetData.nearest_bathing_water_id && (
                          <span>
                              <span className="font-semibold">Nearest Bathing Water:</span>&nbsp;{assetData.nearest_bathing_water_name}<br />
                              <span className="font-semibold">Nearest Bathing Water Distance:</span>&nbsp;{assetData.nearest_bathing_water_distance.toLocaleString("en-GB", {minimumFractionDigits: 0, maximumFractionDigits: 0})} m / {(assetData.nearest_bathing_water_distance / 1607).toLocaleString("en-GB", {minimumFractionDigits: 0, maximumFractionDigits: 0})} mi<br />
                              <span className="font-semibold">Nearest Bathing Water Classification:</span>&nbsp;
                              <span className={`
                                ${assetData.nearest_bathing_water_classification == "Excellent" ? "text-blue-400" :
                                assetData.nearest_bathing_water_classification == "Good" ? "text-green-400" :
                                assetData.nearest_bathing_water_classification == "Sufficient"? "text-orange-400" :
                                assetData.nearest_bathing_water_classification == "Poor" ? "text-red-400" : "text-gray-400"}`}>
                                  {assetData.nearest_bathing_water_classification}</span>
                              <br />
                          </span>
                      )}
                      {assetData.events && (
                          <span>
                              <span className="font-semibold">Total Events:</span>&nbsp;{assetData.events.length}<br />
                              {/* eslint-disable-next-line react-hooks/purity */}
                              <span className="font-semibold">Events This Week:</span>&nbsp;{assetData.events.filter(ev => new Date(ev.event_end).getTime() > Date.now() - 604800000).length}<br />
                              <span className="font-semibold">Events This Month:</span>&nbsp;{assetData.events.filter(ev => isWithinLastMonth(ev.event_end)).length}<br />
                          </span>
                      )}
                      <a href={`https://earth.google.com/web/search/${assetData.latitude},${assetData.longitude}/`} target="_blank" className="cursor-pointer mr-2">
                        <button className="p-2 border-2 rounded-lg mt-2 font-semibold bg-blue-500 border-blue-700 cursor-pointer">Open in Google Earth</button>
                      </a>
                      {assetData.nearest_bathing_water_id && (
                          <a href={`https://environment.data.gov.uk/bwq/profiles/profile.html?site=${assetData.nearest_bathing_water_id}`} target="_blank" className="cursor-pointer">
                            <button className="p-2 border-2 rounded-lg mt-2 font-semibold bg-green-500 border-green-700 cursor-pointer">Open EA Bathing Water Quality</button>
                        </a>
                      )}
                    </p>
                    {assetData.events && (
                        <div className="flex flex-col min-h-0 flex-1">
                          <h3 className="text-xl monserrat font-bold">Events</h3>
                          <div className="overflow-y-auto flex-1">
                          {assetData.events.map(ev => (
                              <p key={`${ev.asset_id}-${ev.event_end}-${ev.event_start}`} className="text-sm border-gray-300 border mb-2 p-2">
                                  <span className="font-semibold">Event Start:</span>&nbsp;{new Date(ev.event_start).toLocaleString()}<br />
                                  <span className="font-semibold">Event End:</span>&nbsp;{new Date(ev.event_end).toLocaleString()}<br />
                                  <span className="font-semibold">Event Duration:</span>&nbsp;{duration(ev.event_start, ev.event_end)}h<br />
                              </p>
                          ))}
                          </div>
                        </div>
                    )}
                </div>
              )}
              <div className="grow min-h-0">
                <Map
                  ref={mapRef}
                  initialViewState={{ longitude: -2.5, latitude: 54.5, zoom: 5 }}
                  style={{ width: '100%', height: '100%' }}
                  minZoom={4}
                  mapStyle="https://tiles.openfreemap.org/styles/liberty"
                  onClick={async e => {
                    const feature = e.features?.[0]
                    if (feature) {
                      await fetchAsset(feature.properties.asset_id, feature.properties.status)
                    } else {
                      await fetchAsset(null)
                    }
                  }}
                  interactiveLayerIds={['assets-notdischarging', 'assets-offline', 'assets-recentdischarge', 'assets-discharging']}
                  cursor={selectedAsset ? 'pointer' : 'auto'}
                >
                  <Source id="assets" type="geojson" data={geojson}>
                    <Layer {...notDischargingLayer} />
                    <Layer {...offlineLayer} />
                    <Layer {...recentDischargeLayer} />
                    <Layer {...dischargingLayer} />
                  </Source>
                  {/*{selectedAsset && (
                    <Popup
                      longitude={selectedAsset.longitude}
                      latitude={selectedAsset.latitude}
                      anchor="bottom"
                      onClose={() => setSelectedAsset(null)}
                      closeOnClick={false}
                    >
                      <AssetPopup assetId={selectedAsset.asset_id} status={selectedAsset.status} />
                    </Popup>
                  )} */}
                </Map>
              </div>
            </div>

      </div>
      {/* Overflows by Company */}
      <h2 className='text-center text-2xl font-semibold mt-6 mb-4'>Overflows by Company</h2>

      {stats && (
        <>
          <div className='sm:hidden px-4 pb-8 space-y-3'>
            <MobileCompanies stats={stats} />
          </div>
          <div className='hidden sm:flex flex-col items-center px-4 pb-8'>
              <DesktopCompanies stats={stats} />
          </div>
        </>
      )}

      {/* Top Active CSOs */}
      <h2 className='text-center text-2xl font-semibold mt-6 mb-4'>Top Active CSOs</h2>

      {topDischarges && (
        <>
          <div className='sm:hidden px-4 pb-8 space-y-3'>
            <MobileCSOs topDischarges={topDischarges} />
          </div>
          <div className='hidden sm:flex flex-col items-center px-4 pb-8'>
            <DesktopCSOs topDischarges={topDischarges} />
          </div>
        </>
      )}

            <div className='shrink-0 bg-brown text-white px-16 py-8 text-center'>
            <span className='text-3xl font-bold'>Sewage Data</span>
            <p className='text-xl font-semibold'>&copy; 2026 <a href='https://deveroonie.co.uk'>Deveroonie</a>. Licensed under MIT.</p>
            <p className='text-lg font-semibold'>This website is <a href='https://github.com/Deveroonie/poo/' className='underline'>open source</a>.</p>
            <span className='text-lg'>Water Data from:<br /></span>
              <span className='text-sm'>
                <a href='https://www.streamwaterdata.co.uk/'>Stream Water Data (CC-BY-SA 4.0)</a>,
                Anglian Water Services Limited,
                Dŵr Cymru Cyf,
                Northumbrian Water Ltd,&nbsp;
                <a href='https://www.scottishwater.co.uk/help-and-resources/open-data/overflow-map-data'>Scottish Water</a>,
                Severn Trent Water Limited,
                South West Water Limited,
                Southern Water Services Limited,
                Thames Water Utilities Limited,
                United Utilities Group plc,
                Wessex Water Services Limited,
                Yorkshire Water Services Limited
              </span>
            <span className='text-lg'><br />Bathing Water Data:<br /></span>
            <span className='text-sm'>Contains Environment Agency information © Environment Agency and database right. Licensed under <a href='https://www.nationalarchives.gov.uk/doc/open-government-licence/version/3/'>Open Government Licence (OGL) v3</a></span>
            <span className='text-lg'><br />Map Data provided by:<br /></span>
            <span className='text-sm'>
              <a href='https://www.openstreetmap.org/copyright'>OpenStreetMap</a>, 
              <a href='https://openmaptiles.org/'>OpenMapTiles</a>, 
              <a href='https://maplibre.org/'>MapLibre</a>, 
              <a href='https://openfreemap.org/'>OpenFreeMap</a>
            </span>
          </div>
      </>
    )
}

export default App

