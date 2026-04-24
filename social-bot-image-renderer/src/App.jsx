import Map, { Marker } from "react-map-gl/maplibre"
import 'maplibre-gl/dist/maplibre-gl.css'

function App() {
  const params = new URLSearchParams(window.location.search)
  const latitude = parseFloat(params.get("latitude"))
  const longitude = parseFloat(params.get("longitude"))
  const locationCSO = params.get("locationcso")
  const bathingwater = params.get("bathingwater")
  const bwdist = params.get("bwdist")
  const bwrating = params.get("bwrating")
  const watercompany = params.get("watercompany")
  const count30d = params.get("count30d")
  const start = params.get("start")
  const csoid = params.get("csoid")

  return (
    <div className="flex flex-row w-[1200px] h-[675px]">
      <div className="flex flex-col basis-2/5 min-w-0 bg-brown-700 text-white p-8">
        <p className="monserrat text-2xl font-black">
          {locationCSO}
          <br />
          <br />
        </p>

        <p className="text-lg">
          <span className="font-bold monserrat">{bwdist} miles from {bathingwater}</span>
          <br />
          {bathingwater} was rated <span className="font-black">{bwrating}</span> by the Environment Agency.
          <br />
          <br />
        </p>

        <p className="text-lg">
          <span className="font-bold monserrat">{watercompany} is responsible for this discharge.</span>
          <br />
          They have released untreated sewage from this sewage pipe <span className="font-bold">{count30d}</span> times this month.
          <br />
          <br />
          This discharge started at {start}.
        </p>

        <p className="mt-auto">https://sewagedata.co.uk/?cso={csoid}<br />Bluesky: @alerts.sewagedata.co.uk<br />Mastodon: @sewagedata_alerts@mastodon.social<br />{new Date().getDate().toString().padStart(2, "0")}/{(new Date().getMonth()+1).toString().padStart(2, "0")}/{new Date().getFullYear()} {new Date().getHours().toString().padStart(2, "0")}:{new Date().getMinutes().toString().padStart(2, "0")}</p>
      </div>

      <div className="basis-3/5 min-w-0">
        <Map
          onIdle={() => { window.__mapReady = true }}
          initialViewState={{ latitude, longitude, zoom: 7 }}
          style={{ width: "100%", height: "100%" }}
          mapStyle="https://tiles.openfreemap.org/styles/liberty"
        >
          <Marker latitude={latitude} longitude={longitude} />
        </Map>
      </div>
    </div>
  )
}

export default App