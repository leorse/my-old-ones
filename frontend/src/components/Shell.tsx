import {useEffect, useRef} from 'react'
import {useQueryClient} from '@tanstack/react-query'
import {useTranslation} from 'react-i18next'
import {useBackendEvents, useHistory, useJobs, useProject, useUndoRedo} from '../queries'
import {ipc} from '../ipc'
import {useUI, type View} from '../store/ui'
import {Home} from '../features/home/Home'
import {PersonList} from '../features/persons/PersonList'
import {TreeView} from '../features/tree/TreeView'
import {CommandPalette} from './CommandPalette'
import {startGedcomImport} from '../features/gedcom/ImportDialog'

const NAV: View[] = ['tree', 'persons', 'places', 'sources', 'media', 'stats', 'quality', 'settings']

function useTheme() {
    const theme = useUI((s) => s.theme)
    useEffect(() => {
        document.documentElement.dataset.theme = theme
    }, [theme])
}

/** Raccourcis globaux : Ctrl/Cmd+Z annule, Ctrl/Cmd+Maj+Z ou Ctrl+Y rétablit. */
function useUndoShortcuts(undo: () => void, redo: () => void, active: boolean) {
    useEffect(() => {
        if (!active) return
        const h = (e: KeyboardEvent) => {
            const t = e.target as HTMLElement | null
            if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
            if (!(e.ctrlKey || e.metaKey)) return
            const k = e.key.toLowerCase()
            if (k === 'z' && !e.shiftKey) { e.preventDefault(); undo() }
            else if ((k === 'z' && e.shiftKey) || k === 'y') { e.preventDefault(); redo() }
        }
        window.addEventListener('keydown', h)
        return () => window.removeEventListener('keydown', h)
    }, [undo, redo, active])
}

export function Shell() {
    const {t} = useTranslation()
    useTheme()
    useBackendEvents()
    const project = useProject()
    const open = !!project.data
    const history = useHistory(open)
    const jobs = useJobs()
    const {undo, redo} = useUndoRedo()
    const view = useUI((s) => s.view)
    const setView = useUI((s) => s.setView)
    const error = useUI((s) => s.error)
    const setError = useUI((s) => s.setError)
    const setPaletteOpen = useUI((s) => s.setPaletteOpen)
    const setImportJob = useUI((s) => s.setImportJob)

    useUndoShortcuts(() => undo.mutate(), () => redo.mutate(), open)

    // Changement de projet (même sans passer par l'accueil) : on oublie la sélection, le centre de
    // l'arbre et toutes les données en cache du projet précédent, qui ne valent rien pour le nouveau.
    const qc = useQueryClient()
    const resetProjectState = useUI((s) => s.resetProjectState)
    const projectPath = project.data?.path ?? null
    const lastPath = useRef<string | null>(projectPath)
    useEffect(() => {
        const previous = lastPath.current
        lastPath.current = projectPath
        if (previous === null || previous === projectPath) return // première ouverture : rien à oublier
        resetProjectState()
        // resetQueries (et non removeQueries) : les requêtes actives repartent de zéro et se rechargent
        void qc.resetQueries({predicate: (q) => !['project', 'recents', 'jobs'].includes(q.queryKey[0] as string)})
    }, [projectPath, qc, resetProjectState])

    if (!open) return <Home/>

    const running = (jobs.data ?? []).filter((j) => j.state === 'running').length

    return (
        <div className="shell">
            <header className="titlebar">
                <strong>{project.data!.name}</strong>
                <span className="spacer"/>
                <button onClick={() => void startGedcomImport('current', setImportJob).catch((e) => setError(String(e?.message ?? e)))}>{t('toolbar.importGedcom')}</button>
                <button onClick={() => setPaletteOpen(true)}>🔍 {t('toolbar.search')}</button>
                <button disabled={!history.data?.canUndo} title={history.data?.undoLabel}
                        onClick={() => undo.mutate()}>↶ {t('toolbar.undo')}</button>
                <button disabled={!history.data?.canRedo} title={history.data?.redoLabel}
                        onClick={() => redo.mutate()}>↷ {t('toolbar.redo')}</button>
                <button onClick={() => void ipc.closeProject()}>{t('toolbar.close')}</button>
            </header>
            <nav className="sidebar" aria-label="Navigation">
                {NAV.map((v) => (
                    <button key={v} aria-current={view === v ? 'page' : undefined}
                            onClick={() => setView(v)}>{t(`nav.${v}`)}</button>
                ))}
            </nav>
            <main className="content" key={projectPath}>
                {error && (
                    <p role="alert" className="error banner">
                        {t('error', {message: error})}
                        <button className="icon" aria-label={t('common.close')} onClick={() => setError(null)}>✕</button>
                    </p>
                )}
                {view === 'persons' ? <PersonList/> : view === 'tree' ? <TreeView/> : <p className="muted pad">{t('placeholder')}</p>}
            </main>
            <CommandPalette/>
            <footer className="statusbar">
                <span>{t('status.persons', {count: project.data!.persons})}</span>
                {running > 0 && <span>{t('status.jobs', {count: running})}</span>}
            </footer>
        </div>
    )
}
