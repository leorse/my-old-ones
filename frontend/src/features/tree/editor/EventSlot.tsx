import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import {ipc, type repo} from '../../../ipc'
import {useAction} from '../../../queries'
import {DateInput, PlacePicker} from '../../persons/EventsSection'

const stripParens = (s: string) => s.replace(/\s*\([^)]*\)\s*$/, '')

/** Texte de date à afficher dans un champ : celui de l'utilisateur, ou l'affichage sans l'équivalent grégorien. */
export function dateFieldText(e?: repo.EventDTO): string {
    if (!e) return ''
    return e.date.precision === 'text' ? e.date.originalText : stripParens(e.dateDisplay)
}

interface Props {
    /** Texte du libellé (« Naissance »…). */
    title: string
    dateLabel: string
    placeLabel?: string // absent : pas de champ lieu (divorce)
    type: string
    personId?: string
    unionId?: string
    events: repo.EventDTO[]
    /** Appelé après un enregistrement réussi (ex. marquer l'union comme terminée). */
    afterSave?: (date: string) => Promise<unknown> | void
}

/**
 * Date + lieu d'un événement (naissance, décès, mariage, divorce…) modifiables sur place.
 * Enregistrement automatique quand on quitte le groupe de champs ; créé au premier remplissage,
 * supprimé quand on vide tout.
 */
export function EventSlot(props: Props) {
    const ev = props.events.find((e) => e.type === props.type)
    // clé = état enregistré : après un enregistrement, annulation ou changement de personne, les champs repartent du bon état
    const sig = ev ? `${ev.id}|${ev.dateDisplay}|${ev.placeId}|${ev.description}` : 'none'
    return <SlotFields key={sig} {...props} ev={ev}/>
}

function SlotFields({title, dateLabel, placeLabel, type, personId, unionId, ev, afterSave}: Props & { ev?: repo.EventDTO }) {
    const {t} = useTranslation()
    const initDate = dateFieldText(ev)
    const initPlace = ev?.placeName ?? ''
    const [date, setDate] = useState(initDate)
    const [placeText, setPlaceText] = useState(initPlace)
    const [placeId, setPlaceId] = useState(ev?.placeId ?? '')

    const save = useAction(async (d: string, p: string) => {
        if (ev) {
            if (!d.trim() && !p && !ev.description && !ev.notesMd) await ipc.deleteEvent(ev.id)
            else await ipc.updateEvent(ev.id, {date: d, placeId: p})
        } else if (d.trim() || p) {
            await ipc.addEvent({type, personId: personId ?? '', unionId: unionId ?? '', date: d, placeId: p, description: '', notesMd: ''})
        }
        await afterSave?.(d)
    })

    const unresolvedPlace = placeLabel !== undefined && placeText.trim() !== '' && placeText !== initPlace && !placeId
    const dirty = date !== initDate || placeText !== initPlace

    const commit = () => {
        if (!dirty || unresolvedPlace || save.isPending) return
        const p = placeLabel === undefined ? (ev?.placeId ?? '') : placeText.trim() === '' ? '' : placeId
        save.mutate([date, p])
    }

    return (
        <fieldset className="slot" onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) commit() }}
                  onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); commit() } }}>
            <legend>{title}</legend>
            <DateInput label={dateLabel} value={date} onChange={setDate}/>
            {placeLabel !== undefined && (
                <PlacePicker label={placeLabel} value={placeId} initialText={initPlace}
                             onChange={(id, _name, text) => { setPlaceId(id); setPlaceText(text) }}/>
            )}
            {unresolvedPlace && <small className="error">{t('editor.placeUnknown')}</small>}
        </fieldset>
    )
}
