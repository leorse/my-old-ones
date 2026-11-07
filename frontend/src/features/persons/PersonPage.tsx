import {useEffect, useState} from 'react'
import {useTranslation} from 'react-i18next'
import {ipc, type repo} from '../../ipc'
import {personLabel} from '../../components/helpers'
import {useAction, usePerson} from '../../queries'
import {useUI} from '../../store/ui'
import {EventsSection} from './EventsSection'
import {FamilySection} from './FamilySection'

const NAME_TYPES = ['birth', 'usage', 'marital', 'religious', 'nickname', 'other'] as const

function IdentitySection({detail}: { detail: repo.PersonDetail }) {
    const {t} = useTranslation()
    const c = detail.core
    const update = useAction(ipc.updatePerson)
    const setPrimary = useAction(ipc.setPrimaryName)
    const delName = useAction(ipc.deleteName)
    const addName = useAction(ipc.addName)
    const setRoot = useAction(ipc.setRoot)
    const [adding, setAdding] = useState(false)
    const [surname, setSurname] = useState('')
    const [given, setGiven] = useState('')
    const [type, setType] = useState<string>('marital')

    return (
        <section aria-labelledby="sec-identity">
            <h3 id="sec-identity">{t('person.identity')}</h3>
            <ul>
                {c.names.map((n) => (
                    <li key={n.id}>
                        <strong>{personLabel(n, t('common.unknown'))}</strong>
                        <small className="muted"> · {t(`person.nameTypes.${n.type}`)}</small>
                        {n.isPrimary
                            ? <small className="badge">{t('person.primaryName')}</small>
                            : <>
                                <button className="icon" title={t('person.makePrimary')} aria-label={t('person.makePrimary')} onClick={() => setPrimary.mutate([n.id])}>★</button>
                                <button className="icon" aria-label={t('person.deleteName')} onClick={() => delName.mutate([n.id])}>✕</button>
                            </>}
                    </li>
                ))}
            </ul>
            {adding ? (
                <form className="row" onSubmit={(e) => {
                    e.preventDefault()
                    addName.mutate([c.id, {type, surname: surname.trim(), givenNames: given.split(/\s+/).filter(Boolean),
                        usualGiven: 0, prefix: '', suffix: ''}], {onSuccess: () => { setAdding(false); setSurname(''); setGiven('') }})
                }}>
                    <select aria-label="Type" value={type} onChange={(e) => setType(e.target.value)}>
                        {NAME_TYPES.map((x) => <option key={x} value={x}>{t(`person.nameTypes.${x}`)}</option>)}
                    </select>
                    <input aria-label={t('persons.surname')} placeholder={t('persons.surname')} value={surname} onChange={(e) => setSurname(e.target.value)}/>
                    <input aria-label={t('persons.givenNames')} placeholder={t('persons.givenNames')} value={given} onChange={(e) => setGiven(e.target.value)}/>
                    <button className="primary" type="submit">{t('common.add')}</button>
                    <button type="button" onClick={() => setAdding(false)}>{t('common.cancel')}</button>
                </form>
            ) : <button onClick={() => setAdding(true)}>{t('person.addName')}</button>}

            <div className="row">
                <label>{t('persons.sex')}{' '}
                    <select value={c.sex} onChange={(e) => update.mutate([c.id, {sex: e.target.value}])}>
                        {(['M', 'F', 'other', 'unknown'] as const).map((s) => <option key={s} value={s}>{t(`persons.sexes.${s}`)}</option>)}
                    </select>
                </label>
                <select aria-label="Statut" value={c.living} onChange={(e) => update.mutate([c.id, {living: e.target.value}])}>
                    {(['living', 'deceased', 'unknown'] as const).map((s) => <option key={s} value={s}>{t(`persons.living.${s}`)}</option>)}
                </select>
                <label><input type="checkbox" checked={c.isFavorite}
                              onChange={(e) => update.mutate([c.id, {isFavorite: e.target.checked}])}/> {t('person.favorite')}</label>
                {detail.isRoot
                    ? <span className="badge">{t('person.root')}</span>
                    : <button onClick={() => setRoot.mutate([c.id])}>{t('person.setRoot')}</button>}
            </div>
        </section>
    )
}

function NotesSection({detail}: { detail: repo.PersonDetail }) {
    const {t} = useTranslation()
    const update = useAction(ipc.updatePerson)
    const [text, setText] = useState(detail.core.notesMd)
    // resynchronise après un annuler/rétablir ou un changement de fiche
    useEffect(() => setText(detail.core.notesMd), [detail.core.id, detail.core.notesMd])
    return (
        <section aria-labelledby="sec-notes">
            <h3 id="sec-notes">{t('person.notes')}</h3>
            <textarea rows={6} placeholder={t('person.notesPlaceholder')} value={text}
                      onChange={(e) => setText(e.target.value)}
                      onBlur={() => { if (text !== detail.core.notesMd) update.mutate([detail.core.id, {notesMd: text}]) }}/>
        </section>
    )
}

function DeleteDialog({detail, onClose}: { detail: repo.PersonDetail; onClose: () => void }) {
    const {t} = useTranslation()
    const select = useUI((s) => s.selectPerson)
    const [mode, setMode] = useState<'only' | 'descendants' | 'ancestors'>('only')
    const [impact, setImpact] = useState<repo.Impact | null>(null)
    const del = useAction(ipc.deletePeople)
    const primary = detail.core.names.find((n) => n.isPrimary) ?? detail.core.names[0]
    const name = primary ? personLabel(primary, t('common.unknown')) : t('common.unknown')

    useEffect(() => {
        let alive = true
        void ipc.deleteImpact(detail.core.id, mode).then((i) => { if (alive) setImpact(i) })
        return () => { alive = false }
    }, [detail.core.id, mode])

    return (
        <div className="dialog-backdrop" role="presentation" onClick={onClose}>
            <div className="dialog" role="dialog" aria-modal="true" aria-label={t('delete.title', {name})}
                 onClick={(e) => e.stopPropagation()}>
                <h3>{t('delete.title', {name})}</h3>
                {(['only', 'descendants', 'ancestors'] as const).map((m) => (
                    <label key={m} className="block">
                        <input type="radio" name="mode" checked={mode === m} onChange={() => setMode(m)}/> {t(`delete.${m}`)}
                    </label>
                ))}
                {impact && <>
                    <p>{t('delete.impact', {people: impact.people.length, events: impact.events, links: impact.parentLinks,
                        unions: impact.unions, citations: impact.citations, media: impact.mediaLinks})}</p>
                    {mode !== 'only' && impact.people.length > 1 &&
                        <p className="muted">{impact.people.map((p) => personLabel(p, t('common.unknown'))).join(', ')}</p>}
                    {impact.becomingIsolated.length > 0 &&
                        <p className="error">{t('delete.isolated', {names: impact.becomingIsolated.map((p) => personLabel(p, t('common.unknown'))).join(', ')})}</p>}
                </>}
                <p className="muted">{t('delete.undoHint')}</p>
                <div className="row">
                    <button className="danger" onClick={() => del.mutate([detail.core.id, mode], {onSuccess: () => { select(null); onClose() }})}>
                        {t('common.delete')}
                    </button>
                    <button onClick={onClose}>{t('common.cancel')}</button>
                </div>
            </div>
        </div>
    )
}

export function PersonPage({id}: { id: string }) {
    const {t} = useTranslation()
    const q = usePerson(id)
    const [deleting, setDeleting] = useState(false)
    const d = q.data
    if (q.isError) return <p role="alert" className="error">{String(q.error)}</p>
    if (!d) return null
    const primary = d.core.names.find((n) => n.isPrimary) ?? d.core.names[0]
    return (
        <article className="person-page">
            <header className="person-head">
                <h2>{primary ? personLabel(primary, t('common.unknown')) : t('common.unknown')}</h2>
                {d.sosa.length > 0 && <span className="badge">{t('person.sosa', {n: d.sosa.join(', ')})}</span>}
                <span className="spacer"/>
                <button className="danger" onClick={() => setDeleting(true)}>{t('common.delete')}</button>
            </header>
            <IdentitySection detail={d}/>
            <EventsSection detail={d}/>
            <FamilySection detail={d}/>
            <NotesSection detail={d}/>
            {deleting && <DeleteDialog detail={d} onClose={() => setDeleting(false)}/>}
        </article>
    )
}
