import {describe, expect, it} from 'vitest'
import {defaultSurnameFor} from './surname'

const man = {surname: 'Le Gall', sex: 'M'}
const woman = {surname: 'Kerjean', sex: 'F'}

describe('defaultSurnameFor', () => {
    it('père et fratrie portent le nom de la personne', () => {
        expect(defaultSurnameFor('father', woman)).toBe('Kerjean')
        expect(defaultSurnameFor('sibling', man)).toBe('Le Gall')
    })
    it("l'enfant porte le nom du père", () => {
        expect(defaultSurnameFor('child', man)).toBe('Le Gall')
        expect(defaultSurnameFor('child', woman, man)).toBe('Le Gall')
        expect(defaultSurnameFor('child', woman, woman)).toBe('')
        expect(defaultSurnameFor('child', woman)).toBe('')
    })
    it('mère et conjoint : pas de nom proposé', () => {
        expect(defaultSurnameFor('mother', man)).toBe('')
        expect(defaultSurnameFor('spouse', man)).toBe('')
    })
})
