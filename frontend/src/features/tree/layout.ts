// Mise en page des arbres généalogiques (fonction pure, sans dépendance UI).
//
// - ancêtres : pedigree vertical, la personne focus en bas, chaque parent centré
//   au-dessus de son enfant ; les implexes (même ancêtre à plusieurs endroits)
//   sont dessinés à chaque occurrence ;
// - descendants : chaque personne est suivie de son ou ses conjoints, les enfants
//   de chaque union pendent sous le trait d'union du couple ;
// - sablier : ancêtres au-dessus, descendants en dessous, alignés sur le focus.

export const BOX_W = 170
export const BOX_H = 58
export const GAP_X = 18
export const GAP_Y = 54
const COUPLE_DROP = 8

export type Mode = 'hourglass' | 'ancestors' | 'descendants'

export interface TreePerson {
    id: string
    surname: string
    givenNames: string[]
    sex: string
    living: string
}

export interface TreeInput {
    nodes: { person: TreePerson; birth: string; death: string }[]
    links: { id: string; childId: string; parentId: string; role: string; kind: string; unionId: string }[]
    unions: { id: string; type: string; status: string; partners: string[]; year: string }[]
}

export interface LNode {
    key: string
    id: string
    x: number // coin supérieur gauche (px)
    y: number
    sosa?: number // numéro relatif au focus (ancêtres uniquement)
    isFocus: boolean
    /** Parents effectivement dessinés au-dessus de cette case (identifiants de personnes). */
    shownParents: string[]
    /** Nombre d'enfants effectivement dessinés sous cette case. */
    shownChildren: number
}

export interface LEdge {
    d: string
    kind: 'child' | 'couple'
}

export interface Layout {
    nodes: LNode[]
    edges: LEdge[]
    width: number
    height: number
}

const STEP = BOX_W + GAP_X
const ROW = BOX_H + GAP_Y

interface Parents {
    father?: string
    mother?: string
}

class Graph {
    byId = new Map<string, TreeInput['nodes'][number]>()
    linksByChild = new Map<string, TreeInput['links']>()
    childrenOfUnion = new Map<string, string[]>()
    soloChildren = new Map<string, string[]>()
    unionsOf = new Map<string, TreeInput['unions']>()
    unionById = new Map<string, TreeInput['unions'][number]>()

    constructor(d: TreeInput) {
        d.nodes.forEach((n) => this.byId.set(n.person.id, n))
        d.links.forEach((l) => {
            const a = this.linksByChild.get(l.childId) ?? []
            a.push(l)
            this.linksByChild.set(l.childId, a)
        })
        d.unions.forEach((u) => {
            this.unionById.set(u.id, u)
            u.partners.forEach((p) => {
                const a = this.unionsOf.get(p) ?? []
                a.push(u)
                this.unionsOf.set(p, a)
            })
        })
        // rattache chaque enfant à une union (explicite, ou déduite des deux parents) ou à un parent seul
        const seen = new Set<string>()
        d.links.forEach((l) => {
            if (seen.has(l.childId)) return
            seen.add(l.childId)
            const links = this.linksByChild.get(l.childId) ?? []
            let uid = links.find((x) => x.unionId)?.unionId ?? ''
            if (!uid) {
                const ps = new Set(links.map((x) => x.parentId))
                uid = d.unions.find((u) => u.partners.filter((p) => ps.has(p)).length >= 2)?.id ?? ''
            }
            if (uid) {
                const a = this.childrenOfUnion.get(uid) ?? []
                a.push(l.childId)
                this.childrenOfUnion.set(uid, a)
            } else {
                // enfant d'un parent sans union : rattaché à chacun de ses parents
                links.forEach((x) => {
                    const a = this.soloChildren.get(x.parentId) ?? []
                    if (!a.includes(l.childId)) a.push(l.childId)
                    this.soloChildren.set(x.parentId, a)
                })
            }
        })
    }

    parentsOf(id: string): Parents {
        let links = this.linksByChild.get(id) ?? []
        const bio = links.filter((l) => l.kind === 'biological' || l.kind === 'unknown')
        if (bio.length) links = bio
        const p: Parents = {}
        links.filter((l) => l.role === 'father').forEach((l) => { p.father ??= l.parentId })
        links.filter((l) => l.role === 'mother').forEach((l) => { p.mother ??= l.parentId })
        links.filter((l) => l.role === 'parent').forEach((l) => {
            if (!p.father && p.mother !== l.parentId) p.father = l.parentId
            else if (!p.mother && p.father !== l.parentId) p.mother = l.parentId
        })
        return p
    }

    /** Tous les enfants d'une personne (toutes unions confondues). */
    childrenOf(id: string): string[] {
        const out: string[] = []
        ;(this.unionsOf.get(id) ?? []).forEach((u) => (this.childrenOfUnion.get(u.id) ?? []).forEach((c) => out.push(c)))
        ;(this.soloChildren.get(id) ?? []).forEach((c) => out.push(c))
        return [...new Set(out)]
    }
}

export function parentIds(data: TreeInput, id: string): string[] {
    const p = new Graph(data).parentsOf(id)
    return [p.father, p.mother].filter((x): x is string => !!x)
}

export function childIds(data: TreeInput, id: string): string[] {
    return new Graph(data).childrenOf(id)
}

// --- ancêtres ---------------------------------------------------------------

interface ATree {
    id: string
    sosa: number
    gen: number
    father?: ATree
    mother?: ATree
}

function buildA(g: Graph, id: string, sosa: number, gen: number, up: number): ATree {
    const t: ATree = {id, sosa, gen}
    if (gen < up) {
        const p = g.parentsOf(id)
        if (p.father && g.byId.has(p.father)) t.father = buildA(g, p.father, sosa * 2, gen + 1, up)
        if (p.mother && g.byId.has(p.mother)) t.mother = buildA(g, p.mother, sosa * 2 + 1, gen + 1, up)
    }
    return t
}

const widthA = (t: ATree): number =>
    !t.father && !t.mother ? 1 : (t.father ? widthA(t.father) : 0) + (t.mother ? widthA(t.mother) : 0)

const depthA = (t: ATree): number => Math.max(t.gen, t.father ? depthA(t.father) : 0, t.mother ? depthA(t.mother) : 0)

interface Sink {
    nodes: LNode[]
    edges: LEdge[]
    maxX: number
    maxRow: number
}

const newSink = (): Sink => ({nodes: [], edges: [], maxX: 0, maxRow: 0})

function box(sink: Sink, key: string, id: string, centerUnit: number, row: number, extra: Partial<LNode> = {}) {
    const x = centerUnit * STEP - BOX_W / 2 + GAP_X / 2
    sink.nodes.push({key, id, x, y: row * ROW, isFocus: false, shownParents: [], shownChildren: 0, ...extra})
    sink.maxX = Math.max(sink.maxX, x + BOX_W)
    sink.maxRow = Math.max(sink.maxRow, row)
    return x + BOX_W / 2
}

const cx = (unit: number) => unit * STEP + GAP_X / 2

/** Relie des parents (positions en px) à un enfant (centre haut en px). */
function parentsToChild(sink: Sink, parentXs: number[], parentRow: number, childX: number, childRow: number) {
    const bottom = parentRow * ROW + BOX_H
    const midY = bottom + GAP_Y / 2 + COUPLE_DROP / 2
    let ax = parentXs[0]
    let ay = bottom
    if (parentXs.length === 2) {
        const [a, b] = parentXs
        ay = bottom + COUPLE_DROP
        sink.edges.push({kind: 'couple', d: `M${a} ${bottom} V${ay} H${b} V${bottom}`})
        ax = (a + b) / 2
    }
    sink.edges.push({kind: 'child', d: `M${ax} ${ay} V${midY} H${childX} V${childRow * ROW}`})
}

function placeA(sink: Sink, t: ATree, left: number, depth: number, skipRoot: boolean): number {
    const row = depth - t.gen
    let center: number // en unités
    const childPx = () => cx(center)
    if (!t.father && !t.mother) {
        center = left + 0.5
    } else {
        let l = left
        const xs: number[] = []
        for (const p of [t.father, t.mother]) {
            if (!p) continue
            const pc = placeA(sink, p, l, depth, false)
            xs.push(pc)
            l += widthA(p)
        }
        center = xs.reduce((a, b) => a + b, 0) / xs.length
        parentsToChild(sink, xs.map(cx), row - 1, childPx(), row)
    }
    if (!skipRoot || t.gen > 0) {
        const shownParents = [t.father?.id, t.mother?.id].filter((x): x is string => !!x)
        // chaque ancêtre n'a, dessiné sous lui, que l'enfant qui mène au focus
        box(sink, `a:${t.sosa}`, t.id, center, row, {sosa: t.sosa, isFocus: t.gen === 0, shownParents, shownChildren: t.gen > 0 ? 1 : 0})
    } else {
        sink.maxRow = Math.max(sink.maxRow, row)
    }
    return center
}

// --- descendants ------------------------------------------------------------

interface DGroup {
    spouse?: string
    children: DUnit[]
}

interface DUnit {
    id: string
    key: string
    groups: DGroup[]
}

function buildD(g: Graph, id: string, gen: number, down: number, key: string): DUnit {
    const groups: DGroup[] = []
    const expand = gen < down
    ;(g.unionsOf.get(id) ?? []).forEach((u) => {
        const spouse = u.partners.find((p) => p !== id && g.byId.has(p))
        const kids = expand ? (g.childrenOfUnion.get(u.id) ?? []) : []
        groups.push({
            spouse,
            children: kids.filter((c) => g.byId.has(c)).map((c, i) => buildD(g, c, gen + 1, down, `${key}.${groups.length}-${i}`)),
        })
    })
    const solo = expand ? (g.soloChildren.get(id) ?? []).filter((c) => g.byId.has(c)) : []
    if (solo.length) groups.push({children: solo.map((c, i) => buildD(g, c, gen + 1, down, `${key}.s-${i}`))})
    return {id, key, groups}
}

const rowWidthD = (u: DUnit) => 1 + u.groups.filter((g) => g.spouse).length
const groupWidthD = (g: DGroup) => g.children.reduce((s, c) => s + widthD(c), 0)
const childrenWidthD = (u: DUnit) => u.groups.reduce((s, g) => s + groupWidthD(g), 0)
const widthD = (u: DUnit): number => Math.max(rowWidthD(u), childrenWidthD(u))

/** Largeur (unités) et centre de la case de la personne, pour un placement à gauche = 0. */
function placeD(sink: Sink, u: DUnit, left: number, row: number, isFocus: boolean, parents: string[]): number {
    const w = widthD(u)
    const rowW = rowWidthD(u)
    const rowLeft = left + (w - rowW) / 2
    let childLeft = left + (w - childrenWidthD(u)) / 2
    const personC = rowLeft + 0.5
    box(sink, `d:${u.key}`, u.id, personC, row, {
        isFocus, shownParents: parents, shownChildren: u.groups.reduce((n, g) => n + g.children.length, 0),
    })
    let si = 0
    for (const grp of u.groups) {
        const bottom = row * ROW + BOX_H
        let ax = cx(personC)
        let ay = bottom
        if (grp.spouse) {
            si++
            const sc = rowLeft + si + 0.5
            box(sink, `s:${u.key}:${si}`, grp.spouse, sc, row, {shownChildren: grp.children.length})
            ay = bottom + COUPLE_DROP
            sink.edges.push({kind: 'couple', d: `M${cx(personC)} ${bottom} V${ay} H${cx(sc)} V${bottom}`})
            ax = (cx(personC) + cx(sc)) / 2
        }
        const midY = bottom + GAP_Y / 2 + COUPLE_DROP / 2
        for (const child of grp.children) {
            const cc = placeD(sink, child, childLeft, row + 1, false, grp.spouse ? [u.id, grp.spouse] : [u.id])
            sink.edges.push({kind: 'child', d: `M${ax} ${ay} V${midY} H${cx(cc)} V${(row + 1) * ROW}`})
            childLeft += widthD(child)
        }
    }
    return personC
}

// --- assemblage -------------------------------------------------------------

export function buildLayout(data: TreeInput, focus: string, mode: Mode, up: number, down: number): Layout {
    const g = new Graph(data)
    if (!g.byId.has(focus)) return {nodes: [], edges: [], width: 0, height: 0}

    const wantA = mode !== 'descendants'
    const wantD = mode !== 'ancestors'
    const A = wantA ? buildA(g, focus, 1, 0, up) : null
    const D = wantD ? buildD(g, focus, 0, down, 'f') : null
    const depth = A ? depthA(A) : 0
    const hasParents = !!A && (!!A.father || !!A.mother)

    const focusParents = hasParents ? [A!.father?.id, A!.mother?.id].filter((x): x is string => !!x) : []

    // alignement : le focus doit tomber sur la même colonne dans les deux moitiés
    let leftA = 0
    let leftD = 0
    if (A && D && hasParents) {
        const xa = placeA(newSink(), A, 0, depth, true)
        const xd = placeD(newSink(), D, 0, depth, true, focusParents)
        if (xa > xd) leftD = xa - xd
        else leftA = xd - xa
    }

    const sink = newSink()
    if (D) {
        placeD(sink, D, leftD, hasParents ? depth : 0, true, focusParents)
        if (A && hasParents) placeA(sink, A, leftA, depth, true)
    } else if (A) {
        placeA(sink, A, leftA, depth, false)
    }
    return {nodes: sink.nodes, edges: sink.edges, width: sink.maxX, height: (sink.maxRow + 1) * ROW - GAP_Y}
}
