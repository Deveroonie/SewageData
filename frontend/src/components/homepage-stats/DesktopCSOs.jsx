import minutesAgo from "../../util/minutesAgo";

export default function DesktopCSOs({ topDischarges }) {
    return (
        <table className='w-full max-w-4xl text-sm border-collapse'>
            <thead>
              <tr className='bg-gray-100 text-gray-700 uppercase text-xs tracking-wide'>
                <th className='text-left lg:px-4 py-2 border border-gray-200'>Company</th>
                <th className='text-right lg:px-4 py-2 border border-gray-200'>Receiving Watercourse</th>
                <th className='text-right lg:px-4 py-2 border border-gray-200'>Discharge Started</th>
                <th className='text-right lg:px-4 py-2 border border-gray-200 hidden sm:table-cell'>Duration (Hours)</th>
                <th className='text-right lg:px-4 py-2 border border-gray-200'>Duration (Days)</th>
              </tr>
            </thead>
            <tbody>
              {topDischarges.map((discharge, i) => (
                <tr key={i} className={i % 2 === 0 ? 'bg-white' : 'bg-gray-50'}>
                  <td className='px-4 py-2 border border-gray-200 font-medium'>{discharge.company}</td>
                  <td className='px-4 py-2 border border-gray-200 font-medium'>{discharge.receiving_watercourse}</td>
                  <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>{new Date(discharge.discharge_start).toLocaleDateString()}</td>
                  <td className='px-4 py-2 border border-gray-200 text-right tabular-nums hidden sm:table-cell'>{(minutesAgo(new Date(discharge.discharge_start))/60).toFixed(2)}</td>
                  <td className='px-4 py-2 border border-gray-200 text-right tabular-nums'>{(minutesAgo(new Date(discharge.discharge_start))/1440).toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
        </table>
    )
}