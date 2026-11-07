import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import {ipc, type repo} from '../../../ipc'
import {personLabel} from '../../../components/helpers'
import {useAction} from '../../../queries'
import {QuickAdd} from '../../persons/FamilySection'
import {EventSlot} from './EventSlot'

function NameFields({detail}: { detail: repo.PersonDetail }) {
    const {t} = useTranslation()
    const primary = detail.core.names.find((n) => n.isPrimary) ?? detail.core.names[0]
    const [surname, setSurname] = useState(primary?.surname ?? '')
    const [given, setGiven] = useState((primary?.givenNames ?? []).join(' '))
    const saveName = useAction(ipc.updateName)
    const savePerson = useAction(ipc.updatePerson)

    const dirty = !!primary && (surname !== primary.surname || given !== primary.givenNames.join(' '))
    const commit = () => {
        if (!dirty || !primary) return
        saveName.mutate([primary.id, {
            type: primary.type, surname: surname.trim(), givenNames: given.split(/\s+/).filter(Boolean),
            usualGiven: primary.usualGiven, prefix: primary.prefix, suffix: primary.suffix,
        }])
    }
    return (
        <fieldset className="slot" onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) commit() }}
                  onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); commit() } }}>
            <legend>{t('person.identity')}</legend>
            <input aria-label={t('persons.surname')} placeholder={t('persons.surname')} value={surname}
                   onChange={(e) => setSurname(e.target.value)}/>
            <input aria-label={t('persons.givenNames')} placeholder={t('persons.givenNames')} value={given}
                   onChange={(e) => setGiven(e.target.value)}/>
            <select aria-label={t('persons.sex')} value={detail.core.sex}
                    onChange={(e) => savePerson.mutate([detail.core.id, {sex: e.target.value}])}>
                {(['M', 'F', 'other', 'unknown'] as const).map((s) => <option key={s} value={s}>{t(`persons.sexes.${s}`)}</option>)}
            </select>
        </fieldset>
    )
}

/** Premier onglet : identité, naissance, décès, et pour chaque union : mariage et divorce. */
export function BasicInfoTab({detail}: { detail: repo.PersonDetail }) {
    const {t} = useTranslation()
    const id = detail.core.id
    const markEnded = useAction((unionId: string) => ipc.updateUnion(unionId, {status: 'ended'}))
    const [addingSpouse, setAddingSpouse] = useState<'spouse' | 'partner' | null>(null)
    const primary = detail.core.names.find((n) => n.isPrimary) ?? detail.core.names[0]

    return (
        <div className="editor-grid">
            <NameFields key={`${id}|${primary?.surname}|${primary?.givenNames.join(' ')}`} detail={detail}/>
            <EventSlot title={t('events.types.birth')} dateLabel={t('editor.birthDate')} placeLabel={t('editor.birthPlace')}
                       type="birth" personId={id} events={detail.events}/>
            <EventSlot title={t('events.types.death')} dateLabel={t('editor.deathDate')} placeLabel={t('editor.deathPlace')}
                       type="death" personId={id} events={detail.events}/>
            {detail.family.unions.map((u) => (
                <div key={u.id} className="union-edit">
                    <div className="union-title">
                        {t(`family.unionTypes.${u.type}`)} · {u.partners.map((p) => personLabel(p, t('common.unknown'))).join(', ') || t('common.unknown')}
                    </div>
                    <EventSlot title={t('events.types.marriage')} dateLabel={t('editor.marriageDate')} placeLabel={t('editor.marriagePlace')}
                               type="marriage" unionId={u.id} events={u.events}/>
                    <EventSlot title={t('events.types.divorce')} dateLabel={t('editor.divorceDate')}
                               type="divorce" unionId={u.id} events={u.events}
                               afterSave={(d) => (d.trim() && u.status !== 'ended' ? markEnded.mutateAsync([u.id]) : undefined)}/>
                </div>
            ))}
            {detail.family.unions.length === 0 && (
                <div className="union-edit">
                    <p className="muted">{t('editor.noUnion')}</p>
                    {addingSpouse
                        ? <QuickAdd key={addingSpouse} adding={{relation: addingSpouse, unionId: ''}} personId={id} defaultSurname="" onDone={() => setAddingSpouse(null)}/>
                        : <><button onClick={() => setAddingSpouse('spouse')}>{t('family.addSpouse')}</button> <button onClick={() => setAddingSpouse('partner')}>{t('family.addPartner')}</button></>}
                </div>
            )}
        </div>
    )
}
