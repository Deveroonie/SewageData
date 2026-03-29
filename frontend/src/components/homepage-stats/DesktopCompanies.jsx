export default function DesktopCompanies({ stats }) {
    return (
        <table className='w-full max-w-4xl text-sm border-collapse'>
            <thead>
              <tr className='bg-gray-100 text-gray-700 uppercase text-xs tracking-wide'>
                <th className='text-left px-4 py-2 border border-gray-200'>Company</th>
                <th className='text-right px-4 py-2 border border-gray-200'>Total CSOs</th>
                <th className='text-right px-4 py-2 border border-gray-200'>Active</th>
                <th className='text-right px-4 py-2 border border-gray-200'>Active %</th>
                <th className='text-right px-4 py-2 border border-gray-200'>Offline</th>
              </tr>
            </thead>
            <tbody>
              {stats.companies.map((company, i) => (
                <tr key={i} className={i % 2 === 0 ? 'bg-white' : 'bg-gray-50'}>
                  <td className='px-4 py-2 border border-gray-200 font-medium'>{company.company}</td>
                  <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>{company.total_assets}</td>
                  <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>
                    <span style={{ color: company.total_discharging > 0 ? 'red' : 'inherit', fontWeight: company.total_discharging > 0 ? '600' : 'normal' }}>
                      {company.total_discharging}
                    </span>
                  </td>
                  <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>{company.percent_active}%</td>
                  <td className='px-4 py-2 border border-gray-200 text-right tabular-nums text-gray-500'>{company.company == "Dwr Cymru Welsh Water" ? "-" : company.total_offline}</td>
                </tr>
              ))}
              {(() => {
                const totalAssets = stats.companies.reduce((s, c) => s + c.total_assets, 0)
                const totalDischarging = stats.companies.reduce((s, c) => s + c.total_discharging, 0)
                const totalOffline = stats.companies.reduce((s, c) => s + c.total_offline, 0)
                const pct = totalAssets > 0 ? ((totalDischarging / totalAssets) * 100).toFixed(1) : '0.0'
                  return (
                  <tr className='bg-gray-100 font-semibold border-t-2 border-gray-400'>
                    <td className='px-4 py-2 border border-gray-200'>Total</td>
                    <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>{totalAssets}</td>
                    <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>
                    <span style={{ color: totalDischarging > 0 ? 'red' : 'inherit', fontWeight: totalDischarging > 0 ? '600' : 'normal' }}>
                      {totalDischarging}
                    </span>
                    </td>
                    <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>{pct}%</td>
                    <td className='px-4 py-2 border border-gray-200 text-right tabular-nums text-gray-500'>{totalOffline - (stats.companies.find(c => c.company === "Dwr Cymru Welsh Water")?.total_offline || 0)}</td>
                  </tr>
                  )
              })()}
            </tbody>
        </table>
    )
}
