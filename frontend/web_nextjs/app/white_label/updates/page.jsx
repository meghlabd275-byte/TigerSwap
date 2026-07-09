'use client';
import { useState, useEffect } from 'react';
import { ThemeToggle } from '../../components/ThemeToggle';
export default function WhiteLabelUpdates() {
    const [updates, setUpdates] = useState([]);
    const [version, setVersion] = useState(null);
    const [loading, setLoading] = useState(true);
    const [applying, setApplying] = useState(null);
    useEffect(() => {
        checkUpdates();
    }, []);
    const checkUpdates = async () => {
        try {
            const response = await fetch('/api/sync/check-updates');
            const data = await response.json();
            setVersion({
                current_version: data.current_version,
                latest_version: data.latest_version,
                update_available: data.update_available,
            });
            setUpdates(data.updates || []);
        }
        catch (error) {
            console.error('Failed to check updates:', error);
        }
        finally {
            setLoading(false);
        }
    };
    const applyUpdate = async (updateId) => {
        setApplying(updateId);
        try {
            const response = await fetch('/api/sync/apply-update', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ update_id: updateId }),
            });
            const data = await response.json();
            if (response.ok) {
                alert(`Update applied! New version: ${data.version}`);
                checkUpdates();
            }
            else {
                alert(data.error || 'Failed to apply update');
            }
        }
        catch (error) {
            alert('Failed to apply update');
        }
        finally {
            setApplying(null);
        }
    };
    const getUpdateTypeColor = (type) => {
        switch (type) {
            case 'feature_add': return 'bg-green-500';
            case 'feature_update': return 'bg-blue-500';
            case 'security_fix': return 'bg-yellow-500';
            case 'breaking_change': return 'bg-red-500';
            default: return 'bg-gray-500';
        }
    };
    return (<div className="min-h-screen">
      <div className="flex justify-between items-center p-4 border-b border-white/10">
        <div className="text-xl font-bold">🐯 TigerSwap Updates</div>
        <ThemeToggle />
      </div>

      <main className="p-8 max-w-5xl mx-auto">
        <h1 className="text-3xl font-bold mb-8">Feature Updates</h1>

        {loading ? (<div className="text-center py-12">Loading...</div>) : version && (<>
            <div className="card mb-8">
              <div className="flex justify-between items-center">
                <div>
                  <p className="text-slate-400">Current Version</p>
                  <p className="text-2xl font-bold">{version.current_version}</p>
                </div>
                <div className="text-right">
                  <p className="text-slate-400">Latest Version</p>
                  <p className="text-2xl font-bold text-orange-500">{version.latest_version}</p>
                </div>
                {version.update_available && (<div className="bg-orange-500/20 px-4 py-2 rounded-lg">
                    <span className="text-orange-500 font-semibold">Update Available!</span>
                  </div>)}
              </div>
            </div>

            {updates.length === 0 ? (<div className="text-center py-12 text-slate-400">
                <p className="text-xl mb-2">All updates applied</p>
                <p>You are running the latest version</p>
              </div>) : (<div className="space-y-4">
                {updates.map((update) => (<div key={update.update_id} className="card">
                    <div className="flex justify-between items-start mb-4">
                      <div>
                        <div className="flex items-center gap-3 mb-2">
                          <h3 className="text-xl font-bold">{update.title}</h3>
                          <span className={`px-2 py-1 rounded text-xs text-white ${getUpdateTypeColor(update.update_type)}`}>
                            {update.update_type}
                          </span>
                          <span className="text-slate-400 text-sm">v{update.update_version}</span>
                        </div>
                        <p className="text-slate-400">{update.description}</p>
                      </div>
                      {update.status === 'available' && (<button onClick={() => applyUpdate(update.update_id)} disabled={applying === update.update_id} className="btn-primary">
                          {applying === update.update_id ? 'Applying...' : 'Apply Update'}
                        </button>)}
                    </div>
                    <p className="text-slate-500 text-sm">
                      Available: {new Date(update.available_at).toLocaleString()}
                    </p>
                  </div>))}
              </div>)}
          </>)}
      </main>
    </div>);
}
//# sourceMappingURL=page.jsx.map