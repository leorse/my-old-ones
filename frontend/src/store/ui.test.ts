import {beforeEach, describe, expect, it} from 'vitest'
import {useUI} from './ui'

describe('store UI', () => {
    beforeEach(() => useUI.setState({view: 'persons', selectedPersonId: null}))

    it('change de vue', () => {
        useUI.getState().setView('stats')
        expect(useUI.getState().view).toBe('stats')
    })

    it('mémorise le thème', () => {
        useUI.getState().setTheme('dark')
        expect(useUI.getState().theme).toBe('dark')
    })

    it('sélectionne une personne', () => {
        useUI.getState().selectPerson('abc')
        expect(useUI.getState().selectedPersonId).toBe('abc')
    })
})
