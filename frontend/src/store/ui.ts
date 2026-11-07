import {create} from 'zustand'

export type View = 'tree' | 'persons' | 'places' | 'sources' | 'media' | 'stats' | 'quality' | 'settings'
export type Theme = 'system' | 'light' | 'dark'
export type TreeMode = 'hourglass' | 'ancestors' | 'descendants'

interface UIState {
    view: View
    theme: Theme
    selectedPersonId: string | null
    paletteOpen: boolean
    error: string | null
    treeFocus: string | null
    importJob: string | null
    treeMode: TreeMode
    treeUp: number
    treeDown: number
    setView: (v: View) => void
    setTheme: (t: Theme) => void
    /** Ouvre la fiche d'une personne. */
    selectPerson: (id: string | null) => void
    setPaletteOpen: (open: boolean) => void
    setError: (message: string | null) => void
    setTreeFocus: (id: string | null) => void
    setImportJob: (id: string | null) => void
    setTreeMode: (m: TreeMode) => void
    setTreeGenerations: (up: number, down: number) => void
    /** Oublie tout l'état lié au projet précédent (sélection, centre de l'arbre…). */
    resetProjectState: () => void
}

function loadTheme(): Theme {
    try {
        const t = localStorage.getItem('theme')
        if (t === 'light' || t === 'dark' || t === 'system') return t
    } catch { /* stockage indisponible */ }
    return 'system'
}

const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n))

/** État purement UI (spec §2) ; les données métier passent par TanStack Query. */
export const useUI = create<UIState>((set) => ({
    view: 'tree',
    theme: loadTheme(),
    selectedPersonId: null,
    paletteOpen: false,
    error: null,
    treeFocus: null,
    importJob: null,
    treeMode: 'hourglass',
    treeUp: 4,
    treeDown: 2,
    setView: (view) => set({view}),
    setTheme: (theme) => {
        try { localStorage.setItem('theme', theme) } catch { /* ignoré */ }
        set({theme})
    },
    selectPerson: (selectedPersonId) => set({selectedPersonId, view: 'persons'}),
    setPaletteOpen: (paletteOpen) => set({paletteOpen}),
    setError: (error) => set({error}),
    setTreeFocus: (treeFocus) => set({treeFocus}),
    setImportJob: (importJob) => set({importJob}),
    setTreeMode: (treeMode) => set({treeMode}),
    setTreeGenerations: (up, down) => set({treeUp: clamp(up, 0, 12), treeDown: clamp(down, 0, 8)}),
    resetProjectState: () => set({selectedPersonId: null, treeFocus: null, paletteOpen: false, error: null}),
}))
