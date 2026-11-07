import type {TFunction} from 'i18next'
import {useTranslation} from 'react-i18next'
import {useUI} from '../store/ui'

interface Named {
    id?: string
    surname: string
    givenNames: string[]
}

/** « Prénoms NOM » ; repli si la personne n'a aucun nom saisi. */
export function personLabel(p: Named, fallback = ''): string {
    const label = `${p.givenNames.join(' ')} ${p.surname.toUpperCase()}`.trim()
    return label || fallback
}

/** Lien cliquable vers la fiche d'une personne. */
export function PersonLink({person}: { person: Named & { id: string } }) {
    const select = useUI((s) => s.selectPerson)
    const {t} = useTranslation()
    return <button className="link" onClick={() => select(person.id)}>{personLabel(person, t('common.unknown'))}</button>
}

export function ageText(t: TFunction, a: { years: number; approx: boolean }): string {
    return a.approx ? t('person.ageApprox', {count: a.years}) : t('person.age', {count: a.years})
}
