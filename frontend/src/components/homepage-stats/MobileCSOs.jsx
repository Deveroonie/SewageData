import minutesAgo from "../../util/minutesAgo";

export default function MobileCSOs({ topDischarges }) {
    return (
        <>
            {topDischarges.map((discharge, i) => (
                <div key={i} className='border border-gray-200 rounded-lg p-3 bg-white'>
                  <p className='font-semibold text-sm mb-1'>{discharge.company}</p>
                  <p className='text-sm text-gray-600 mb-2'>{discharge.receiving_watercourse}</p>
                  <div className='grid grid-cols-2 gap-1 text-sm'>
                    <span className='text-gray-500'>Started</span><span className='text-right tabular-nums'>{new Date(discharge.discharge_start).toLocaleDateString()}</span>
                    <span className='text-gray-500'>Duration</span><span className='text-right tabular-nums'>{(minutesAgo(new Date(discharge.discharge_start))/1440).toFixed(2)} days</span>
                  </div>
                </div>
            ))}
        </>
    )
}
