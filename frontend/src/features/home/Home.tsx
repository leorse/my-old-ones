import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import {useQueryClient} from '@tanstack/react-query'
import {ipc} from '../../ipc'
import {useRecents} from '../../queries'
import {useUI} from '../../store/ui'
import {startGedcomImport} from '../gedcom/ImportDialog'

export function Home() {
    const {t} = useTranslation()
    const qc = useQueryClient()
    const recents = useRecents()
    const [error, setError] = useState<string | null>(null)
    const [busy, setBusy] = useState(false)
    const setJob = useUI((s) => s.setImportJob)

    const run = async (fn: () => Promise<unknown>) => {
        setError(null)
        try {
            await fn()
            await qc.invalidateQueries()
        } catch (e) {
            setError(String((e as Error)?.message ?? e))
        }
    }

    return (
        <div className="home">
            <h1>{t('app.name')}</h1>
            <p className="muted">{t('app.tagline')}</p>
            <div className="row">
                <button className="primary" onClick={() => void run(ipc.newProjectDialog)}>{t('home.new')}</button>
                <button onClick={() => void run(ipc.openProjectDialog)}>{t('home.open')}</button>
                <button onClick={() => void run(() => startGedcomImport('new', setJob))}>{t('home.gedcom')}</button>
                <button disabled={busy} onClick={() => { setBusy(true); void run(ipc.sampleDialog).finally(() => setBusy(false)) }}>
                    {busy ? t('home.sampleBusy') : t('home.sample')}
                </button>
            </div>
            {error && <p role="alert" className="error">{t('error', {message: error})}</p>}
            <h2>{t('home.recent')}</h2>
            {recents.data?.length ? (
                <ul className="recents">
                    {recents.data.map((r) => (
                        <li key={r.path}>
                            <button onClick={() => void run(() => ipc.openProject(r.path))}>
                                <strong>{r.name}</strong><small>{r.path}</small>
                            </button>
                        </li>
                    ))}
                </ul>
            ) : <p className="muted">{t('home.noRecent')}</p>}
        </div>
    )
}
