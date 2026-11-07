import {beforeAll, describe, expect, it, vi} from 'vitest'
import {render, screen, waitFor, act} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import '../i18n'

const person = (id: string, given: string) => ({id, surname: 'Test', givenNames: [given], sex: 'M', living: 'deceased'})
const treeFor = (id: string, given: string) => ({
    focus: id, truncated: false, nodes: [{person: person(id, given), birth: '1900', death: '1970'}], links: [], unions: [],
})

let current = {path: '/a.moo', name: 'A', mediaDir: '', persons: 1}
const roots: Record<string, string> = {'/a.moo': 'a1', '/b.moo': 'b1'}
const names: Record<string, string> = {a1: 'Alice', b1: 'Bruno'}

vi.mock('../ipc', () => ({
    Events: {projectChanged: 'project:changed', historyChanged: 'history:changed', jobUpdate: 'job:update'},
    onEvent: vi.fn(() => () => {}),
    ipc: {
        currentProject: vi.fn(async () => current),
        recentProjects: vi.fn(async () => []),
        historyState: vi.fn(async () => ({canUndo: false, canRedo: false, undoLabel: '', redoLabel: ''})),
        listJobs: vi.fn(async () => []),
        listPersons: vi.fn(async () => [person(roots[current.path], names[roots[current.path]])]),
        getRoot: vi.fn(async () => roots[current.path]),
        // le focus d'un autre projet n'existe pas ici : comme le vrai backend, on refuse
        getTree: vi.fn(async (focus: string) => {
            if (roots[current.path] !== focus) throw new Error(`personne ${focus} introuvable`)
            return treeFor(focus, names[focus])
        }),
        closeProject: vi.fn(),
    },
}))

import {useUI} from '../store/ui'
import {Shell} from './Shell'

beforeAll(() => {
    globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} } as never
})

describe('changement de projet', () => {
    it("réaffiche l'arbre du nouveau projet et oublie le centre de l'ancien", async () => {
        const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
        useUI.setState({view: 'tree', treeFocus: null, selectedPersonId: 'a1'})
        render(<QueryClientProvider client={qc}><Shell/></QueryClientProvider>)
        expect(await screen.findByText('Alice TEST', {selector: 'text'})).toBeInTheDocument()

        // l'utilisateur recentre l'arbre sur une personne du projet A
        act(() => useUI.getState().setTreeFocus('a1'))

        // ouverture d'un autre projet (le backend émet « project:changed » -> invalidation)
        current = {path: '/b.moo', name: 'B', mediaDir: '', persons: 1}
        await act(async () => { await qc.invalidateQueries() })

        expect(await screen.findByText('Bruno TEST', {selector: 'text'})).toBeInTheDocument()
        expect(screen.queryByText('Alice TEST', {selector: 'text'})).not.toBeInTheDocument()
        await waitFor(() => expect(useUI.getState().treeFocus).toBeNull())
        expect(useUI.getState().selectedPersonId).toBeNull()
    })
})
