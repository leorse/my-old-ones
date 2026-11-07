import {describe, expect, it, vi} from 'vitest'
import {render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import '../../i18n'

const detail = {
    core: {
        id: 'p1', sex: 'M', living: 'deceased', isFavorite: false, notesMd: 'Marin', createdAt: '', updatedAt: '',
        names: [{id: 'n1', type: 'birth', surname: 'Le Gall', givenNames: ['Yves'], usualGiven: 0, prefix: '', suffix: '', isPrimary: true}],
    },
    events: [
        {id: 'e1', type: 'birth', personId: 'p1', unionId: '', date: {}, dateDisplay: '12 mars 1845', placeId: '', placeName: 'Saint-Malo', description: '', notesMd: ''},
        {id: 'e2', type: 'death', personId: 'p1', unionId: '', date: {}, dateDisplay: '1900', placeId: '', placeName: '', description: '', notesMd: ''},
    ],
    family: {
        parents: [{linkId: 'l1', role: 'father', kind: 'biological', person: {id: 'p0', surname: 'Le Gall', givenNames: ['Jean'], sex: 'M', living: 'deceased'}}],
        unions: [], siblings: [], otherChildren: [],
    },
    ages: [{eventId: 'e2', years: 54, approx: true}],
    ageAtDeath: {eventId: 'e2', years: 54, approx: true},
    sosa: [4],
    isRoot: false,
    parentAges: [{eventId: 'p0', years: 30, approx: false}],
}

vi.mock('../../ipc', () => ({
    ipc: {
        getPerson: vi.fn(async () => detail),
        listPlaces: vi.fn(async () => []),
        previewDate: vi.fn(async (s: string) => ({display: s, precision: 'exact', calendar: 'gregorian', structured: true})),
        addEvent: vi.fn(async () => 'x'), deleteEvent: vi.fn(), updatePerson: vi.fn(), setPrimaryName: vi.fn(),
        deleteName: vi.fn(), addName: vi.fn(), setRoot: vi.fn(), unlinkParent: vi.fn(), reorderChildren: vi.fn(),
        deleteUnion: vi.fn(), createRelative: vi.fn(), createPlace: vi.fn(), deleteImpact: vi.fn(async () => ({
            people: [], events: 2, parentLinks: 1, unions: 0, mediaLinks: 0, citations: 0, becomingIsolated: [],
        })), deletePeople: vi.fn(),
    },
}))

import {ipc} from '../../ipc'
import {PersonPage} from './PersonPage'

function renderPage() {
    const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
    return render(<QueryClientProvider client={qc}><PersonPage id="p1"/></QueryClientProvider>)
}

describe('PersonPage', () => {
    it('affiche identité, Sosa, événements, âges et parents', async () => {
        renderPage()
        expect(await screen.findByRole('heading', {level: 2, name: 'Yves LE GALL'})).toBeInTheDocument()
        expect(screen.getByText('Sosa n° 4')).toBeInTheDocument()
        expect(screen.getByText(/12 mars 1845/)).toBeInTheDocument()
        expect(screen.getByText(/Saint-Malo/)).toBeInTheDocument()
        expect(screen.getByText('Âge au décès : ~54 ans')).toBeInTheDocument()
        expect(screen.getByText(/Âge du père à la naissance : 30 ans/)).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Jean LE GALL'})).toBeInTheDocument()
    })

    it('ajoute un événement via le formulaire', async () => {
        const user = userEvent.setup()
        renderPage()
        await user.click(await screen.findByRole('button', {name: 'Ajouter un événement'}))
        await user.type(screen.getByRole('textbox', {name: 'Date'}), '3 germinal an XII')
        await user.click(screen.getByRole('button', {name: 'Ajouter'}))
        await waitFor(() => expect(ipc.addEvent).toHaveBeenCalled())
        expect(vi.mocked(ipc.addEvent).mock.calls[0][0]).toMatchObject({type: 'birth', personId: 'p1', date: '3 germinal an XII'})
    })

    it("affiche l'aperçu d'impact avant suppression", async () => {
        const user = userEvent.setup()
        renderPage()
        await user.click(await screen.findByRole('button', {name: 'Supprimer'}))
        expect(await screen.findByText(/2 événement\(s\), 1 lien\(s\) de filiation/)).toBeInTheDocument()
        await user.click(screen.getByRole('radio', {name: /descendants exclusifs/}))
        await waitFor(() => expect(ipc.deleteImpact).toHaveBeenLastCalledWith('p1', 'descendants'))
    })
})
