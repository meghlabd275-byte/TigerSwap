'use client'

import { useState, useEffect } from 'react'

interface LicenseErrorProps {
  error?: string
  errorCode?: string
}

export default function LicenseRequired({ 
  error = "Please input authorized API keys. Contact TigerSwap admin.",
  errorCode = "LICENSE_REQUIRED"
}: LicenseErrorProps) {
  const [licenseKey, setLicenseKey] = useState('')
  const [licenseSecret, setLicenseSecret] = useState('')
  const [loading, setLoading] = useState(false)
  const [errorMsg, setErrorMsg] = useState(error)

  const validateLicense = async () => {
    if (!licenseKey || !licenseSecret) {
      setErrorMsg("Please enter both license key and secret")
      return
    }

    setLoading(true)
    setErrorMsg("")

    try {
      const response = await fetch('/api/license/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          license_key: licenseKey,
          license_secret: licenseSecret,
          client_id: licenseKey.split('-')[1] || ''
        })
      })

      const data = await response.json()

      if (response.ok && data.valid) {
        // Store license and reload
        localStorage.setItem('wl_license_key', licenseKey)
        localStorage.setItem('wl_license_valid', 'true')
        window.location.reload()
      } else {
        setErrorMsg(data.error || "Invalid license. Contact TigerSwap admin.")
      }
    } catch (err) {
      setErrorMsg("Failed to validate. Contact TigerSwap admin.")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-slate-900 flex items-center justify-center p-4">
      <div className="max-w-md w-full bg-slate-800 rounded-2xl p-8 border border-red-500/30">
        <div className="text-center mb-8">
          <div className="w-20 h-20 mx-auto mb-4 bg-red-500/20 rounded-full flex items-center justify-center">
            <svg className="w-10 h-10 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-red-500 mb-2">License Required</h1>
          <p className="text-slate-400">{errorMsg}</p>
          {errorCode && (
            <p className="text-slate-500 text-sm mt-2">Error Code: {errorCode}</p>
          )}
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-slate-400 mb-2 text-sm">License Key</label>
            <input
              type="text"
              value={licenseKey}
              onChange={(e) => setLicenseKey(e.target.value)}
              placeholder="WL-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
              className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-3 text-white placeholder-slate-500 focus:outline-none focus:border-red-500"
            />
          </div>

          <div>
            <label className="block text-slate-400 mb-2 text-sm">License Secret</label>
            <input
              type="password"
              value={licenseSecret}
              onChange={(e) => setLicenseSecret(e.target.value)}
              placeholder="Enter license secret"
              className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-3 text-white placeholder-slate-500 focus:outline-none focus:border-red-500"
            />
          </div>

          <button
            onClick={validateLicense}
            disabled={loading}
            className="w-full bg-orange-500 hover:bg-orange-600 disabled:bg-slate-700 text-white font-semibold py-3 rounded-lg transition-colors"
          >
            {loading ? 'Validating...' : 'Validate License'}
          </button>
        </div>

        <div className="mt-6 pt-6 border-t border-slate-700 text-center">
          <p className="text-slate-500 text-sm">
            Don't have a license?
          </p>
          <a 
            href="https://tigerswap.io/white-label" 
            className="text-orange-500 hover:text-orange-400 text-sm"
          >
            Contact TigerSwap Admin →
          </a>
        </div>
      </div>
    </div>
  )
}