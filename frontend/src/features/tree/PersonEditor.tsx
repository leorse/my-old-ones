import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import {personLabel} from '../../components/helpers'
import {usePerson} from '../../queries'
import {BasicInfoTab} from './editor/BasicInfoTab'
import {ChildrenTab} from './editor/ChildrenTab'
import {ParentsTab} from './editor/ParentsTab'

type Tab = 'info' | 'parents' | 'children'
const TABS: Tab[] = ['info', 'parents', 'children']

interface Props {
    personId: string | null
    /** Ouvre la fiche complète. */
    onOpen: (id: string) => void
    /** Recentre l'arbre sur une personne. */
    onFocus: (id: string) => void
    /** Années « 1845 – 1920 » d'une personne de l'arbre. */
    years: (id: string) => string
}

/** Zone du bas de la vue arbre : modification de la personne sélectionnée, en onglets. */
export function PersonEditor({personId, onOpen, onFocus, years}: Props) {
    const {t} = useTranslation()
    const [tab, setTab] = useState<Tab>('info')
    const q = usePerson(personId)
    const d = q.data
    const primary = d ? (d.core.names.find((n) => n.isPrimary) ?? d.core.names[0]) : undefined

    return (
        <section className="tree-editor" aria-label={t('editor.zone')}>
            {!personId && <p className="muted pad-sm">{t('editor.hint')}</p>}
            {personId && !d && <p className="muted pad-sm">{q.isError ? String(q.error) : t('info.loading')}</p>}
            {personId && d && <>
                <div className="editor-head">
                    <strong>{primary ? personLabel(primary, t('common.unknown')) : t('common.unknown')}</strong>
                    <span className="muted">
                        {t(`persons.living.${d.core.living}`)}
                        {d.sosa.length > 0 && ` · ${t('person.sosa', {n: d.sosa.join(', ')})}`}
                        {d.isRoot && ` · ${t('person.root')}`}
                    </span>
                    <div role="tablist" aria-label={t('editor.tabs')} className="tabs">
                        {TABS.map((k) => (
                            <button key={k} role="tab" aria-selected={tab === k} onClick={() => setTab(k)}>{t(`editor.tab.${k}`)}</button>
                        ))}
                    </div>
                    <span className="spacer"/>
                    <button onClick={() => onOpen(d.core.id)}>{t('info.open')}</button>
                </div>
                <div className="editor-body" role="tabpanel" key={`${d.core.id}|${tab}`}>
                    {tab === 'info' && <BasicInfoTab detail={d}/>}
                    {tab === 'parents' && <ParentsTab detail={d} years={years} onFocus={onFocus}/>}
                    {tab === 'children' && <ChildrenTab detail={d} years={years} onFocus={onFocus}/>}
                </div>
            </>}
        </section>
    )
}
