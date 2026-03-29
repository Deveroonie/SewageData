export default function MobileCompanies({ stats }) {
    return (
        <div>
            {stats.companies.map((company, i) => (
                <div key={i} className='border border-gray-200 rounded-lg p-3 bg-white'>
                  <p className='font-semibold text-sm mb-2'>{company.company}</p>
                  <div className='grid grid-cols-2 gap-1 text-sm'>
                    <span className='text-gray-500'>Total CSOs</span><span className='text-right tabular-nums'>{company.total_assets}</span>
                    <span className='text-gray-500'>Active</span><span className='text-right tabular-nums' style={{color: company.total_discharging > 0 ? 'red' : 'inherit', fontWeight: company.total_discharging > 0 ? '600' : 'normal'}}>{company.total_discharging}</span>
                    <span className='text-gray-500'>Active %</span><span className='text-right tabular-nums'>{company.percent_active}%</span>
                    <span className='text-gray-500'>Offline</span><span className='text-right tabular-nums text-gray-500'>{company.company === "Dwr Cymru Welsh Water" ? "-" : company.total_offline}</span>
                  </div>
                </div>
            ))}
            {(() => {
              const totalAssets = stats.companies.reduce((s, c) => s + c.total_assets, 0)
              const totalDischarging = stats.companies.reduce((s, c) => s + c.total_discharging, 0)
              const totalOffline = stats.companies.reduce((s, c) => s + c.total_offline, 0)
              const pct = totalAssets > 0 ? ((totalDischarging / totalAssets) * 100).toFixed(1) : '0.0'
              return (
                <div className='border-2 border-gray-400 rounded-lg p-3 bg-gray-100 font-semibold'>
                  <p className='text-sm mb-2'>Total</p>
                  <div className='grid grid-cols-2 gap-1 text-sm'>
                    <span className='text-gray-500'>Total CSOs</span><span className='text-right tabular-nums'>{totalAssets}</span>
                    <span className='text-gray-500'>Active</span><span className='text-right tabular-nums' style={{color: totalDischarging > 0 ? 'red' : 'inherit', fontWeight: totalDischarging > 0 ? '600' : 'normal'}}>{totalDischarging}</span>
                    <span className='text-gray-500'>Active %</span><span className='text-right tabular-nums'>{pct}%</span>
                    <span className='text-gray-500'>Offline</span><span className='text-right tabular-nums text-gray-500'>{totalOffline - (stats.companies.find(c => c.company === "Dwr Cymru Welsh Water")?.total_offline || 0)}</span>
                  </div>
                </div>
              )
            })()}
        </div>
    )
}