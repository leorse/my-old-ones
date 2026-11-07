export type NewRelation = 'father' | 'mother' | 'spouse' | 'partner' | 'child' | 'sibling'

interface Who {
    surname: string
    sex: string
}

/**
 * Nom de famille proposé à la création d'un proche :
 * - père, frère/sœur : le nom de la personne ;
 * - enfant : le nom du père, c'est-à-dire celui de la personne si c'est un homme,
 *   sinon celui de son conjoint homme (s'il est connu) ;
 * - mère, conjoint(e) : aucun (nom de naissance propre).
 */
export function defaultSurnameFor(relation: NewRelation, person: Who, partner?: Who): string {
    switch (relation) {
        case 'father':
        case 'sibling':
            return person.surname
        case 'child':
            if (person.sex === 'M') return person.surname
            return partner && partner.sex === 'M' ? partner.surname : ''
        default:
            return ''
    }
}
