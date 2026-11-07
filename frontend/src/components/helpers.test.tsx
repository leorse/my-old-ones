import {describe, expect, it} from 'vitest'
import i18n from '../i18n'
import {ageText, personLabel} from './helpers'

describe('helpers', () => {
    it('formate un nom', () => {
        expect(personLabel({surname: 'Le Gall', givenNames: ['Yves', 'Marie']})).toBe('Yves Marie LE GALL')
        expect(personLabel({surname: '', givenNames: []}, 'Inconnu')).toBe('Inconnu')
    })
    it('formate un âge exact ou approximatif', () => {
        expect(ageText(i18n.t, {years: 1, approx: false})).toBe('1 an')
        expect(ageText(i18n.t, {years: 54, approx: false})).toBe('54 ans')
        expect(ageText(i18n.t, {years: 54, approx: true})).toBe('~54 ans')
    })
})
