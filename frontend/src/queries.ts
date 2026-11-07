import {useEffect, useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {Events, ipc, onEvent} from './ipc'
import {useUI} from './store/ui'

export const keys = {
    project: ['project'] as const,
    recents: ['recents'] as const,
    persons: ['persons'] as const,
    person: (id: string) => ['person', id] as const,
    places: (q: string) => ['places', q] as const,
    history: ['history'] as const,
    jobs: ['jobs'] as const,
}

export const useProject = () => useQuery({queryKey: keys.project, queryFn: () => ipc.currentProject()})
export const useRecents = () => useQuery({queryKey: keys.recents, queryFn: () => ipc.recentProjects()})
export const usePersons = (enabled = true) =>
    useQuery({queryKey: keys.persons, queryFn: () => ipc.listPersons(), enabled})
export const usePerson = (id: string | null) =>
    useQuery({queryKey: keys.person(id ?? ''), queryFn: () => ipc.getPerson(id!), enabled: !!id})
export const usePlaces = (q: string) =>
    useQuery({queryKey: keys.places(q), queryFn: () => ipc.listPlaces(q)})
export const useHistory = (enabled = true) =>
    useQuery({queryKey: keys.history, queryFn: () => ipc.historyState(), enabled})
export const useJobs = () => useQuery({queryKey: keys.jobs, queryFn: () => ipc.listJobs()})

/** Relie les événements backend à l'invalidation du cache de requêtes. */
export function useBackendEvents() {
    const qc = useQueryClient()
    useEffect(() => {
        const offs = [
            onEvent(Events.projectChanged, () => void qc.invalidateQueries()),
            // toute écriture ou annulation peut toucher n'importe quelle fiche
            onEvent(Events.historyChanged, () => void qc.invalidateQueries()),
            onEvent(Events.jobUpdate, () => void qc.invalidateQueries({queryKey: keys.jobs})),
        ]
        return () => offs.forEach((off) => off())
    }, [qc])
}

/**
 * Écriture générique : invalide tout le cache en cas de succès et affiche
 * l'erreur du backend dans la bannière globale en cas d'échec.
 */
export function useAction<A extends unknown[], R>(fn: (...args: A) => Promise<R>) {
    const qc = useQueryClient()
    const setError = useUI((s) => s.setError)
    return useMutation({
        mutationFn: (args: A) => fn(...args),
        onSuccess: () => { setError(null); return qc.invalidateQueries() },
        onError: (e) => setError(String((e as Error)?.message ?? e)),
    })
}

export function useUndoRedo() {
    const qc = useQueryClient()
    const done = () => qc.invalidateQueries()
    return {
        undo: useMutation({mutationFn: () => ipc.undo(), onSuccess: done}),
        redo: useMutation({mutationFn: () => ipc.redo(), onSuccess: done}),
    }
}

/** Valeur retardée (anti-rebond) pour les aperçus et recherches pendant la frappe. */
export function useDebounced<T>(value: T, ms = 200): T {
    const [v, setV] = useState(value)
    useEffect(() => {
        const t = setTimeout(() => setV(value), ms)
        return () => clearTimeout(t)
    }, [value, ms])
    return v
}
