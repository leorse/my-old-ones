import {describe, expect, it} from 'vitest'
import i18n from './index'

describe('i18n', () => {
    it('traduit en français et gère le pluriel', () => {
        expect(i18n.t('nav.persons')).toBe('Personnes')
        expect(i18n.t('status.persons', {count: 1})).toBe('1 personne')
        expect(i18n.t('status.persons', {count: 3})).toBe('3 personnes')
    })
})
