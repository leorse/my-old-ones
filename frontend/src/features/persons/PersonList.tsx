import {useState, type FormEvent} from 'react'
import {useTranslation} from 'react-i18next'
import {ipc} from '../../ipc'
import {personLabel} from '../../components/helpers'
import {useAction, usePersons} from '../../queries'
import {useUI} from '../../store/ui'
import {PersonPage} from './PersonPage'

const SEXES = ['M', 'F', 'other', 'unknown'] as const

export function PersonList() {
    const {t} = useTranslation()
    const persons = usePersons()
    const create = useAction(ipc.createPerson)
    const selected = useUI((s) => s.selectedPersonId)
    const select = useUI((s) => s.selectPerson)
    const [surname, setSurname] = useState('')
    const [given, setGiven] = useState('')
    const [sex, setSex] = useState<string>('unknown')

    const submit = (e: FormEvent) => {
        e.preventDefault()
        if (!surname.trim() && !given.trim()) return
        create.mutate(
            [{surname: surname.trim(), givenNames: given.split(/\s+/).filter(Boolean), sex}],
            {onSuccess: (id) => { setSurname(''); setGiven(''); select(id) }},
        )
    }

    return (
        <div className="split">
            <section className="list-pane" aria-label={t('persons.title')}>
                <form className="stack" onSubmit={submit}>
                    <input aria-label={t('persons.surname')} placeholder={t('persons.surname')}
                           value={surname} onChange={(e) => setSurname(e.target.value)}/>
                    <input aria-label={t('persons.givenNames')} placeholder={t('persons.givenNames')}
                           value={given} onChange={(e) => setGiven(e.target.value)}/>
                    <div className="row tight">
                        <select aria-label={t('persons.sex')} value={sex} onChange={(e) => setSex(e.target.value)}>
                            {SEXES.map((s) => <option key={s} value={s}>{t(`persons.sexes.${s}`)}</option>)}
                        </select>
                        <button className="primary" type="submit">{t('persons.add')}</button>
                    </div>
                </form>
                {persons.data?.length ? (
                    <ul className="plist">
                        {persons.data.map((p) => (
                            <li key={p.id}>
                                <button aria-current={selected === p.id ? 'true' : undefined} onClick={() => select(p.id)}>
                                    {personLabel(p, t('common.unknown'))}
                                </button>
                            </li>
                        ))}
                    </ul>
                ) : <p className="muted">{t('persons.empty')}</p>}
            </section>
            <section className="detail-pane">
                {selected ? <PersonPage key={selected} id={selected}/> : <p className="muted">{t('persons.select')}</p>}
            </section>
        </div>
    )
}
