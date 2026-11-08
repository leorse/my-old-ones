import {useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent} from 'react'
import {useTranslation} from 'react-i18next'
import {keepPreviousData, useQuery} from '@tanstack/react-query'
import {ipc} from '../../ipc'
import {personLabel} from '../../components/helpers'
import {usePersons} from '../../queries'
import {useUI, type TreeMode} from '../../store/ui'
import {QuickAdd, type Adding} from '../persons/FamilySection'
import {defaultSurnameFor} from '../persons/surname'
import manIcon from '../../assets/icons/man.svg'
import womanIcon from '../../assets/icons/woman.svg'
import childIcon from '../../assets/icons/child.svg'
import broIcon from '../../assets/icons/bro.svg'
import heartIcon from '../../assets/icons/heart.svg'
import {PersonEditor} from './PersonEditor'
import {BOX_H, BOX_W, buildLayout, childIds, parentIds, type LEdge, type Layout, type LNode} from './layout'

const SEX_COLOR: Record<string, string> = {M: '#3b82f6', F: '#ec4899', other: '#a855f7', unknown: '#9ca3af'}
const MODES: TreeMode[] = ['hourglass', 'ancestors', 'descendants']
const MIN_K = 0.1
const MAX_K = 2.5

interface View {
    x: number
    y: number
    k: number
}

function years(birth: string, death: string, living: string): string {
    if (!birth && !death) return ''
    if (death) return `${birth || '?'} – ${death}`
    return living === 'deceased' ? `${birth || '?'} – ?` : `${birth} –`
}

type AddRel = 'father' | 'mother' | 'sibling' | 'child' | 'partner'

const ICONS: Record<AddRel, string> = {father: manIcon, mother: womanIcon, sibling: broIcon, child: childIcon, partner: heartIcon}

const ADD_R = 12
// Position des boutons d'ajout : à cheval sur le bord de la case, pour que le survol reste continu.
const ADD_POS: Record<AddRel, [number, number]> = {
    father: [BOX_W * 0.27, 0], mother: [BOX_W * 0.73, 0], sibling: [0, BOX_H / 2], child: [BOX_W * 0.62, BOX_H], partner: [BOX_W, BOX_H / 2],
}

/** Bouton rond (icône du proche à ajouter) autour d'une case ; apparaît au survol ou à la sélection. */
function AddButton({rel, label, onAdd}: { rel: AddRel; label: string; onAdd: () => void }) {
    const [x, y] = ADD_POS[rel]
    return (
        <g className="tadd" role="button" aria-label={label} transform={`translate(${x} ${y})`}
           onClick={(e) => { e.stopPropagation(); onAdd() }} onDoubleClick={(e) => e.stopPropagation()}>
            <title>{label}</title>
            <circle r={ADD_R}/>
            <image href={ICONS[rel]} x={-8} y={-8} width={16} height={16}/>
        </g>
    )
}

type BadgeKind = 'father' | 'mother' | 'child'
const BADGE_W = 30
const BADGE_H = 40
const BADGE_POS: Record<BadgeKind, [number, number]> = {
    father: [BOX_W * 0.27 - BADGE_W / 2, -BADGE_H - 5],
    mother: [BOX_W * 0.73 - BADGE_W / 2, -BADGE_H - 5],
    child: [BOX_W * 0.2 - BADGE_W / 2, BOX_H + 5],
}

/** Petit panneau « il y en a, mais on ne les voit pas » : icône du proche et nombre. */
function HiddenBadge({kind, count, label, onClick}: { kind: BadgeKind; count: number; label: string; onClick: () => void }) {
    const [x, y] = BADGE_POS[kind]
    return (
        <g className="tbadge" role="button" aria-label={label} transform={`translate(${x} ${y})`}
           onClick={(e) => { e.stopPropagation(); onClick() }} onDoubleClick={(e) => e.stopPropagation()}>
            <title>{label}</title>
            <rect width={BADGE_W} height={BADGE_H} rx={6}/>
            <image href={ICONS[kind]} x={5} y={3} width={20} height={20}/>
            <text x={BADGE_W / 2} y={35} textAnchor="middle">{count}</text>
        </g>
    )
}

/**
 * Remplace les clés de mise en page (liées à la position dans l'arbre) par des clés liées à la PERSONNE.
 * Ainsi, quand l'arbre est recentré, les cases qui restent à l'écran glissent vers leur nouvelle place
 * au lieu d'être détruites puis recréées. Le centre de l'arbre garde la clé « pure » ; une personne dessinée
 * plusieurs fois (implexe) reçoit « #2 », « #3 »…
 */
function withPersonKeys(l: Layout): Layout {
    const seen = new Map<string, number>()
    const keyOf = new Map<LNode, string>()
    for (const n of [...l.nodes].sort((a, b) => Number(b.isFocus) - Number(a.isFocus))) {
        const c = seen.get(n.id) ?? 0
        seen.set(n.id, c + 1)
        keyOf.set(n, c === 0 ? n.id : `${n.id}#${c}`)
    }
    return {...l, nodes: l.nodes.map((n) => ({...n, key: keyOf.get(n)!}))}
}

const ANIM_MS = 520

/** Mise en page affichée, exprimée dans un repère « monde » continu d'une mise en page à la suivante. */
type Shown = Layout & { ox: number; oy: number }

function truncate(s: string, n: number) {
    return s.length > n ? s.slice(0, n - 1) + '…' : s
}

export function TreeView() {
    const {t} = useTranslation()
    const treeFocus = useUI((s) => s.treeFocus)
    const mode = useUI((s) => s.treeMode)
    const up = useUI((s) => s.treeUp)
    const down = useUI((s) => s.treeDown)
    const setTreeFocus = useUI((s) => s.setTreeFocus)
    const setMode = useUI((s) => s.setTreeMode)
    const setGens = useUI((s) => s.setTreeGenerations)
    const openPerson = useUI((s) => s.selectPerson)

    const persons = usePersons()
    const root = useQuery({queryKey: ['root'], queryFn: () => ipc.getRoot()})
    const rootId = root.data || ''
    const focus = treeFocus ?? (rootId || persons.data?.[0]?.id) ?? null

    const qUp = mode === 'descendants' ? 0 : up
    const qDown = mode === 'ancestors' ? 0 : down
    const tree = useQuery({
        queryKey: ['tree', focus, qUp, qDown],
        queryFn: () => ipc.getTree(focus!, qUp, qDown),
        enabled: !!focus,
        placeholderData: keepPreviousData,
    })
    // le centre mémorisé n'existe plus (autre projet, personne supprimée) : retour à la racine
    useEffect(() => {
        if (tree.isError && treeFocus) setTreeFocus(null)
    }, [tree.isError, treeFocus, setTreeFocus])
    const data = tree.isError ? undefined : tree.data
    // `fresh` : mise en page de la personne demandée ; null tant que ses données se chargent.
    // On continue alors d'afficher la précédente (pas de blanc) jusqu'à l'arrivée de la nouvelle, qui s'animera.
    const fresh = useMemo(
        () => (data && focus && data.focus === focus ? buildLayout(data, focus, mode, qUp, qDown) : null),
        [data, focus, mode, qUp, qDown],
    )
    // Repère continu : chaque mise en page est calculée autour de son centre, donc deux mises en page successives
    // n'ont pas la même origine. On décale la nouvelle pour qu'une personne présente dans les deux (le nouveau centre
    // si possible) reste exactement où elle était : les autres cases qui existaient déjà glissent alors seulement
    // pour faire place aux nouvelles, au lieu de donner l'impression d'un autre arbre.
    const shown = useRef<Shown | null>(null)
    const layout = useMemo<Shown | null>(() => {
        if (!fresh) return shown.current
        const keyed = withPersonKeys(fresh)
        const old = shown.current
        let ox = 0, oy = 0
        if (old) {
            const oldByKey = new Map(old.nodes.map((n) => [n.key, n]))
            const anchor = keyed.nodes.find((n) => n.isFocus && oldByKey.has(n.key)) ?? keyed.nodes.find((n) => oldByKey.has(n.key))
            if (anchor) {
                const o = oldByKey.get(anchor.key)!
                ox = o.x - anchor.x
                oy = o.y - anchor.y
            }
        }
        const next: Shown = {...keyed, nodes: keyed.nodes.map((n) => ({...n, x: n.x + ox, y: n.y + oy})), ox, oy}
        shown.current = next
        return next
    }, [fresh])
    const layoutSeq = useRef(0)
    const seq = useMemo(() => ++layoutSeq.current, [layout]) // eslint-disable-line react-hooks/exhaustive-deps
    const people = useMemo(() => new Map((data?.nodes ?? []).map((n) => [n.person.id, n])), [data])
    const bioParents = useMemo(() => {
        const m = new Map<string, { father?: string; mother?: string }>()
        for (const l of data?.links ?? []) {
            if (l.kind !== 'biological' || (l.role !== 'father' && l.role !== 'mother')) continue
            const e = m.get(l.childId) ?? {}
            e[l.role] = l.parentId
            m.set(l.childId, e)
        }
        return m
    }, [data])
    /** Conjoint connu de la personne (de préférence un homme : il donne le nom aux enfants). */
    const partnerOf = (id: string) => {
        let found: { surname: string; sex: string } | undefined
        for (const u of data?.unions ?? []) {
            if (!u.partners.includes(id)) continue
            for (const pid of u.partners) {
                const q = pid !== id ? people.get(pid)?.person : undefined
                if (q && (!found || q.sex === 'M')) found = q
            }
        }
        return found
    }

    // --- sélection ---
    const [selectedKey, setSelectedKey] = useState<string | null>(null)
    const [adding, setAdding] = useState<Adding | null>(null)
    const selectedNode = layout?.nodes.find((n) => n.key === selectedKey) ?? null
    const selected = selectedNode ? people.get(selectedNode.id) : undefined

    // --- zoom / déplacement ---
    // Le conteneur n'existe qu'une fois la personne centrale connue : on l'observe via une ref d'état,
    // sinon l'observateur ne serait jamais attaché (taille figée) si le premier rendu n'a pas de conteneur.
    const [boxEl, setBoxEl] = useState<HTMLDivElement | null>(null)
    const svg = useRef<SVGSVGElement>(null)
    const [view, setView] = useState<View>({x: 20, y: 20, k: 1})
    const viewRef = useRef(view)
    viewRef.current = view
    const [size, setSize] = useState({w: 800, h: 600})
    const initialized = useRef(false)

    useLayoutEffect(() => {
        if (!boxEl) return
        const measure = () => setSize({w: boxEl.clientWidth, h: boxEl.clientHeight})
        const ro = new ResizeObserver(measure)
        ro.observe(boxEl)
        measure()
        return () => ro.disconnect()
    }, [boxEl])

    // Déplacements de la caméra programmés (recentrage, ajuster, zoom +/-) : animés. Souris (glisser, molette) : immédiats.
    const [animating, setAnimating] = useState(false)
    const animTimer = useRef<number | undefined>(undefined)
    const stopAnimation = useCallback(() => {
        window.clearTimeout(animTimer.current)
        setAnimating(false)
    }, [])
    const goTo = useCallback((v: View, animate: boolean) => {
        window.clearTimeout(animTimer.current)
        setAnimating(animate)
        setView(v)
        if (animate) animTimer.current = window.setTimeout(() => setAnimating(false), ANIM_MS + 80)
    }, [])
    useEffect(() => () => window.clearTimeout(animTimer.current), [])

    const fit = useCallback((animate = true) => {
        if (!layout || layout.width === 0) return
        const k = Math.max(MIN_K, Math.min(1, (size.w - 40) / layout.width, (size.h - 40) / layout.height))
        goTo({k, x: (size.w - layout.width * k) / 2 + layout.ox * k, y: Math.max(20, (size.h - layout.height * k) / 2) + layout.oy * k}, animate)
    }, [layout, size, goTo])

    const centerOn = useCallback((n: LNode, animate = true, k = viewRef.current.k) => {
        goTo({k, x: size.w / 2 - (n.x + BOX_W / 2) * k, y: size.h / 2 - (n.y + BOX_H / 2) * k}, animate)
    }, [size, goTo])

    // case « arrivée » : brève mise en évidence après un recentrage, pour repérer où l'on se trouve
    const [arrivedKey, setArrivedKey] = useState<string | null>(null)
    const arrivedTimer = useRef<number | undefined>(undefined)
    useEffect(() => () => window.clearTimeout(arrivedTimer.current), [])
    const markArrived = useCallback((key: string) => {
        setArrivedKey(key)
        window.clearTimeout(arrivedTimer.current)
        arrivedTimer.current = window.setTimeout(() => setArrivedKey(null), 1400)
    }, [])

    // Recadre à chaque changement de centre / mode / profondeur (pas à chaque rafraîchissement des données).
    // Quand le CENTRE change (double-clic, pastille, « recentrer »…), la nouvelle personne est sélectionnée
    // et amenée au milieu de l'écran par une transition ; les autres cases glissent vers leur nouvelle place.
    const lastKey = useRef('')
    const lastFocus = useRef<string | null>(null)
    useEffect(() => {
        if (!fresh || !layout) return
        const key = `${focus}|${mode}|${qUp}|${qDown}`
        if (key === lastKey.current) return
        lastKey.current = key
        const fn = layout.nodes.find((n) => n.isFocus)
        const first = !initialized.current
        const focusChanged = lastFocus.current !== null && lastFocus.current !== focus
        lastFocus.current = focus
        if (first || !fn) {
            initialized.current = true
            fit(false)
        } else {
            centerOn(fn, true)
        }
        if (focusChanged && fn) {
            setSelectedKey(fn.key)
            markArrived(fn.key)
        } else {
            // la sélection survit si la personne est toujours dessinée (les clés sont celles des personnes)
            setSelectedKey((k) => (k && layout.nodes.some((n) => n.key === k) ? k : null))
        }
        setAdding(null)
    }, [fresh, layout, focus, mode, qUp, qDown, fit, centerOn, markArrived])

    useEffect(() => {
        const el = svg.current
        if (!el) return
        const onWheel = (e: WheelEvent) => {
            e.preventDefault()
            const r = el.getBoundingClientRect()
            const px = e.clientX - r.left
            const py = e.clientY - r.top
            const v = viewRef.current
            const k = Math.max(MIN_K, Math.min(MAX_K, v.k * Math.exp(-e.deltaY * 0.0015)))
            stopAnimation()
            setView({k, x: px - (px - v.x) * (k / v.k), y: py - (py - v.y) * (k / v.k)})
        }
        el.addEventListener('wheel', onWheel, {passive: false})
        return () => el.removeEventListener('wheel', onWheel)
    }, [layout])

    // Déplacement à la souris : le glisser peut partir de n'importe où, y compris d'une case.
    // On n'attend pas de « vrai » glisser (seuil de 4 px) pour laisser passer les clics simples.
    const DRAG_THRESHOLD = 4
    const drag = useRef<{ x: number; y: number; vx: number; vy: number; moved: boolean; onNode: boolean } | null>(null)
    const justDragged = useRef(false) // un glisser vient de se terminer : le clic qui suit ne doit rien sélectionner
    const onPointerDown = (e: PointerEvent<SVGSVGElement>) => {
        if (e.button !== 0) return
        const target = e.target as Element
        if (target.closest('.tadd, .tbadge')) return // boutons : pas de déplacement
        justDragged.current = false
        stopAnimation()
        drag.current = {x: e.clientX, y: e.clientY, vx: view.x, vy: view.y, moved: false, onNode: !!target.closest('.tnode')}
    }
    const onPointerMove = (e: PointerEvent<SVGSVGElement>) => {
        const d = drag.current
        if (!d) return
        const dx = e.clientX - d.x
        const dy = e.clientY - d.y
        if (!d.moved) {
            if (Math.hypot(dx, dy) < DRAG_THRESHOLD) return
            d.moved = true
            e.currentTarget.setPointerCapture(e.pointerId)
        }
        setView((v) => ({...v, x: d.vx + dx, y: d.vy + dy}))
    }
    const onPointerUp = (e: PointerEvent<SVGSVGElement>) => {
        const d = drag.current
        drag.current = null
        if (!d) return
        if (d.moved) {
            justDragged.current = true
            setTimeout(() => { justDragged.current = false }, 0) // le clic synthétique suit immédiatement
            if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId)
        } else if (!d.onNode) {
            setSelectedKey(null)
        }
    }

    // Arêtes : celles de la mise en page précédente s'effacent, les nouvelles apparaissent en fondu.
    type EdgeSet = { edges: LEdge[]; ox: number; oy: number }
    const lastEdges = useRef<EdgeSet | null>(null)
    const [leaving, setLeaving] = useState<EdgeSet | null>(null)
    useLayoutEffect(() => {
        if (!layout) return
        const old = lastEdges.current
        lastEdges.current = {edges: layout.edges, ox: layout.ox, oy: layout.oy}
        if (!old || old.edges.length === 0 || old.edges === layout.edges) return
        setLeaving(old)
        const timer = window.setTimeout(() => setLeaving(null), 320)
        return () => window.clearTimeout(timer)
    }, [layout])

    const zoomBy = (f: number) => {
        const v = viewRef.current
        const k = Math.max(MIN_K, Math.min(MAX_K, v.k * f))
        const cx = size.w / 2
        const cy = size.h / 2
        goTo({k, x: cx - (cx - v.x) * (k / v.k), y: cy - (cy - v.y) * (k / v.k)}, true)
    }

    // --- clavier : flèches = parent / enfant / voisin, Entrée = fiche, Origine = focus ---
    const move = (pred: (n: LNode) => boolean) => {
        if (!selectedNode || !layout) return
        const cand = layout.nodes.filter(pred)
        if (!cand.length) return
        cand.sort((a, b) => Math.abs(a.x - selectedNode.x) - Math.abs(b.x - selectedNode.x))
        setSelectedKey(cand[0].key)
        centerOnIfOutside(cand[0])
    }
    const centerOnIfOutside = (n: LNode) => {
        const v = viewRef.current
        const sx = n.x * v.k + v.x
        const sy = n.y * v.k + v.y
        if (sx < 0 || sy < 0 || sx + BOX_W * v.k > size.w || sy + BOX_H * v.k > size.h) centerOn(n)
    }
    const onKeyDown = (e: KeyboardEvent<SVGSVGElement>) => {
        if (!layout || !data) return
        if (!selectedNode) {
            if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Enter'].includes(e.key)) {
                const f = layout.nodes.find((n) => n.isFocus) ?? layout.nodes[0]
                if (f) { e.preventDefault(); setSelectedKey(f.key) }
            }
            return
        }
        const sel = selectedNode
        switch (e.key) {
            case 'ArrowUp': {
                e.preventDefault()
                const ps = new Set(parentIds(data, sel.id))
                move((n) => n.y < sel.y && ps.has(n.id))
                break
            }
            case 'ArrowDown': {
                e.preventDefault()
                const cs = new Set(childIds(data, sel.id))
                move((n) => n.y > sel.y && cs.has(n.id))
                break
            }
            case 'ArrowLeft':
            case 'ArrowRight': {
                e.preventDefault()
                const dir = e.key === 'ArrowLeft' ? -1 : 1
                const row = layout.nodes.filter((n) => n.y === sel.y && (n.x - sel.x) * dir > 0)
                row.sort((a, b) => Math.abs(a.x - sel.x) - Math.abs(b.x - sel.x))
                if (row[0]) { setSelectedKey(row[0].key); centerOnIfOutside(row[0]) }
                break
            }
            case 'Enter':
                e.preventDefault()
                openPerson(sel.id)
                break
            case ' ':
                e.preventDefault()
                setTreeFocus(sel.id)
                break
            case 'Escape':
                setSelectedKey(null)
                break
        }
    }

    /** Parents / enfants qui existent mais ne sont pas dessinés dans la vue courante. */
    const hiddenFor = (n: LNode, p: NonNullable<ReturnType<typeof people.get>>) => {
        // count = nombre à afficher dans la pastille ; hidden = parmi eux, ceux qui ne sont pas dessinés
        const out: { kind: BadgeKind; count: number; hidden: number }[] = []
        const bio = bioParents.get(n.id)
        if (p.hasFather && !(bio?.father && n.shownParents.includes(bio.father))) out.push({kind: 'father', count: 1, hidden: 1})
        if (p.hasMother && !(bio?.mother && n.shownParents.includes(bio.mother))) out.push({kind: 'mother', count: 1, hidden: 1})
        const kids = p.childCount - n.shownChildren
        // la pastille donne le NOMBRE TOTAL d'enfants (pas seulement les masqués) : c'est ce que l'on cherche à savoir
        if (kids > 0) out.push({kind: 'child', count: p.childCount, hidden: kids})
        return out
    }
    /** Clic sur une pastille : recentre sur la personne, en passant en sablier si la vue ne montre pas ce côté. */
    const revealHidden = (id: string, kind: BadgeKind) => {
        if (kind !== 'child' && mode === 'descendants') setMode('hourglass')
        if (kind === 'child' && mode === 'ancestors') setMode('hourglass')
        // la personne dont on a déplié les proches est sélectionnée et mise en évidence, même si elle était déjà au centre
        // (les clés des cases sont celles des personnes)
        setSelectedKey(id)
        markArrived(id)
        setTreeFocus(id)
    }

    if (!focus) return <p className="muted pad">{t('persons.empty')}</p>

    const focusNode = people.get(focus)
    const showSosa = !!rootId && focus === rootId

    return (
        <div className="tree-view">
            <div className="tree-toolbar" role="toolbar" aria-label={t('tree.toolbar')}>
                <div className="seg" role="group" aria-label={t('tree.mode')}>
                    {MODES.map((m) => (
                        <button key={m} aria-pressed={mode === m} onClick={() => setMode(m)}>{t(`tree.modes.${m}`)}</button>
                    ))}
                </div>
                {mode !== 'descendants' && (
                    <span className="stepper">{t('tree.ancestors')}
                        <button aria-label={t('tree.lessAncestors')} onClick={() => setGens(up - 1, down)} disabled={up <= 1}>−</button>
                        <b>{up}</b>
                        <button aria-label={t('tree.moreAncestors')} onClick={() => setGens(up + 1, down)} disabled={up >= 12}>+</button>
                    </span>
                )}
                {mode !== 'ancestors' && (
                    <span className="stepper">{t('tree.descendants')}
                        <button aria-label={t('tree.lessDescendants')} onClick={() => setGens(up, down - 1)} disabled={down <= 1}>−</button>
                        <b>{down}</b>
                        <button aria-label={t('tree.moreDescendants')} onClick={() => setGens(up, down + 1)} disabled={down >= 8}>+</button>
                    </span>
                )}
                <span className="spacer"/>
                {rootId && focus !== rootId && <button onClick={() => setTreeFocus(rootId)}>{t('tree.toRoot')}</button>}
                <button aria-label={t('tree.zoomOut')} onClick={() => zoomBy(1 / 1.25)}>−</button>
                <button aria-label={t('tree.zoomIn')} onClick={() => zoomBy(1.25)}>+</button>
                <button onClick={() => fit(true)}>{t('tree.fit')}</button>
            </div>
            <div className="tree-main">
                <div className="tree-canvas" ref={setBoxEl}>
                    {focusNode && (
                        <div className="tree-focus-label muted" aria-live="polite">
                            {t('tree.centeredOn', {name: personLabel(focusNode.person, t('common.unknown'))})}
                            {layout && ` · ${t('tree.count', {count: layout.nodes.length})}`}
                        </div>
                    )}
                    {data?.truncated && <p role="alert" className="error pad">{t('tree.truncated')}</p>}
                    <svg ref={svg} tabIndex={0} role="application"
                         aria-label={t('tree.title')} onKeyDown={onKeyDown}
                         onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp}>
                        <g className={`tview${animating ? ' animating' : ''}`}
                           style={{transform: `translate(${view.x}px, ${view.y}px) scale(${view.k})`}}>
                            {leaving && <g style={{transform: `translate(${leaving.ox}px, ${leaving.oy}px)`}}>
                                {leaving.edges.map((e, i) => <path key={`l${i}`} d={e.d} className={`tedge ${e.kind} leave`}/>)}
                            </g>}
                            {layout && <g style={{transform: `translate(${layout.ox}px, ${layout.oy}px)`}}>
                                {layout.edges.map((e, i) => <path key={`e${seq}-${i}`} d={e.d} className={`tedge ${e.kind} enter`}/>)}
                            </g>}
                            {layout?.nodes.map((n) => {
                                const p = people.get(n.id)
                                if (!p) return null
                                const label = personLabel(p.person, t('common.unknown'))
                                const isSel = n.key === selectedKey
                                return (
                                    <g key={n.key} className={`tnode${isSel ? ' selected' : ''}${n.isFocus ? ' focus' : ''}${n.key === arrivedKey ? ' arrived' : ''}`}
                                       style={{transform: `translate(${n.x}px, ${n.y}px)`}}
                                       onClick={(e) => { e.stopPropagation(); if (justDragged.current) return; setSelectedKey(n.key); setAdding(null); svg.current?.focus() }}
                                       onDoubleClick={() => setTreeFocus(n.id)}>
                                        <title>{label}</title>
                                        <rect className="tbox" width={BOX_W} height={BOX_H} rx={7}/>
                                        <rect width={6} height={BOX_H} rx={3} fill={SEX_COLOR[p.person.sex] ?? SEX_COLOR.unknown}/>
                                        <text x={14} y={24} className="tname">{truncate(label, 24)}</text>
                                        <text x={14} y={43} className="tyears">{years(p.birth, p.death, p.person.living)}</text>
                                        {(['father', 'mother', 'sibling', 'child', 'partner'] as const).filter((r) =>
                                            r === 'father' ? !p.hasFather : r === 'mother' ? !p.hasMother : r === 'sibling' ? p.hasParents : true,
                                        ).map((r) => (
                                            <AddButton key={r} rel={r} label={t(`tree.addTitle.${r}`)}
                                                       onAdd={() => { setSelectedKey(n.key); setAdding({relation: r, unionId: ''}) }}/>
                                        ))}
                                        {hiddenFor(n, p).map((h) => (
                                            <HiddenBadge key={h.kind} kind={h.kind} count={h.count} label={h.kind === 'child' && h.hidden < h.count
                                                ? t('tree.hidden.childSome', {count: h.count, hidden: h.hidden})
                                                : t(`tree.hidden.${h.kind}`, {count: h.count})}
                                                         onClick={() => revealHidden(n.id, h.kind)}/>
                                        ))}
                                        {showSosa && n.sosa !== undefined && <text x={BOX_W - 8} y={16} textAnchor="end" className="tsosa">{n.sosa}</text>}
                                    </g>
                                )
                            })}
                        </g>
                    </svg>
                </div>
                {selected && selectedNode && (
                    <aside className="tree-side" aria-label={t('tree.selection')}>
                        <h3>{personLabel(selected.person, t('common.unknown'))}</h3>
                        <p className="muted">{years(selected.birth, selected.death, selected.person.living) || t('common.none')}</p>
                        <div className="stack">
                            <button className="primary" onClick={() => openPerson(selected.person.id)}>{t('tree.openCard')}</button>
                            <button disabled={selected.person.id === focus} onClick={() => setTreeFocus(selected.person.id)}>{t('tree.recenter')}</button>
                        </div>
                        <h4>{t('tree.add')}</h4>
                        <div className="row">
                            {(['father', 'mother', 'spouse', 'partner', 'child', 'sibling'] as const).map((r) => (
                                <button key={r} aria-pressed={adding?.relation === r}
                                        onClick={() => setAdding(adding?.relation === r ? null : {relation: r, unionId: ''})}>
                                    {r !== 'spouse' && <img src={ICONS[r]} alt="" width={14} height={14}/>}{t(`tree.rel.${r}`)}
                                </button>
                            ))}
                        </div>
                        {adding && (
                            <QuickAdd key={`${selected.person.id}|${adding.relation}`} adding={adding} personId={selected.person.id}
                                      defaultSurname={defaultSurnameFor(adding.relation, selected.person, partnerOf(selected.person.id))}
                                      onDone={() => setAdding(null)}/>
                        )}
                    </aside>
                )}
            </div>
            <PersonEditor personId={selectedNode?.id ?? null} onOpen={openPerson} onFocus={setTreeFocus}
                          years={(id) => { const n = people.get(id); return n ? years(n.birth, n.death, n.person.living) : '' }}/>
        </div>
    )
}
