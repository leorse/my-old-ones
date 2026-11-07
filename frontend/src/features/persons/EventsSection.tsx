import {useEffect, useState, type FormEvent} from 'react'
import {useTranslation} from 'react-i18next'
import type {repo} from '../../ipc'
import {ipc} from '../../ipc'
import {ageText} from '../../components/helpers'
import {useAction, useDebounced, usePlaces} from '../../queries'

const PERSON_TYPES = ['birth', 'baptism', 'death', 'burial', 'cremation', 'occupation', 'residence', 'census',
    'military', 'decoration', 'emigration', 'immigration', 'naturalization', 'degree', 'religion', 'custom']

/** Saisie d'une date avec aperçu de l'interprétation faite par le backend. */
export function DateInput({value, onChange, label}: { value: string; onChange: (v: string) => void; label: string }) {
    const {t} = useTranslation()
    const debounced = useDebounced(value, 150)
    const [preview, setPreview] = useState<{ display: string; structured: boolean } | null>(null)
    useEffect(() => {
        let alive = true
        if (!debounced.trim()) {
            setPreview(null)
            return
        }
        void ipc.previewDate(debounced).then((p) => { if (alive) setPreview(p) })
        return () => { alive = false }
    }, [debounced])
    return (
        <span className="date-input">
            <input aria-label={label} placeholder={label} title={t('events.dateHint')} value={value}
                   onChange={(e) => onChange(e.target.value)}/>
            {value.trim() && preview && (
                <small className={preview.structured ? 'muted' : 'error'}>
                    {preview.structured ? `→ ${preview.display}` : t('events.dateText')}
                </small>
            )}
        </span>
    )
}

/** Choix d'un lieu existant par recherche, avec création rapide. */
export function PlacePicker({value, onChange}: { value: string; onChange: (id: string, name: string) => void }) {
    const {t} = useTranslation()
    const [text, setText] = useState('')
    const q = useDebounced(text, 150)
    const places = usePlaces(q)
    const create = useAction(async (name: string) => {
        const id = await ipc.createPlace({name, type: 'commune', parentId: '', codeInsee: '', validFrom: '', validTo: ''})
        onChange(id, name)
        setText(name)
    })
    const exact = places.data?.some((p) => p.displayName.toLowerCase() === text.trim().toLowerCase())
    return (
        <span className="place-picker">
            <input aria-label={t('events.place')} placeholder={t('events.place')} list="place-options" value={text}
                   onChange={(e) => {
                       setText(e.target.value)
                       const hit = places.data?.find((p) => p.displayName === e.target.value)
                       onChange(hit?.id ?? '', hit?.displayName ?? '')
                   }}/>
            <datalist id="place-options">
                {places.data?.slice(0, 20).map((p) => <option key={p.id} value={p.displayName}/>)}
            </datalist>
            {text.trim() && !exact && !value && (
                <button type="button" onClick={() => create.mutate([text.trim()])}>
                    {t('events.newPlace', {name: text.trim()})}
                </button>
            )}
        </span>
    )
}

export function EventsSection({detail}: { detail: repo.PersonDetail }) {
    const {t} = useTranslation()
    const personId = detail.core.id
    const [open, setOpen] = useState(false)
    const [type, setType] = useState('birth')
    const [date, setDate] = useState('')
    const [placeId, setPlaceId] = useState('')
    const [description, setDescription] = useState('')
    const add = useAction(ipc.addEvent)
    const del = useAction(ipc.deleteEvent)

    const submit = (e: FormEvent) => {
        e.preventDefault()
        add.mutate([{type, personId, unionId: '', date, placeId, description, notesMd: ''}], {
            onSuccess: () => { setDate(''); setPlaceId(''); setDescription(''); setOpen(false) },
        })
    }
    const ageOf = (id: string) => detail.ages.find((a) => a.eventId === id)

    return (
        <section aria-labelledby="sec-events">
            <h3 id="sec-events">{t('person.events')}</h3>
            {detail.events.length === 0 && <p className="muted">{t('events.empty')}</p>}
            <ul className="events">
                {detail.events.map((e) => {
                    const age = ageOf(e.id)
                    return (
                        <li key={e.id}>
                            <strong>{t(`events.types.${e.type}`)}</strong>
                            {e.dateDisplay && <span> · {e.dateDisplay}</span>}
                            {e.placeName && <span> · {e.placeName}</span>}
                            {e.description && <span> · {e.description}</span>}
                            {age && <small className="muted"> ({t('person.atAge', {age: ageText(t, age)})})</small>}
                            <button className="icon" aria-label={t('events.delete')} onClick={() => del.mutate([e.id])}>✕</button>
                        </li>
                    )
                })}
            </ul>
            {detail.ageAtDeath && <p className="muted">{t('person.ageAtDeath', {age: ageText(t, detail.ageAtDeath)})}</p>}
            {open ? (
                <form className="row" onSubmit={submit}>
                    <select aria-label="Type" value={type} onChange={(e) => setType(e.target.value)}>
                        {PERSON_TYPES.map((x) => <option key={x} value={x}>{t(`events.types.${x}`)}</option>)}
                    </select>
                    <DateInput label={t('events.date')} value={date} onChange={setDate}/>
                    <PlacePicker value={placeId} onChange={(id) => setPlaceId(id)}/>
                    <input aria-label={t('events.description')} placeholder={t('events.description')}
                           value={description} onChange={(e) => setDescription(e.target.value)}/>
                    <button className="primary" type="submit">{t('common.add')}</button>
                    <button type="button" onClick={() => setOpen(false)}>{t('common.cancel')}</button>
                </form>
            ) : <button onClick={() => setOpen(true)}>{t('events.add')}</button>}
        </section>
    )
}
