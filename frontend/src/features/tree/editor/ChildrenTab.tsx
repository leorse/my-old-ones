import {useState, type FormEvent} from 'react'
import {useTranslation} from 'react-i18next'
import {ipc, type repo} from '../../../ipc'
import {personLabel} from '../../../components/helpers'
import {useAction} from '../../../queries'
import {defaultSurnameFor} from '../../persons/surname'
import {DateInput} from '../../persons/EventsSection'

interface Props {
    detail: repo.PersonDetail
    years: (id: string) => string
    onFocus: (id: string) => void
}

/** Troisième onglet : liste des enfants, avec ajout direct (en série : le formulaire reste ouvert). */
export function ChildrenTab({detail, years, onFocus}: Props) {
    const {t} = useTranslation()
    const f = detail.family
    const unlink = useAction(ipc.unlinkParent)
    const primary = detail.core.names.find((n) => n.isPrimary) ?? detail.core.names[0]

    const [unionId, setUnionId] = useState(f.unions.length === 1 ? f.unions[0].id : '')
    const union = f.unions.find((u) => u.id === unionId)
    const partner = (union ?? f.unions[0])?.partners[0]
    const suggested = defaultSurnameFor('child', {surname: primary?.surname ?? '', sex: detail.core.sex}, partner)
    const [surname, setSurname] = useState<string | null>(null) // null : nom suggéré, tant que l'utilisateur n'a rien saisi
    const [given, setGiven] = useState('')
    const [sex, setSex] = useState('unknown')
    const [birth, setBirth] = useState('')
    const [last, setLast] = useState('')

    const add = useAction(async () => {
        const name = (surname ?? suggested).trim()
        const id = await ipc.createRelative(detail.core.id, 'child',
            {surname: name, givenNames: given.split(/\s+/).filter(Boolean), sex}, unionId)
        if (birth.trim()) await ipc.addEvent({type: 'birth', personId: id, unionId: '', date: birth, placeId: '', description: '', notesMd: ''})
        setLast(personLabel({surname: name, givenNames: given.split(/\s+/).filter(Boolean)}, t('common.unknown')))
        setGiven('')
        setBirth('')
    })
    const submit = (e: FormEvent) => {
        e.preventDefault()
        if (!(surname ?? suggested).trim() && !given.trim()) return
        add.mutate([])
    }

    const group = (children: repo.ChildRef[]) => (
        <ul>
            {children.map((c) => (
                <li key={c.linkId}>
                    <button className="link" onClick={() => onFocus(c.person.id)}>{personLabel(c.person, t('common.unknown'))}</button>
                    <span className="muted">{years(c.person.id)}</span>
                    {c.kind !== 'biological' && <small className="muted">({t(`family.kinds.${c.kind}`)})</small>}
                    <button className="icon" aria-label={t('family.unlink')} title={t('family.unlink')} onClick={() => unlink.mutate([c.linkId])}>✕</button>
                </li>
            ))}
        </ul>
    )
    const total = f.unions.reduce((n, u) => n + u.children.length, 0) + f.otherChildren.length

    return (
        <div className="editor-children">
            <div className="editor-list">
                {total === 0 && <p className="muted">{t('editor.noChildren')}</p>}
                {f.unions.filter((u) => u.children.length > 0).map((u) => (
                    <div key={u.id}>
                        {f.unions.length > 1 && <h5>{t(`family.unionTypes.${u.type}`)} · {u.partners.map((p) => personLabel(p, t('common.unknown'))).join(', ')}</h5>}
                        {group(u.children)}
                    </div>
                ))}
                {f.otherChildren.length > 0 && <div>{f.unions.length > 0 && <h5>{t('family.otherChildren')}</h5>}{group(f.otherChildren)}</div>}
            </div>
            <form className="add-child" onSubmit={submit} aria-label={t('editor.addChild')}>
                <strong>{t('editor.addChild')}</strong>
                {f.unions.length > 1 && (
                    <select aria-label={t('editor.withUnion')} value={unionId} onChange={(e) => setUnionId(e.target.value)}>
                        <option value="">{t('editor.noUnionChoice')}</option>
                        {f.unions.map((u) => (
                            <option key={u.id} value={u.id}>{u.partners.map((p) => personLabel(p, t('common.unknown'))).join(', ') || t('common.unknown')}</option>
                        ))}
                    </select>
                )}
                <input aria-label={t('persons.surname')} placeholder={t('persons.surname')} value={surname ?? suggested}
                       onChange={(e) => setSurname(e.target.value)}/>
                <input aria-label={t('persons.givenNames')} placeholder={t('persons.givenNames')} value={given}
                       onChange={(e) => setGiven(e.target.value)}/>
                <select aria-label={t('persons.sex')} value={sex} onChange={(e) => setSex(e.target.value)}>
                    {(['M', 'F', 'other', 'unknown'] as const).map((s) => <option key={s} value={s}>{t(`persons.sexes.${s}`)}</option>)}
                </select>
                <DateInput label={t('editor.birthDate')} value={birth} onChange={setBirth}/>
                <button className="primary" type="submit" disabled={add.isPending}>{t('common.add')}</button>
                {last && <small className="muted" role="status">{t('editor.added', {name: last})}</small>}
            </form>
        </div>
    )
}
