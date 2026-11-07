import {useState, type FormEvent} from 'react'
import {useTranslation} from 'react-i18next'
import {ipc, type repo} from '../../ipc'
import {PersonLink, ageText} from '../../components/helpers'
import {useAction} from '../../queries'
import {defaultSurnameFor} from './surname'

export type Relation = 'father' | 'mother' | 'spouse' | 'partner' | 'child' | 'sibling'

export interface Adding {
    relation: Relation
    unionId: string
}

export function QuickAdd({adding, defaultSurname, personId, onDone}: {
    adding: Adding; defaultSurname: string; personId: string; onDone: () => void
}) {
    const {t} = useTranslation()
    const [surname, setSurname] = useState(defaultSurname)
    const [given, setGiven] = useState('')
    const [sex, setSex] = useState(adding.relation === 'father' ? 'M' : adding.relation === 'mother' ? 'F' : 'unknown')
    const create = useAction(ipc.createRelative)
    const fixedSex = adding.relation === 'father' || adding.relation === 'mother'

    const submit = (e: FormEvent) => {
        e.preventDefault()
        create.mutate(
            [personId, adding.relation, {surname: surname.trim(), givenNames: given.split(/\s+/).filter(Boolean), sex}, adding.unionId],
            {onSuccess: onDone},
        )
    }
    return (
        <form className="row" onSubmit={submit} aria-label={t('family.newPerson')}>
            <input autoFocus={!defaultSurname} aria-label={t('persons.surname')} placeholder={t('persons.surname')} value={surname}
                   onChange={(e) => setSurname(e.target.value)}/>
            <input autoFocus={!!defaultSurname} aria-label={t('persons.givenNames')} placeholder={t('persons.givenNames')} value={given}
                   onChange={(e) => setGiven(e.target.value)}/>
            {!fixedSex && (
                <select aria-label={t('persons.sex')} value={sex} onChange={(e) => setSex(e.target.value)}>
                    {(['M', 'F', 'other', 'unknown'] as const).map((s) => <option key={s} value={s}>{t(`persons.sexes.${s}`)}</option>)}
                </select>
            )}
            <button className="primary" type="submit">{t('family.create')}</button>
            <button type="button" onClick={onDone}>{t('common.cancel')}</button>
        </form>
    )
}

export function FamilySection({detail}: { detail: repo.PersonDetail }) {
    const {t} = useTranslation()
    const personId = detail.core.id
    const primary = detail.core.names.find((n) => n.isPrimary) ?? detail.core.names[0]
    const [adding, setAdding] = useState<Adding | null>(null)
    const unlink = useAction(ipc.unlinkParent)
    const reorder = useAction(ipc.reorderChildren)
    const delUnion = useAction(ipc.deleteUnion)
    const f = detail.family

    const addBtn = (relation: Relation, label: string, unionId = '') => (
        <button onClick={() => setAdding({relation, unionId})}>{label}</button>
    )
    const parentAge = (parentId: string) => detail.parentAges.find((a) => a.eventId === parentId)

    const move = (links: repo.ChildRef[], i: number, d: -1 | 1) => {
        const ids = links.map((c) => c.linkId)
        const j = i + d
        if (j < 0 || j >= ids.length) return
        ;[ids[i], ids[j]] = [ids[j], ids[i]]
        reorder.mutate([ids])
    }
    const childList = (children: repo.ChildRef[], reorderable: boolean) => (
        <ul>
            {children.map((c, i) => (
                <li key={c.linkId}>
                    <PersonLink person={c.person}/>
                    {c.kind !== 'biological' && <small className="muted"> ({t(`family.kinds.${c.kind}`)})</small>}
                    {reorderable && children.length > 1 && <>
                        <button className="icon" aria-label={t('family.moveUp')} disabled={i === 0} onClick={() => move(children, i, -1)}>↑</button>
                        <button className="icon" aria-label={t('family.moveDown')} disabled={i === children.length - 1} onClick={() => move(children, i, 1)}>↓</button>
                    </>}
                </li>
            ))}
        </ul>
    )

    return (
        <section aria-labelledby="sec-family">
            <h3 id="sec-family">{t('person.family')}</h3>

            <h4>{t('family.parents')}</h4>
            {f.parents.length === 0 && <p className="muted">{t('family.none')}</p>}
            <ul>
                {f.parents.map((p) => {
                    const a = parentAge(p.person.id)
                    return (
                        <li key={p.linkId}>
                            <span className="muted">{t(`family.roles.${p.role}`)} · </span>
                            <PersonLink person={p.person}/>
                            {p.kind !== 'biological' && <small className="muted"> ({t(`family.kinds.${p.kind}`)})</small>}
                            {a && <small className="muted"> — {t(p.role === 'father' ? 'person.fatherAgeAtBirth' : p.role === 'mother' ? 'person.motherAgeAtBirth' : 'person.parentAgeAtBirth', {age: ageText(t, a)})}</small>}
                            <button className="icon" aria-label={t('family.unlink')} title={t('family.unlink')} onClick={() => unlink.mutate([p.linkId])}>⛓‍💥</button>
                        </li>
                    )
                })}
            </ul>
            <div className="row">{addBtn('father', t('family.addFather'))}{addBtn('mother', t('family.addMother'))}</div>

            <h4>{t('family.unions')}</h4>
            {f.unions.length === 0 && <p className="muted">{t('family.none')}</p>}
            {f.unions.map((u) => (
                <div key={u.id} className="union">
                    <div>
                        <strong>{t(`family.unionTypes.${u.type}`)}</strong>
                        {u.partners.length > 0 ? <> · {u.partners.map((p, i) => <span key={p.id}>{i > 0 && ', '}<PersonLink person={p}/></span>)}</> : null}
                        {u.events.map((e) => <small key={e.id} className="muted"> · {t(`events.types.${e.type}`)} {e.dateDisplay}</small>)}
                        <button className="icon" aria-label={t('family.deleteUnion')} title={t('family.deleteUnion')} onClick={() => delUnion.mutate([u.id])}>✕</button>
                    </div>
                    {childList(u.children, true)}
                    {addBtn('child', t('family.addChild'), u.id)}
                </div>
            ))}
            <div className="row">{addBtn('spouse', t('family.addSpouse'))}{addBtn('partner', t('family.addPartner'))}</div>

            {(f.otherChildren.length > 0 || f.unions.length === 0) && <>
                <h4>{f.unions.length ? t('family.otherChildren') : t('family.children')}</h4>
                {childList(f.otherChildren, true)}
                <div className="row">{addBtn('child', t('family.addChild'))}</div>
            </>}

            <h4>{t('family.siblings')}</h4>
            {f.siblings.length === 0 ? <p className="muted">{t('family.none')}</p> :
                <ul>{f.siblings.map((s) => <li key={s.id}><PersonLink person={s}/></li>)}</ul>}
            <div className="row">{addBtn('sibling', t('family.addSibling'))}</div>

            {adding && (
                <QuickAdd key={`${adding.relation}|${adding.unionId}`} adding={adding} personId={personId}
                          defaultSurname={defaultSurnameFor(adding.relation, {surname: primary?.surname ?? '', sex: detail.core.sex},
                              f.unions.find((u) => u.id === adding.unionId)?.partners[0] ??  f.unions[0]?.partners[0])}
                          onDone={() => setAdding(null)}/>
            )}
        </section>
    )
}
