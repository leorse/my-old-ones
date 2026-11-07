import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import {ipc, type repo} from '../../../ipc'
import {personLabel} from '../../../components/helpers'
import {useAction} from '../../../queries'
import {QuickAdd} from '../../persons/FamilySection'
import {defaultSurnameFor} from '../../persons/surname'

const KINDS = ['biological', 'adoptive', 'step', 'foster', 'legal', 'unknown'] as const

interface Props {
    detail: repo.PersonDetail
    /** Années « 1845 – 1920 » d'une personne affichée dans l'arbre ("" si inconnues). */
    years: (id: string) => string
    onFocus: (id: string) => void
}

/** Deuxième onglet : parents, nature du lien, ajout d'un parent manquant. */
export function ParentsTab({detail, years, onFocus}: Props) {
    const {t} = useTranslation()
    const unlink = useAction(ipc.unlinkParent)
    const updateLink = useAction(ipc.updateParentLink)
    const [adding, setAdding] = useState<'father' | 'mother' | null>(null)
    const parents = detail.family.parents
    const primary = detail.core.names.find((n) => n.isPrimary) ?? detail.core.names[0]
    const has = (role: string) => parents.some((p) => p.role === role)

    return (
        <div className="editor-list">
            {parents.length === 0 && <p className="muted">{t('editor.noParents')}</p>}
            <ul>
                {parents.map((p) => (
                    <li key={p.linkId}>
                        <span className="role">{t(`family.roles.${p.role}`)}</span>
                        <button className="link" onClick={() => onFocus(p.person.id)}>{personLabel(p.person, t('common.unknown'))}</button>
                        <span className="muted">{years(p.person.id)}</span>
                        <select aria-label={t('editor.linkKind')} value={p.kind}
                                onChange={(e) => updateLink.mutate([p.linkId, {kind: e.target.value}])}>
                            {KINDS.map((k) => <option key={k} value={k}>{t(`family.kinds.${k}`)}</option>)}
                        </select>
                        <button className="icon" aria-label={t('family.unlink')} title={t('family.unlink')} onClick={() => unlink.mutate([p.linkId])}>✕</button>
                    </li>
                ))}
            </ul>
            <div className="row">
                {!has('father') && <button aria-pressed={adding === 'father'} onClick={() => setAdding(adding === 'father' ? null : 'father')}>{t('family.addFather')}</button>}
                {!has('mother') && <button aria-pressed={adding === 'mother'} onClick={() => setAdding(adding === 'mother' ? null : 'mother')}>{t('family.addMother')}</button>}
            </div>
            {adding && (
                <QuickAdd key={adding} adding={{relation: adding, unionId: ''}} personId={detail.core.id}
                          defaultSurname={defaultSurnameFor(adding, {surname: primary?.surname ?? '', sex: detail.core.sex})}
                          onDone={() => setAdding(null)}/>
            )}
        </div>
    )
}
