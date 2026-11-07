import {useTranslation} from 'react-i18next'
import type {repo} from '../../ipc'
import {ageText, personLabel} from '../../components/helpers'
import {usePerson} from '../../queries'

type Event = repo.EventDTO

function when(e: Event | undefined, unknown: string): string {
    if (!e) return ''
    return [e.dateDisplay || unknown, e.placeName].filter(Boolean).join(' · ')
}

/** Détails de la personne sélectionnée (naissance, décès, parcours, famille), en bas de la vue arbre. */
export function PersonInfoBar({personId, onOpen}: { personId: string | null; onOpen: (id: string) => void }) {
    const {t} = useTranslation()
    const q = usePerson(personId)
    const d = q.data

    return (
        <section className="tree-info" aria-label={t('info.zone')}>
            {!personId && <p className="muted">{t('info.hint')}</p>}
            {personId && !d && <p className="muted">{q.isError ? String(q.error) : t('info.loading')}</p>}
            {personId && d && <Details d={d} onOpen={onOpen}/>}
        </section>
    )
}

function Details({d, onOpen}: { d: repo.PersonDetail; onOpen: (id: string) => void }) {
    const {t} = useTranslation()
    const unknown = t('info.unknownDate')
    const primary = d.core.names.find((n) => n.isPrimary) ?? d.core.names[0]
    const others = d.core.names.filter((n) => n !== primary)
    const birth = d.events.find((e) => e.type === 'birth')
    const death = d.events.find((e) => e.type === 'death')
    const burial = d.events.find((e) => e.type === 'burial')
    const course = d.events.filter((e) => !['birth', 'death', 'burial'].includes(e.type)).slice(0, 6)
    const f = d.family
    const childCount = f.unions.reduce((n, u) => n + u.children.length, 0) + f.otherChildren.length

    return (
        <>
            <div className="info-head">
                <h3>{primary ? personLabel(primary, t('common.unknown')) : t('common.unknown')}</h3>
                <span className="muted">
                    {t(`persons.sexes.${d.core.sex}`)} · {t(`persons.living.${d.core.living}`)}
                    {d.sosa.length > 0 && ` · ${t('person.sosa', {n: d.sosa.join(', ')})}`}
                    {d.isRoot && ` · ${t('person.root')}`}
                </span>
                <span className="spacer"/>
                <button onClick={() => onOpen(d.core.id)}>{t('info.open')}</button>
            </div>
            <div className="info-grid">
                <div>
                    <h4>{t('info.birth')}</h4>
                    <p>{birth ? when(birth, unknown) : <span className="muted">{t('info.none')}</span>}</p>
                </div>
                <div>
                    <h4>{t('info.death')}</h4>
                    <p>{death ? when(death, unknown) : <span className="muted">{t('info.none')}</span>}</p>
                    {d.ageAtDeath && <p className="muted">{t('info.age', {age: ageText(t, d.ageAtDeath)})}</p>}
                    {burial && <p className="muted">{t('info.burial')} : {when(burial, unknown)}</p>}
                </div>
                <div>
                    <h4>{t('info.events')}</h4>
                    {course.length === 0 ? <p className="muted">{t('info.none')}</p> : (
                        <ul>
                            {course.map((e) => (
                                <li key={e.id}>
                                    {t(`events.types.${e.type}`)}{e.description && ` : ${e.description}`}
                                    {(e.dateDisplay || e.placeName) && <span className="muted"> ({[e.dateDisplay, e.placeName].filter(Boolean).join(' · ')})</span>}
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
                <div>
                    <h4>{t('info.family')}</h4>
                    <p>
                        {t('info.parents')} : {f.parents.length === 0 ? <span className="muted">{t('info.none')}</span>
                            : f.parents.map((p) => personLabel(p.person, t('common.unknown'))).join(', ')}
                    </p>
                    {f.unions.map((u) => (
                        <p key={u.id}>
                            {t(`family.unionTypes.${u.type}`)} : {u.partners.map((p) => personLabel(p, t('common.unknown'))).join(', ') || t('common.unknown')}
                            {u.events.find((e) => e.type === 'marriage')?.dateDisplay &&
                                <span className="muted"> ({u.events.find((e) => e.type === 'marriage')!.dateDisplay})</span>}
                        </p>
                    ))}
                    <p>{t('info.childrenCount', {count: childCount})}{f.siblings.length > 0 && ` · ${t('info.siblings')} : ${f.siblings.length}`}</p>
                </div>
                {others.length > 0 && (
                    <div>
                        <h4>{t('info.otherNames')}</h4>
                        <ul>{others.map((n) => <li key={n.id}>{personLabel(n, t('common.unknown'))} <span className="muted">({t(`person.nameTypes.${n.type}`)})</span></li>)}</ul>
                    </div>
                )}
                {d.core.notesMd && (
                    <div>
                        <h4>{t('info.notes')}</h4>
                        <p className="muted">{d.core.notesMd.length > 160 ? d.core.notesMd.slice(0, 159) + '…' : d.core.notesMd}</p>
                    </div>
                )}
            </div>
        </>
    )
}
