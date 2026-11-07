import {describe, expect, it} from 'vitest'
import {BOX_H, BOX_W, buildLayout, childIds, parentIds, type TreeInput} from './layout'

let seq = 0
function person(id: string, sex = 'M') {
    return {person: {id, surname: id.toUpperCase(), givenNames: [id], sex, living: 'unknown'}, birth: '', death: ''}
}

/** Petit jeu : gp(grand-père)+gm → dad ; mum (sans parents) ; dad+mum → me, sis ; me + wife → kid1, kid2 ; kid1 + kw → gk. */
function family(): TreeInput {
    const link = (c: string, p: string, role: string, unionId = '', kind = 'biological') =>
        ({id: `l${seq++}`, childId: c, parentId: p, role, kind, unionId})
    return {
        nodes: ['gp', 'gm', 'dad', 'mum', 'me', 'sis', 'wife', 'kid1', 'kid2', 'kw', 'gk'].map((i) => person(i, ['gm', 'mum', 'sis', 'wife', 'kw'].includes(i) ? 'F' : 'M')),
        unions: [
            {id: 'u0', type: 'marriage', status: 'ended', partners: ['gp', 'gm'], year: '1900'},
            {id: 'u1', type: 'marriage', status: 'ongoing', partners: ['dad', 'mum'], year: '1930'},
            {id: 'u2', type: 'marriage', status: 'ongoing', partners: ['me', 'wife'], year: '1960'},
            {id: 'u3', type: 'marriage', status: 'ongoing', partners: ['kid1', 'kw'], year: '1990'},
        ],
        links: [
            link('dad', 'gp', 'father', 'u0'), link('dad', 'gm', 'mother', 'u0'),
            link('me', 'dad', 'father', 'u1'), link('me', 'mum', 'mother', 'u1'),
            link('sis', 'dad', 'father'), link('sis', 'mum', 'mother'), // pas d'union_id : déduite
            link('kid1', 'me', 'father', 'u2'), link('kid1', 'wife', 'mother', 'u2'),
            link('kid2', 'me', 'father', 'u2'), link('kid2', 'wife', 'mother', 'u2'),
            link('gk', 'kid1', 'father', 'u3'), link('gk', 'kw', 'mother', 'u3'),
        ],
    }
}

const byId = (l: ReturnType<typeof buildLayout>, id: string) => l.nodes.filter((n) => n.id === id)
const center = (n: { x: number }) => n.x + BOX_W / 2

describe('buildLayout', () => {
    it("n'a aucun recouvrement et aucune case dupliquée par clé", () => {
        for (const mode of ['hourglass', 'ancestors', 'descendants'] as const) {
            const l = buildLayout(family(), 'me', mode, 3, 3)
            expect(new Set(l.nodes.map((n) => n.key)).size).toBe(l.nodes.length)
            const rows = new Map<number, number[]>()
            l.nodes.forEach((n) => rows.set(n.y, [...(rows.get(n.y) ?? []), n.x]))
            rows.forEach((xs) => {
                const s = [...xs].sort((a, b) => a - b)
                for (let i = 1; i < s.length; i++) expect(s[i] - s[i - 1]).toBeGreaterThanOrEqual(BOX_W)
            })
            expect(l.nodes.every((n) => n.x >= 0 && n.y >= 0)).toBe(true)
            expect(l.width).toBeGreaterThan(0)
        }
    })

    it('ancêtres : parents au-dessus, centrés sur leur enfant, numéros Sosa', () => {
        const l = buildLayout(family(), 'me', 'ancestors', 3, 0)
        const me = byId(l, 'me')[0]
        const dad = byId(l, 'dad')[0]
        const mum = byId(l, 'mum')[0]
        expect(me.sosa).toBe(1)
        expect(dad.sosa).toBe(2)
        expect(mum.sosa).toBe(3)
        expect(dad.y).toBeLessThan(me.y)
        expect(dad.y).toBe(mum.y)
        expect(dad.x).toBeLessThan(mum.x)
        expect(center(me)).toBeCloseTo((center(dad) + center(mum)) / 2, 5)
        expect(byId(l, 'gp')[0].sosa).toBe(4)
        expect(byId(l, 'gm')[0].sosa).toBe(5)
        expect(byId(l, 'gp')[0].y).toBeLessThan(dad.y)
        expect(l.nodes.some((n) => n.id === 'kid1')).toBe(false)
    })

    it('limite le nombre de générations', () => {
        const l = buildLayout(family(), 'me', 'ancestors', 1, 0)
        expect(l.nodes.map((n) => n.id).sort()).toEqual(['dad', 'me', 'mum'])
    })

    it('descendants : conjoint à côté, enfants sous le couple, petit-enfant plus bas', () => {
        const l = buildLayout(family(), 'me', 'descendants', 0, 3)
        const me = byId(l, 'me')[0]
        const wife = byId(l, 'wife')[0]
        expect(wife.y).toBe(me.y)
        expect(wife.x - me.x).toBeGreaterThanOrEqual(BOX_W)
        const k1 = byId(l, 'kid1')[0]
        const k2 = byId(l, 'kid2')[0]
        expect(k1.y).toBeGreaterThan(me.y)
        expect(k1.y).toBe(k2.y)
        expect((center(k1) + center(k2)) / 2).toBeCloseTo((center(me) + center(wife)) / 2, 5)
        const gk = byId(l, 'gk')[0]
        expect(gk.y).toBeGreaterThan(k1.y)
        expect(byId(l, 'kw')).toHaveLength(1)
        // pas d'ancêtres
        expect(l.nodes.some((n) => n.id === 'dad')).toBe(false)
        expect(l.nodes.filter((n) => n.isFocus)).toHaveLength(1)
    })

    it('sablier : le focus est aligné sous ses parents, un seul exemplaire', () => {
        const l = buildLayout(family(), 'me', 'hourglass', 2, 2)
        expect(byId(l, 'me')).toHaveLength(1)
        const me = byId(l, 'me')[0]
        const dad = byId(l, 'dad')[0]
        const mum = byId(l, 'mum')[0]
        expect(center(me)).toBeCloseTo((center(dad) + center(mum)) / 2, 5)
        expect(dad.y).toBeLessThan(me.y)
        expect(byId(l, 'kid1')[0].y).toBeGreaterThan(me.y)
        expect(me.isFocus).toBe(true)
        expect(l.height).toBeGreaterThanOrEqual(4 * BOX_H)
    })

    it('déduit l\'union des parents quand le lien n\'en porte pas, et groupe la fratrie', () => {
        const l = buildLayout(family(), 'dad', 'descendants', 0, 1)
        expect(byId(l, 'me')).toHaveLength(1)
        expect(byId(l, 'sis')).toHaveLength(1)
        expect(byId(l, 'me')[0].y).toBe(byId(l, 'sis')[0].y)
    })

    it('gère un focus inconnu et une personne isolée', () => {
        expect(buildLayout(family(), 'nobody', 'hourglass', 3, 3).nodes).toHaveLength(0)
        const alone = buildLayout({nodes: [person('x')], links: [], unions: []}, 'x', 'hourglass', 3, 3)
        expect(alone.nodes).toHaveLength(1)
        expect(alone.height).toBe(BOX_H)
    })

    it('dessine les implexes à chaque occurrence', () => {
        // a a deux parents b et c, qui ont tous deux le même père d
        const d: TreeInput = {
            nodes: ['a', 'b', 'c', 'd'].map((i) => person(i)),
            unions: [],
            links: [
                {id: '1', childId: 'a', parentId: 'b', role: 'father', kind: 'biological', unionId: ''},
                {id: '2', childId: 'a', parentId: 'c', role: 'mother', kind: 'biological', unionId: ''},
                {id: '3', childId: 'b', parentId: 'd', role: 'father', kind: 'biological', unionId: ''},
                {id: '4', childId: 'c', parentId: 'd', role: 'father', kind: 'biological', unionId: ''},
            ],
        }
        const l = buildLayout(d, 'a', 'ancestors', 3, 0)
        expect(byId(l, 'd').map((n) => n.sosa).sort()).toEqual([4, 6])
    })

    it('ne boucle pas sur des données cycliques', () => {
        const d: TreeInput = {
            nodes: [person('a'), person('b')], unions: [],
            links: [
                {id: '1', childId: 'a', parentId: 'b', role: 'father', kind: 'biological', unionId: ''},
                {id: '2', childId: 'b', parentId: 'a', role: 'father', kind: 'biological', unionId: ''},
            ],
        }
        const l = buildLayout(d, 'a', 'hourglass', 5, 5)
        expect(l.nodes.length).toBeLessThanOrEqual(12)
    })

    it('expose les parents et enfants', () => {
        const f = family()
        expect(parentIds(f, 'me')).toEqual(['dad', 'mum'])
        expect(childIds(f, 'me').sort()).toEqual(['kid1', 'kid2'])
        expect(childIds(f, 'dad').sort()).toEqual(['me', 'sis'])
    })

    it('indique les parents et enfants réellement dessinés sous / sur chaque case', () => {
        const byKey = (lay: ReturnType<typeof buildLayout>, id: string) => lay.nodes.find((n) => n.id === id)!
        // sablier : le focus voit ses 2 parents, ses 2 enfants ; ses parents ne voient qu'un enfant (le chemin vers lui)
        const h = buildLayout(family(), 'me', 'hourglass', 2, 1)
        expect(byKey(h, 'me').shownParents.sort()).toEqual(['dad', 'mum'])
        expect(byKey(h, 'me').shownChildren).toBe(2)
        expect(byKey(h, 'dad').shownChildren).toBe(1)
        expect(byKey(h, 'dad').shownParents.sort()).toEqual(['gm', 'gp'])
        expect(byKey(h, 'kid1').shownChildren).toBe(0) // profondeur 1 : les petits-enfants ne sont pas dessinés
        expect(byKey(h, 'kid1').shownParents.sort()).toEqual(['me', 'wife'])
        // le conjoint voit les enfants de son union, pas ses parents
        expect(byKey(h, 'wife').shownChildren).toBe(2)
        expect(byKey(h, 'wife').shownParents).toEqual([])

        // ascendants seuls : pas d'enfant dessiné sous le focus
        const a = buildLayout(family(), 'me', 'ancestors', 1, 0)
        expect(byKey(a, 'me').shownChildren).toBe(0)
        expect(byKey(a, 'dad').shownParents).toEqual([]) // limite de générations
        // descendants seuls : le focus ne montre aucun parent
        const d = buildLayout(family(), 'me', 'descendants', 0, 1)
        expect(byKey(d, 'me').shownParents).toEqual([])
        expect(byKey(d, 'me').shownChildren).toBe(2)
    })
})
