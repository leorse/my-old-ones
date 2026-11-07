import {beforeEach, describe, expect, it, vi} from 'vitest'
import {render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import '../../i18n'

const person = (id: string, given: string, sex = 'M') => ({id, surname: 'Le Gall', givenNames: [given], sex, living: 'deceased'})
const ev = (id: string, type: string, display: string, placeId = '', placeName = '', extra: object = {}) => ({
    id, type, personId: 'me', unionId: '', date: {precision: 'exact', calendar: 'gregorian', originalText: display},
    dateDisplay: display, placeId, placeName, description: '', notesMd: '', ...extra,
})

let detail: ReturnType<typeof makeDetail>
function makeDetail() {
    return {
        core: {id: 'me', sex: 'M', living: 'deceased', isFavorite: false, notesMd: '', createdAt: '', updatedAt: '',
            names: [{id: 'n1', type: 'birth', surname: 'Le Gall', givenNames: ['Yves'], usualGiven: 0, prefix: '', suffix: '', isPrimary: true}]},
        events: [
            ev('b', 'birth', '12 mars 1845', 'pl1', 'Saint-Malo'),
            ev('d', 'death', '3 mai 1920', 'pl2', 'Dinan'),
        ],
        family: {
            parents: [{linkId: 'lp', role: 'father', kind: 'biological', person: person('dad', 'Jean')}],
            unions: [{
                id: 'u1', type: 'marriage', status: 'ongoing', partners: [person('wife', 'Anne', 'F')],
                events: [{...ev('m', 'marriage', '5 juin 1870', 'pl1', 'Saint-Malo'), personId: '', unionId: 'u1'}],
                children: [{linkId: 'lc', kind: 'biological', person: person('kid', 'Paul')}],
            }],
            siblings: [], otherChildren: [],
        },
        ages: [], ageAtDeath: null, sosa: [], isRoot: false, parentAges: [],
    }
}

vi.mock('../../ipc', () => ({
    ipc: {
        getPerson: vi.fn(async () => detail),
        listPlaces: vi.fn(async () => [
            {id: 'pl1', displayName: 'Saint-Malo'}, {id: 'pl2', displayName: 'Dinan'}, {id: 'pl3', displayName: 'Rennes'},
        ]),
        previewDate: vi.fn(async (s: string) => ({display: s, precision: 'exact', calendar: 'gregorian', structured: true})),
        updateEvent: vi.fn(async () => undefined), addEvent: vi.fn(async () => 'new-event'), deleteEvent: vi.fn(async () => undefined),
        updateName: vi.fn(async () => undefined), updatePerson: vi.fn(async () => undefined), updateUnion: vi.fn(async () => undefined),
        unlinkParent: vi.fn(async () => undefined), updateParentLink: vi.fn(async () => undefined),
        createRelative: vi.fn(async () => 'new-kid'), createPlace: vi.fn(async () => 'pl-new'),
    },
}))

import {ipc} from '../../ipc'
import {PersonEditor} from './PersonEditor'

function renderEditor(personId: string | null = 'me') {
    const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
    const onOpen = vi.fn()
    const onFocus = vi.fn()
    render(
        <QueryClientProvider client={qc}>
            <PersonEditor personId={personId} onOpen={onOpen} onFocus={onFocus} years={(id) => (id === 'dad' ? '1810 – 1880' : '')}/>
        </QueryClientProvider>,
    )
    return {onOpen, onFocus}
}

beforeEach(() => {
    detail = makeDetail()
    vi.clearAllMocks()
})

describe('PersonEditor – onglet Informations', () => {
    it('invite à sélectionner une personne quand il n\'y en a pas', () => {
        renderEditor(null)
        expect(screen.getByText(/Sélectionnez une personne dans l'arbre pour la modifier/)).toBeInTheDocument()
    })

    it('affiche les dates et lieux de naissance, décès et mariage', async () => {
        renderEditor()
        expect(await screen.findByRole('textbox', {name: 'Date de naissance'})).toHaveValue('12 mars 1845')
        expect(screen.getByRole('combobox', {name: 'Lieu de naissance'})).toHaveValue('Saint-Malo')
        expect(screen.getByRole('textbox', {name: 'Date de décès'})).toHaveValue('3 mai 1920')
        expect(screen.getByRole('combobox', {name: 'Lieu de décès'})).toHaveValue('Dinan')
        expect(screen.getByRole('textbox', {name: 'Date de mariage'})).toHaveValue('5 juin 1870')
        expect(screen.getByRole('combobox', {name: 'Lieu de mariage'})).toHaveValue('Saint-Malo')
        expect(screen.getByRole('textbox', {name: 'Date de divorce'})).toHaveValue('')
        expect(screen.getByText(/Mariage · Anne LE GALL/)).toBeInTheDocument()
    })

    it("enregistre à la sortie du champ, seulement si la valeur a changé", async () => {
        const user = userEvent.setup()
        renderEditor()
        const date = await screen.findByRole('textbox', {name: 'Date de naissance'})
        await user.click(date)
        await user.tab() // aucun changement : rien n'est enregistré
        await user.tab()
        expect(ipc.updateEvent).not.toHaveBeenCalled()

        await user.clear(date)
        await user.type(date, '13 mars 1845')
        await user.click(screen.getByRole('textbox', {name: 'Date de décès'})) // quitte le groupe « naissance »
        await waitFor(() => expect(ipc.updateEvent).toHaveBeenCalledWith('b', {date: '13 mars 1845', placeId: 'pl1'}))
    })

    it('change le lieu en choisissant un lieu connu', async () => {
        const user = userEvent.setup()
        renderEditor()
        const place = await screen.findByRole('combobox', {name: 'Lieu de naissance'})
        await user.clear(place)
        await user.type(place, 'Rennes')
        await user.click(screen.getByRole('textbox', {name: 'Date de décès'}))
        await waitFor(() => expect(ipc.updateEvent).toHaveBeenCalledWith('b', {date: '12 mars 1845', placeId: 'pl3'}))
    })

    it("n'enregistre pas un lieu inconnu et propose de le créer", async () => {
        const user = userEvent.setup()
        renderEditor()
        const place = await screen.findByRole('combobox', {name: 'Lieu de naissance'})
        await user.clear(place)
        await user.type(place, 'Nulle-part')
        await user.click(screen.getByRole('textbox', {name: 'Date de décès'}))
        expect(ipc.updateEvent).not.toHaveBeenCalled()
        expect(screen.getByText(/Lieu inconnu/)).toBeInTheDocument()
        await user.click(screen.getByRole('button', {name: /Créer le lieu « Nulle-part »/}))
        await waitFor(() => expect(ipc.createPlace).toHaveBeenCalled())
    })

    it('supprime l\'événement quand on vide la date et le lieu', async () => {
        const user = userEvent.setup()
        renderEditor()
        await user.clear(await screen.findByRole('textbox', {name: 'Date de décès'}))
        await user.clear(screen.getByRole('combobox', {name: 'Lieu de décès'}))
        await user.click(screen.getByRole('textbox', {name: 'Date de naissance'}))
        await waitFor(() => expect(ipc.deleteEvent).toHaveBeenCalledWith('d'))
    })

    it("crée l'événement du divorce au premier remplissage et marque l'union comme terminée", async () => {
        const user = userEvent.setup()
        renderEditor()
        await user.type(await screen.findByRole('textbox', {name: 'Date de divorce'}), '1/2/1880')
        await user.click(screen.getByRole('textbox', {name: 'Date de naissance'}))
        await waitFor(() => expect(ipc.addEvent).toHaveBeenCalledWith(
            expect.objectContaining({type: 'divorce', unionId: 'u1', personId: '', date: '1/2/1880'})))
        await waitFor(() => expect(ipc.updateUnion).toHaveBeenCalledWith('u1', {status: 'ended'}))
    })

    it('crée un mariage sur une union qui n\'en a pas encore', async () => {
        detail.family.unions[0].events = []
        const user = userEvent.setup()
        renderEditor()
        await user.type(await screen.findByRole('textbox', {name: 'Date de mariage'}), '5/6/1870')
        await user.click(screen.getByRole('textbox', {name: 'Date de naissance'}))
        await waitFor(() => expect(ipc.addEvent).toHaveBeenCalledWith(
            expect.objectContaining({type: 'marriage', unionId: 'u1', date: '5/6/1870'})))
    })

    it('modifie le nom et le sexe', async () => {
        const user = userEvent.setup()
        renderEditor()
        const given = await screen.findByRole('textbox', {name: 'Prénoms'})
        await user.clear(given)
        await user.type(given, 'Yves Marie')
        await user.click(screen.getByRole('textbox', {name: 'Date de naissance'}))
        await waitFor(() => expect(ipc.updateName).toHaveBeenCalledWith('n1',
            expect.objectContaining({surname: 'Le Gall', givenNames: ['Yves', 'Marie'], type: 'birth'})))
        await user.selectOptions(screen.getByRole('combobox', {name: 'Sexe'}), 'F')
        expect(ipc.updatePerson).toHaveBeenCalledWith('me', {sex: 'F'})
    })

    it('propose d\'ajouter un conjoint quand il n\'y a pas d\'union', async () => {
        detail.family.unions = []
        renderEditor()
        expect(await screen.findByText(/Aucune union/)).toBeInTheDocument()
        expect(screen.getByRole('button', {name: '+ Conjoint(e)'})).toBeInTheDocument()
    })
})

describe('PersonEditor – onglet Parents', () => {
    it('liste les parents, permet de recentrer, de changer la nature du lien et de délier', async () => {
        const user = userEvent.setup()
        const {onFocus} = renderEditor()
        await user.click(await screen.findByRole('tab', {name: 'Parents'}))
        expect(screen.getByText('1810 – 1880')).toBeInTheDocument()
        await user.click(screen.getByRole('button', {name: 'Jean LE GALL'}))
        expect(onFocus).toHaveBeenCalledWith('dad')
        await user.selectOptions(screen.getByRole('combobox', {name: 'Nature du lien'}), 'adoptive')
        expect(ipc.updateParentLink).toHaveBeenCalledWith('lp', {kind: 'adoptive'})
        await user.click(screen.getByRole('button', {name: 'Délier'}))
        expect(ipc.unlinkParent).toHaveBeenCalledWith('lp')
    })

    it("n'offre que le parent manquant, avec le nom prérempli pour un père", async () => {
        const user = userEvent.setup()
        renderEditor()
        await user.click(await screen.findByRole('tab', {name: 'Parents'}))
        expect(screen.queryByRole('button', {name: '+ Père'})).not.toBeInTheDocument()
        await user.click(screen.getByRole('button', {name: '+ Mère'}))
        expect(screen.getByRole('button', {name: '+ Mère'})).toHaveAttribute('aria-pressed', 'true')
        expect(screen.getByRole('textbox', {name: 'Nom'})).toHaveValue('') // la mère garde son nom de naissance
    })
})

describe('PersonEditor – onglet Enfants', () => {
    it('liste les enfants et ajoute directement un enfant avec sa date de naissance', async () => {
        const user = userEvent.setup()
        renderEditor()
        await user.click(await screen.findByRole('tab', {name: 'Enfants'}))
        expect(screen.getByRole('button', {name: 'Paul LE GALL'})).toBeInTheDocument()
        const form = screen.getByRole('form', {name: 'Ajouter un enfant'})
        expect(screen.getByRole('textbox', {name: 'Nom'})).toHaveValue('Le Gall') // nom du père
        await user.type(screen.getByRole('textbox', {name: 'Prénoms'}), 'Marie')
        await user.selectOptions(screen.getByRole('combobox', {name: 'Sexe'}), 'F')
        await user.type(screen.getByRole('textbox', {name: 'Date de naissance'}), '1/1/1900')
        await user.click(screen.getByRole('button', {name: 'Ajouter'}))
        await waitFor(() => expect(ipc.createRelative).toHaveBeenCalledWith('me', 'child',
            {surname: 'Le Gall', givenNames: ['Marie'], sex: 'F'}, 'u1'))
        await waitFor(() => expect(ipc.addEvent).toHaveBeenCalledWith(
            expect.objectContaining({type: 'birth', personId: 'new-kid', date: '1/1/1900'})))
        // ajout en série : le formulaire reste, prêt pour le suivant
        expect(await screen.findByRole('status')).toHaveTextContent('Marie LE GALL')
        expect(form).toBeInTheDocument()
        expect(screen.getByRole('textbox', {name: 'Prénoms'})).toHaveValue('')
    })

    it("demande l'union quand la personne en a plusieurs", async () => {
        detail.family.unions.push({
            id: 'u2', type: 'marriage', status: 'ended', partners: [person('wife2', 'Louise', 'F')], events: [], children: [],
        })
        const user = userEvent.setup()
        renderEditor()
        await user.click(await screen.findByRole('tab', {name: 'Enfants'}))
        const sel = screen.getByRole('combobox', {name: 'Union des parents'})
        expect(sel).toHaveValue('')
        await user.selectOptions(sel, 'u2')
        await user.type(screen.getByRole('textbox', {name: 'Prénoms'}), 'Luc')
        await user.click(screen.getByRole('button', {name: 'Ajouter'}))
        await waitFor(() => expect(ipc.createRelative).toHaveBeenCalledWith('me', 'child', expect.objectContaining({givenNames: ['Luc']}), 'u2'))
    })

    it('délie un enfant sans le supprimer', async () => {
        const user = userEvent.setup()
        renderEditor()
        await user.click(await screen.findByRole('tab', {name: 'Enfants'}))
        await user.click(screen.getByRole('button', {name: 'Délier'}))
        expect(ipc.unlinkParent).toHaveBeenCalledWith('lc')
    })
})
