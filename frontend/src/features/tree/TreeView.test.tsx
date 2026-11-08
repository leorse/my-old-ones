import {beforeAll, describe, expect, it, vi} from 'vitest'
import {fireEvent, render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import '../../i18n'

const p = (id: string, given: string, surname: string, sex: string, hasFather = false, hasMother = false, childCount = 0) => ({
    person: {id, surname, givenNames: [given], sex, living: 'deceased'}, birth: '1900', death: '1970',
    hasFather, hasMother, hasParents: hasFather || hasMother, childCount,
})
const tree = {
    focus: 'me', truncated: false,
    nodes: [p('me', 'Yves', 'Le Gall', 'M', true, true, 3), p('dad', 'Jean', 'Le Gall', 'M', false, false, 1), p('mum', 'Anne', 'Kerjean', 'F'), p('kid', 'Paul', 'Le Gall', 'M', true, false, 0)],
    links: [
        {id: '1', childId: 'me', parentId: 'dad', role: 'father', kind: 'biological', unionId: ''},
        {id: '2', childId: 'me', parentId: 'mum', role: 'mother', kind: 'biological', unionId: ''},
        {id: '3', childId: 'kid', parentId: 'me', role: 'father', kind: 'biological', unionId: ''},
    ],
    unions: [],
}

const observed: Element[] = []

const detail = {
    core: {id: 'dad', sex: 'M', living: 'deceased', isFavorite: false, notesMd: 'Marin pêcheur', createdAt: '', updatedAt: '',
        names: [{id: 'n', type: 'birth', surname: 'Le Gall', givenNames: ['Jean'], usualGiven: 0, prefix: '', suffix: '', isPrimary: true}]},
    events: [
        {id: 'b', type: 'birth', personId: 'dad', unionId: '', date: {}, dateDisplay: '12 mars 1845', placeId: '', placeName: 'Saint-Malo', description: '', notesMd: ''},
        {id: 'd', type: 'death', personId: 'dad', unionId: '', date: {}, dateDisplay: '3 mai 1920', placeId: '', placeName: 'Dinan', description: '', notesMd: ''},
        {id: 'o', type: 'occupation', personId: 'dad', unionId: '', date: {}, dateDisplay: '', placeId: '', placeName: '', description: 'laboureur', notesMd: ''},
    ],
    family: {parents: [], unions: [], siblings: [], otherChildren: []},
    ages: [], ageAtDeath: {eventId: 'd', years: 75, approx: false}, sosa: [4], isRoot: false, parentAges: [],
}

vi.mock('../../ipc', () => ({
    ipc: {
        getRoot: vi.fn(async () => 'me'),
        getPerson: vi.fn(async () => detail),
        listPlaces: vi.fn(async () => []),
        previewDate: vi.fn(async (s: string) => ({display: s, precision: 'exact', calendar: 'gregorian', structured: true})),
        listPersons: vi.fn(async () => [{id: 'me', surname: 'Le Gall', givenNames: ['Yves'], sex: 'M', living: 'deceased'}]),
        getTree: vi.fn(async (focus: string) => ({...tree, focus})),
        createRelative: vi.fn(async () => 'new'),
    },
}))

import {ipc} from '../../ipc'
import {useUI} from '../../store/ui'
import {TreeView} from './TreeView'

beforeAll(() => {
    // jsdom n'implémente pas la capture de pointeur
    SVGElement.prototype.setPointerCapture = () => {}
    SVGElement.prototype.releasePointerCapture = () => {}
    SVGElement.prototype.hasPointerCapture = () => false
    // jsdom n'implémente pas ResizeObserver
    globalThis.ResizeObserver = class { observe(el: Element) { observed.push(el) } unobserve() {} disconnect() {} } as never
})

function renderTree() {
    const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
    return render(<QueryClientProvider client={qc}><TreeView/></QueryClientProvider>)
}

describe('TreeView', () => {
    it("dessine la personne, ses parents et ses enfants autour de la racine", async () => {
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        expect(await screen.findByText('Yves LE GALL', {selector: 'text'})).toBeInTheDocument()
        expect(screen.getByText('Jean LE GALL', {selector: 'text'})).toBeInTheDocument()
        expect(screen.getByText('Anne KERJEAN', {selector: 'text'})).toBeInTheDocument()
        expect(screen.getByText('Paul LE GALL', {selector: 'text'})).toBeInTheDocument()
        expect(screen.getAllByText('1900 – 1970', {selector: 'text'}).length).toBe(4)
        expect(ipc.getTree).toHaveBeenCalledWith('me', 4, 2)
    })

    it('change de type d\'arbre et demande les bonnes profondeurs', async () => {
        const user = userEvent.setup()
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        await screen.findByText('Yves LE GALL', {selector: 'text'})
        await user.click(screen.getByRole('button', {name: 'Ascendants'}))
        await waitFor(() => expect(ipc.getTree).toHaveBeenLastCalledWith('me', 4, 0))
        await user.click(screen.getByRole('button', {name: 'Descendants'}))
        await waitFor(() => expect(ipc.getTree).toHaveBeenLastCalledWith('me', 0, 2))
    })

    it('sélectionne une personne, propose les actions et recentre au double-clic', async () => {
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        const dad = (await screen.findByText('Jean LE GALL', {selector: 'text'})).closest('g')!
        fireEvent.click(dad)
        expect(await screen.findByRole('button', {name: 'Ouvrir la fiche'})).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', {name: '+ Enfant'}))
        expect(screen.getByRole('form', {name: 'Nouvelle personne'})).toBeInTheDocument()
        fireEvent.doubleClick(dad)
        expect(useUI.getState().treeFocus).toBe('dad')
    })

    it("propose d'ajouter père/mère seulement s'il n'y en a pas, et enfant / fratrie selon le cas", async () => {
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        const nodeOf = async (name: string) => (await screen.findByText(name, {selector: 'text'})).closest('g.tnode') as SVGGElement
        const adds = (g: SVGGElement) => [...g.querySelectorAll('.tadd')].map((e) => e.getAttribute('aria-label'))

        // me : deux parents connus => pas de + père / mère ; fratrie et enfant proposés
        expect(adds(await nodeOf('Yves LE GALL'))).toEqual(['Ajouter un frère ou une sœur', 'Ajouter un enfant', 'Ajouter un concubin'])
        // dad : aucun parent => père, mère et enfant, mais pas de fratrie (impossible sans parents)
        expect(adds(await nodeOf('Jean LE GALL'))).toEqual(['Ajouter un père', 'Ajouter une mère', 'Ajouter un enfant', 'Ajouter un concubin'])
        // kid : père connu, mère absente
        expect(adds(await nodeOf('Paul LE GALL'))).toEqual(['Ajouter une mère', 'Ajouter un frère ou une sœur', 'Ajouter un enfant', 'Ajouter un concubin'])
    })

    it('un clic sur « + enfant » sélectionne la case et ouvre le formulaire de création', async () => {
        const user = userEvent.setup()
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        const dad = (await screen.findByText('Jean LE GALL', {selector: 'text'})).closest('g.tnode') as SVGGElement
        fireEvent.click(dad.querySelector('[aria-label="Ajouter un enfant"]')!)
        const form = await screen.findByRole('form', {name: 'Nouvelle personne'})
        expect(screen.getByRole('heading', {name: 'Jean LE GALL'})).toBeInTheDocument()
        await user.type(screen.getByRole('textbox', {name: 'Prénoms'}), 'Marie')
        await user.click(screen.getByRole('button', {name: 'Créer'}))
        await waitFor(() => expect(ipc.createRelative).toHaveBeenCalled())
        expect(vi.mocked(ipc.createRelative).mock.calls.at(-1)![0]).toBe('dad')
        expect(vi.mocked(ipc.createRelative).mock.calls.at(-1)![1]).toBe('child')
        expect(form).toBeDefined()
    })

    it('affiche une pastille pour les enfants existants mais non dessinés, et un clic recentre', async () => {
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        const me = (await screen.findByText('Yves LE GALL', {selector: 'text'})).closest('g.tnode') as SVGGElement
        // Yves a 3 enfants en base mais un seul (Paul) est dessiné : la pastille annonce le TOTAL (3), dont 2 masqués
        const badge = me.querySelector('.tbadge')!
        expect(badge).toHaveAttribute('aria-label', '3 enfants, dont 2 non affiché(s) : cliquer pour les afficher')
        expect(badge.querySelector('text')!.textContent).toBe('3')
        // père et mère présents dans l'arbre : aucune pastille parent
        expect(me.querySelectorAll('.tbadge')).toHaveLength(1)
        fireEvent.click(badge)
        expect(useUI.getState().treeFocus).toBe('me')
    })

    it('affiche une pastille parent quand le parent existe mais est hors de la vue', async () => {
        useUI.setState({treeFocus: null, treeMode: 'descendants', treeUp: 4, treeDown: 2})
        renderTree()
        // en mode descendants, Yves (focus) a père et mère en base mais aucun n'est dessiné
        const me = (await screen.findByText('Yves LE GALL', {selector: 'text'})).closest('g.tnode') as SVGGElement
        const labels = [...me.querySelectorAll('.tbadge')].map((b) => b.getAttribute('aria-label'))
        expect(labels.some((l) => l?.includes('père'))).toBe(true)
        expect(labels.some((l) => l?.includes('mère'))).toBe(true)
        // cliquer sur la pastille d'un parent rebascule en sablier pour le montrer
        fireEvent.click(me.querySelector('.tbadge[aria-label*="père"]')!)
        expect(useUI.getState().treeMode).toBe('hourglass')
    })

    it("utilise les icônes d'ajout et marque le bouton du formulaire comme enfoncé, nom prérempli", async () => {
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        const dad = (await screen.findByText('Jean LE GALL', {selector: 'text'})).closest('g.tnode') as SVGGElement
        const icons = [...dad.querySelectorAll('.tadd image')].map((i) => i.getAttribute('href') ?? '')
        expect(icons).toHaveLength(4)
        expect(icons.every((h) => /\.svg$|^data:image\/svg/.test(h))).toBe(true)
        fireEvent.click(dad.querySelector('[aria-label="Ajouter un père"]')!)
        // même effet que le bouton « + Père » du panneau : formulaire ouvert, bouton enfoncé, nom prérempli
        const side = screen.getByRole('complementary', {name: 'Personne sélectionnée'})
        const pressed = [...side.querySelectorAll('button[aria-pressed="true"]')].map((b) => b.textContent)
        expect(pressed).toEqual(['+ Père'])
        expect(screen.getByRole('textbox', {name: 'Nom'})).toHaveValue('Le Gall')
        expect(screen.getByRole('textbox', {name: 'Prénoms'})).toHaveFocus()
        // changer de relation déplace l'état enfoncé et vide le nom (une mère garde son nom de naissance)
        fireEvent.click(screen.getByRole('button', {name: '+ Mère'}))
        expect([...side.querySelectorAll('button[aria-pressed="true"]')].map((b) => b.textContent)).toEqual(['+ Mère'])
        expect(screen.getByRole('textbox', {name: 'Nom'})).toHaveValue('')
        // un nouveau clic sur le bouton enfoncé referme le formulaire
        fireEvent.click(screen.getByRole('button', {name: '+ Mère'}))
        expect(screen.queryByRole('form', {name: 'Nouvelle personne'})).not.toBeInTheDocument()
    })

    it('un glisser parti d\'une case déplace l\'arbre sans sélectionner la case ; un simple clic la sélectionne', async () => {
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        const {container} = renderTree()
        const dad = (await screen.findByText('Jean LE GALL', {selector: 'text'})).closest('g.tnode') as SVGGElement
        const svg = container.querySelector('svg')!
        const viewG = svg.querySelector(':scope > g') as SVGGElement
        const before = viewG.style.transform

        fireEvent.pointerDown(dad, {clientX: 100, clientY: 100, button: 0, pointerId: 1})
        fireEvent.pointerMove(svg, {clientX: 160, clientY: 130, pointerId: 1})
        fireEvent.pointerUp(svg, {clientX: 160, clientY: 130, pointerId: 1})
        fireEvent.click(dad) // clic synthétique qui suit un glisser
        expect(viewG.style.transform).not.toBe(before)
        expect(viewG).not.toHaveClass('animating') // un glisser à la souris n'est jamais animé
        expect(screen.queryByRole('complementary', {name: 'Personne sélectionnée'})).not.toBeInTheDocument()

        // simple clic (sans mouvement) : sélection
        fireEvent.pointerDown(dad, {clientX: 100, clientY: 100, button: 0, pointerId: 1})
        fireEvent.pointerUp(svg, {clientX: 100, clientY: 100, pointerId: 1})
        fireEvent.click(dad)
        expect(await screen.findByRole('complementary', {name: 'Personne sélectionnée'})).toBeInTheDocument()
    })

    it("observe la taille du conteneur même quand le premier rendu n'a pas encore de personne centrale", async () => {
        observed.length = 0
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        const {container} = renderTree()
        await screen.findByText('Yves LE GALL', {selector: 'text'})
        const canvas = container.querySelector('.tree-canvas')
        expect(canvas).not.toBeNull()
        expect(observed).toContain(canvas)
        // le SVG remplit le conteneur par CSS (pas de largeur figée en pixels)
        expect(container.querySelector('svg')!.hasAttribute('width')).toBe(false)
    })

    it("modifie la personne sélectionnée dans la zone du bas, avec ses onglets", async () => {
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        const zone = await screen.findByRole('region', {name: 'Modification de la personne sélectionnée'})
        expect(zone).toHaveTextContent("Sélectionnez une personne dans l'arbre pour la modifier")
        const dad = (await screen.findByText('Jean LE GALL', {selector: 'text'})).closest('g.tnode') as SVGGElement
        fireEvent.click(dad)
        // onglet « Informations » par défaut : dates et lieux modifiables
        expect(await screen.findByRole('textbox', {name: 'Date de naissance'})).toHaveValue('12 mars 1845')
        expect(screen.getByRole('textbox', {name: 'Date de décès'})).toHaveValue('3 mai 1920')
        expect(screen.getByRole('tab', {name: 'Informations'})).toHaveAttribute('aria-selected', 'true')
        expect(zone).toHaveTextContent('Sosa n° 4')
        fireEvent.click(screen.getByRole('tab', {name: 'Parents'}))
        expect(zone).toHaveTextContent('Aucun parent connu')
        fireEvent.click(screen.getByRole('tab', {name: 'Enfants'}))
        expect(screen.getByRole('form', {name: 'Ajouter un enfant'})).toBeInTheDocument()
    })

    it("ajoute un concubin avec le cœur placé à droite de la case : union libre, formulaire et bouton enfoncé", async () => {
        const user = userEvent.setup()
        useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
        renderTree()
        const dad = (await screen.findByText('Jean LE GALL', {selector: 'text'})).closest('g.tnode') as SVGGElement
        const heart = dad.querySelector('[aria-label="Ajouter un concubin"]') as SVGGElement
        // à droite de la case, à mi-hauteur
        expect(heart.getAttribute('transform')).toBe('translate(170 29)')
        const iconOf = (label: string) => dad.querySelector(`[aria-label="${label}"] image`)!.getAttribute('href')
        expect(iconOf('Ajouter un concubin')).toBeTruthy()
        expect(iconOf('Ajouter un concubin')).not.toBe(iconOf('Ajouter un enfant'))
        fireEvent.click(heart)
        const side = screen.getByRole('complementary', {name: 'Personne sélectionnée'})
        expect([...side.querySelectorAll('button[aria-pressed="true"]')].map((b) => b.textContent)).toEqual(['+ Concubin(e)'])
        expect(screen.getByRole('textbox', {name: 'Nom'})).toHaveValue('') // un concubin a son propre nom
        await user.type(screen.getByRole('textbox', {name: 'Prénoms'}), 'Marie')
        await user.click(screen.getByRole('button', {name: 'Créer'}))
        await waitFor(() => expect(ipc.createRelative).toHaveBeenCalled())
        const call = vi.mocked(ipc.createRelative).mock.calls.at(-1)!
        expect(call[0]).toBe('dad')
        expect(call[1]).toBe('partner')
    })

    describe('transitions lors du recentrage', () => {
        const caseOf = async (name: string) => (await screen.findByText(name, {selector: 'text'})).closest('g.tnode') as SVGGElement

        it('garde le même élément pour une personne quand l\'arbre est recentré : la case glisse au lieu d\'être recréée', async () => {
            useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
            renderTree()
            const dad = await caseOf('Jean LE GALL')
            const meBefore = await caseOf('Yves LE GALL')
            const before = meBefore.style.transform
            fireEvent.doubleClick(dad)
            await waitFor(() => expect(ipc.getTree).toHaveBeenCalledWith('dad', 4, 2))
            // Yves (qui était entre ses parents) est le MÊME élément après recentrage sur son père : il glisse
            // vers sa nouvelle place au lieu d'être recréé
            await waitFor(() => expect(meBefore.style.transform).not.toBe(before))
            expect(meBefore.isConnected).toBe(true)
            expect(await caseOf('Yves LE GALL')).toBe(meBefore)
            expect(await caseOf('Jean LE GALL')).toBe(dad)
        })

        it("garde la personne cliquée à sa place dans le repère de l'arbre : les autres glissent autour d'elle", async () => {
            useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
            renderTree()
            const dad = await caseOf('Jean LE GALL')
            const before = dad.style.transform
            fireEvent.doubleClick(dad)
            await waitFor(() => expect(dad).toHaveClass('selected'))
            expect(dad.style.transform).toBe(before)
        })

        it('sélectionne la nouvelle personne centrale, la met en évidence et anime la caméra', async () => {
            useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
            const {container} = renderTree()
            const dad = await caseOf('Jean LE GALL')
            expect(dad).not.toHaveClass('selected')
            fireEvent.doubleClick(dad)
            await waitFor(() => expect(dad).toHaveClass('selected'))
            expect(dad).toHaveClass('focus')
            expect(dad).toHaveClass('arrived') // pulsation d'arrivée
            // la caméra se déplace avec une transition
            expect(container.querySelector('svg > g')).toHaveClass('animating')
            // le panneau montre la personne arrivée
            expect(screen.getByRole('complementary', {name: 'Personne sélectionnée'})).toHaveTextContent('Jean LE GALL')
            // la mise en évidence est temporaire
            await waitFor(() => expect(dad).not.toHaveClass('arrived'), {timeout: 3000})
        })

        it('une pastille « enfants cachés » recentre avec la même transition et sélectionne la personne', async () => {
            useUI.setState({treeFocus: null, treeMode: 'ancestors', treeUp: 4, treeDown: 2})
            renderTree()
            const dad = await caseOf('Jean LE GALL')
            // en mode ascendants, dad a 1 enfant en base (childCount) mais 1 dessiné (me) : pas de pastille ;
            // Yves en a 3 dont 0 dessiné en mode ascendants
            const me = await caseOf('Yves LE GALL')
            fireEvent.click(me.querySelector('.tbadge[aria-label*="enfants"]')!)
            expect(useUI.getState().treeMode).toBe('hourglass')
            await waitFor(() => expect(me).toHaveClass('selected'))
            expect(dad.isConnected).toBe(true)
        })

        it("ne change pas la sélection quand on modifie seulement la profondeur de l'arbre", async () => {
            const user = userEvent.setup()
            useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
            renderTree()
            const dad = await caseOf('Jean LE GALL')
            fireEvent.click(dad)
            expect(dad).toHaveClass('selected')
            await user.click(screen.getByRole('button', {name: 'Plus de descendants'}))
            await waitFor(() => expect(ipc.getTree).toHaveBeenLastCalledWith('me', 4, 3))
            expect(await caseOf('Jean LE GALL')).toHaveClass('selected')
        })

        it('conserve l\'ancien arbre à l\'écran pendant le chargement du nouveau (pas de blanc)', async () => {
            useUI.setState({treeFocus: null, treeMode: 'hourglass', treeUp: 4, treeDown: 2})
            renderTree()
            const dad = await caseOf('Jean LE GALL')
            let release: () => void = () => {}
            vi.mocked(ipc.getTree).mockImplementationOnce(() => new Promise((resolve) => {
                release = () => resolve({...tree, focus: 'dad'} as never)
            }))
            fireEvent.doubleClick(dad)
            await waitFor(() => expect(ipc.getTree).toHaveBeenCalledWith('dad', 4, 2))
            // chargement en cours : toujours des cases à l'écran
            expect(document.querySelectorAll('g.tnode').length).toBeGreaterThan(0)
            expect(dad.isConnected).toBe(true)
            release()
            await waitFor(() => expect(dad).toHaveClass('selected'))
        })
    })
})
