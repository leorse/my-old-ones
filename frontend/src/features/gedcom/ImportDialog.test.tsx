import {describe, expect, it, vi} from 'vitest'
import {render, screen, waitFor} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import '../../i18n'

const report = {
    fileName: 'x.ged', software: 'GENEATIQUE 2025', gedcomVersion: '5.5.1', encoding: 'ANSEL', declaredEncoding: 'ANSEL',
    individuals: 452, families: 181, unions: 174, parentLinks: 524, names: 453, events: 594, places: 113, sources: 1,
    citations: 3, notes: 0, extraTags: 1044, mediaSkipped: 1, autoDeceased: 99,
    skipped: {'MARR (vide)': 56}, warnings: ['famille @F108@ : lien de filiation en double pour @I100@, ignoré'],
    warningsDropped: 0, corrections: ['99 personne(s) nées il y a plus de 110 ans marquées décédées'], durationMs: 120,
}

let state = 'running'
vi.mock('../../ipc', () => ({
    ipc: {
        listJobs: vi.fn(async () => [{id: 'job-1', kind: 'gedcom-import', state, done: 50, total: 100, message: 'Personnes', error: 'fichier illisible'}]),
        gedcomReport: vi.fn(async () => report),
        cancelJob: vi.fn(),
        gedcomDialog: vi.fn(async () => '/tmp/x.ged'),
        importGedcomNewProject: vi.fn(async () => 'job-9'),
        importGedcomHere: vi.fn(async () => 'job-8'),
    },
}))

import {ipc} from '../../ipc'
import {useUI} from '../../store/ui'
import {ImportDialog, startGedcomImport} from './ImportDialog'

function renderDialog() {
    const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
    useUI.setState({importJob: 'job-1'})
    return render(<QueryClientProvider client={qc}><ImportDialog/></QueryClientProvider>)
}

describe('ImportDialog', () => {
    it('affiche la progression puis permet d\'annuler', async () => {
        state = 'running'
        renderDialog()
        expect(await screen.findByText(/50 \/ 100/)).toBeInTheDocument()
        expect(screen.getByRole('progressbar')).toHaveValue(50)
        screen.getByRole('button', {name: 'Annuler'}).click()
        await waitFor(() => expect(ipc.cancelJob).toHaveBeenCalledWith('job-1'))
    })

    it('affiche le rapport quand la tâche est terminée', async () => {
        state = 'done'
        renderDialog()
        expect(await screen.findByText('452')).toBeInTheDocument()
        expect(screen.getByText(/GENEATIQUE 2025 · GEDCOM 5.5.1 · encodage ANSEL/)).toBeInTheDocument()
        expect(screen.getByText(/1044 balise\(s\) non interprétée\(s\)/)).toBeInTheDocument()
        expect(screen.getByText(/1 objet\(s\) multimédia non importé/)).toBeInTheDocument()
        expect(screen.getByText(/lien de filiation en double/)).toBeInTheDocument()
        expect(screen.getByText(/MARR \(vide\) \(56\)/)).toBeInTheDocument()
    })

    it('signale un échec sans rien avoir écrit', async () => {
        state = 'failed'
        renderDialog()
        expect(await screen.findByRole('alert')).toHaveTextContent('fichier illisible')
        expect(screen.getByText('Rien n\'a été ajouté au projet.')).toBeInTheDocument()
    })
})

describe('startGedcomImport', () => {
    it('importe dans un nouveau projet ou dans le projet courant', async () => {
        const setJob = vi.fn()
        await startGedcomImport('new', setJob)
        expect(ipc.importGedcomNewProject).toHaveBeenCalledWith('/tmp/x.ged')
        expect(setJob).toHaveBeenLastCalledWith('job-9')
        await startGedcomImport('current', setJob)
        expect(ipc.importGedcomHere).toHaveBeenCalledWith('/tmp/x.ged')
        expect(setJob).toHaveBeenLastCalledWith('job-8')
    })

    it('ne fait rien si le dialogue est annulé', async () => {
        vi.mocked(ipc.gedcomDialog).mockResolvedValueOnce('')
        const setJob = vi.fn()
        await startGedcomImport('new', setJob)
        expect(setJob).not.toHaveBeenCalled()
    })
})
