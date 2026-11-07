import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import {useQuery, useQueryClient} from '@tanstack/react-query'
import {ipc, type gedcom} from '../../ipc'
import {useJobs} from '../../queries'
import {useUI} from '../../store/ui'

/** Lance un import : choix du fichier, puis (nouveau projet | projet courant). */
export async function startGedcomImport(into: 'new' | 'current', setJob: (id: string | null) => void) {
    const path = await ipc.gedcomDialog()
    if (!path) return
    const id = into === 'new' ? await ipc.importGedcomNewProject(path) : await ipc.importGedcomHere(path)
    if (id) setJob(id)
}

const COUNTS: [keyof gedcom.Report, string][] = [
    ['individuals', 'persons'], ['families', 'families'], ['unions', 'unions'], ['parentLinks', 'parentLinks'],
    ['names', 'names'], ['events', 'events'], ['places', 'places'], ['sources', 'sources'],
    ['citations', 'citations'], ['notes', 'notes'],
]

function Report({r}: { r: gedcom.Report }) {
    const {t} = useTranslation()
    const skipped = Object.entries(r.skipped ?? {})
    return (
        <div className="gedcom-report">
            <p className="muted">
                {t('gedcom.source', {software: r.software || '?', version: r.gedcomVersion || '?', encoding: r.encoding})}
                {r.declaredEncoding && r.declaredEncoding.toUpperCase() !== r.encoding.toUpperCase() &&
                    ` ${t('gedcom.declared', {declared: r.declaredEncoding})}`}
            </p>
            <dl className="counts">
                {COUNTS.map(([k, label]) => (
                    <div key={k}><dt>{t(`gedcom.counts.${label}`)}</dt><dd>{r[k] as number}</dd></div>
                ))}
            </dl>
            {r.extraTags > 0 && <p>{t('gedcom.extras', {count: r.extraTags})}</p>}
            {r.mediaSkipped > 0 && <p className="error">{t('gedcom.media', {count: r.mediaSkipped})}</p>}
            {r.corrections?.length > 0 && (
                <details>
                    <summary>{t('gedcom.corrections', {count: r.corrections.length})}</summary>
                    <ul>{r.corrections.map((c, i) => <li key={i}>{c}</li>)}</ul>
                </details>
            )}
            {(r.warnings?.length > 0) && (
                <details open={r.warnings.length <= 5}>
                    <summary>{t('gedcom.warnings', {count: r.warnings.length + r.warningsDropped})}</summary>
                    <ul>{r.warnings.map((w, i) => <li key={i}>{w}</li>)}</ul>
                    {r.warningsDropped > 0 && <p className="muted">{t('gedcom.more', {count: r.warningsDropped})}</p>}
                </details>
            )}
            {skipped.length > 0 && (
                <p className="muted">{t('gedcom.skipped')} {skipped.map(([k, n]) => `${k} (${n})`).join(', ')}</p>
            )}
            {(!r.warnings || r.warnings.length === 0) && <p className="muted">{t('gedcom.noWarnings')}</p>}
        </div>
    )
}

/** Fenêtre de suivi d'un import GEDCOM : progression, puis rapport. */
export function ImportDialog() {
    const {t} = useTranslation()
    const id = useUI((s) => s.importJob)
    const setJob = useUI((s) => s.setImportJob)
    const jobs = useJobs()
    const qc = useQueryClient()
    const job = jobs.data?.find((j) => j.id === id)
    const done = job?.state === 'done'
    const report = useQuery({
        queryKey: ['gedcom-report', id],
        queryFn: () => ipc.gedcomReport(id!),
        enabled: !!id && done,
    })
    const [cancelling, setCancelling] = useState(false)
    if (!id) return null

    const close = () => { setJob(null); setCancelling(false); void qc.invalidateQueries() }
    const pct = job && job.total > 0 ? Math.round((job.done / job.total) * 100) : 0

    return (
        <div className="dialog-backdrop" role="presentation">
            <div className="dialog wide" role="dialog" aria-modal="true" aria-label={t('gedcom.title')}>
                <h3>{t('gedcom.title')}</h3>
                {(!job || job.state === 'running') && <>
                    <progress max={100} value={pct} aria-label={t('gedcom.progress')}/>
                    <p className="muted">{job?.message || t('gedcom.starting')} {job && job.total > 0 && `${job.done} / ${job.total}`}</p>
                    <div className="row">
                        <button disabled={cancelling} onClick={() => { setCancelling(true); void ipc.cancelJob(id) }}>{t('common.cancel')}</button>
                    </div>
                </>}
                {job?.state === 'failed' && <>
                    <p role="alert" className="error">{t('gedcom.failed', {message: job.error})}</p>
                    <p className="muted">{t('gedcom.nothingWritten')}</p>
                    <div className="row"><button onClick={close}>{t('common.close')}</button></div>
                </>}
                {job?.state === 'cancelled' && <>
                    <p>{t('gedcom.cancelled')}</p>
                    <div className="row"><button onClick={close}>{t('common.close')}</button></div>
                </>}
                {done && <>
                    {report.data ? <Report r={report.data}/> : <p className="muted">…</p>}
                    <p className="muted">{t('gedcom.undoHint')}</p>
                    <div className="row"><button className="primary" onClick={close}>{t('common.close')}</button></div>
                </>}
            </div>
        </div>
    )
}
